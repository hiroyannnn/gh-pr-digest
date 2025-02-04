package client

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cli/go-gh"
	"github.com/cli/go-gh/pkg/api"
)

// テスト用にtime.Now()をモック可能にする
var timeNow = time.Now

type PullRequest struct {
	Title      string    `json:"title"`
	URL        string    `json:"url"`
	HTMLURL    string    `json:"html_url"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	State      string    `json:"state"`
	Merged     bool      `json:"merged"`
	Draft      bool      `json:"draft"`
	Number     int       `json:"number"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

type PRClient struct {
	client api.RESTClient
	debug  bool
}

func NewPRClient() (*PRClient, error) {
	client, err := gh.RESTClient(nil)
	if err != nil {
		return nil, fmt.Errorf("GitHub クライアントの作成に失敗: %w", err)
	}
	return &PRClient{client: client}, nil
}

func (c *PRClient) SetDebug(debug bool) {
	c.debug = debug
}

func (c *PRClient) debugPrint(format string, args ...interface{}) {
	if c.debug {
		fmt.Printf(format, args...)
	}
}

func (c *PRClient) FetchTodaysPRs(org, repo, since, until string) ([]PullRequest, error) {
	query := buildSearchQuery(org, repo, since, until)

	// GitHub Search APIを使用してPRを検索
	response := struct {
		Items []struct {
			Title      string    `json:"title"`
			URL        string    `json:"url"`
			HTMLURL    string    `json:"html_url"`
			CreatedAt  time.Time `json:"created_at"`
			UpdatedAt  time.Time `json:"updated_at"`
			State      string    `json:"state"`
			Draft      bool      `json:"draft"`
			Number     int       `json:"number"`
			Repository struct {
				FullName string `json:"full_name"`
				HTMLURL  string `json:"html_url"`
			} `json:"repository"`
		} `json:"items"`
		Total int `json:"total_count"`
	}{}

	path := fmt.Sprintf("search/issues?%s", url.Values{
		"q":     []string{query},
		"sort":  []string{"updated"},
		"order": []string{"desc"},
	}.Encode())

	// デバッグ出力
	c.debugPrint("APIパス: %s\n", path)

	err := c.client.Get(path, &response)
	if err != nil {
		return nil, fmt.Errorf("PRの取得に失敗: %w", err)
	}

	// デバッグ出力
	c.debugPrint("検索結果: %d件\n", response.Total)
	for i, item := range response.Items {
		c.debugPrint("PR %d: [%s] %s (#%d)\n", i+1, extractRepoFullName(item.URL), item.Title, item.Number)
		c.debugPrint("  URL: %s\n", item.URL)
		c.debugPrint("  HTML URL: %s\n", item.HTMLURL)
	}

	// レスポンスをPullRequest型に変換
	var prs []PullRequest
	for _, item := range response.Items {
		// PRの詳細情報を取得
		var prDetail struct {
			Title      string    `json:"title"`
			URL        string    `json:"url"`
			HTMLURL    string    `json:"html_url"`
			CreatedAt  time.Time `json:"created_at"`
			UpdatedAt  time.Time `json:"updated_at"`
			State      string    `json:"state"`
			Merged     bool      `json:"merged"`
			Draft      bool      `json:"draft"`
			Number     int       `json:"number"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		}

		repoPath := extractRepoFullName(item.URL)
		prPath := fmt.Sprintf("repos/%s/pulls/%d", repoPath, item.Number)

		// デバッグ出力を追加
		c.debugPrint("PR API パス: %s\n", prPath)
		c.debugPrint("  元のURL: %s\n", item.URL)
		c.debugPrint("  変換後のURL: %s\n", convertToPullsURL(item.URL))

		err := c.client.Get(prPath, &prDetail)
		if err != nil {
			c.debugPrint("PR詳細の取得に失敗: %v\n", err)
			continue
		}

		// デバッグ出力を追加
		c.debugPrint("PR詳細: [%s] %s (#%d)\n", repoPath, prDetail.Title, prDetail.Number)
		c.debugPrint("  Draft (Search API): %v\n", item.Draft)
		c.debugPrint("  Draft (Pulls API): %v\n", prDetail.Draft)
		c.debugPrint("  State: %s\n", prDetail.State)
		c.debugPrint("  Merged: %v\n", prDetail.Merged)
		c.debugPrint("  URL: %s\n", prDetail.URL)

		// Search APIとPulls APIの両方からドラフト状態を確認
		isDraft := prDetail.Draft || item.Draft

		prs = append(prs, PullRequest{
			Title:     prDetail.Title,
			URL:       convertToPullsURL(item.URL), // URLを/pullsに変換
			HTMLURL:   prDetail.HTMLURL,
			CreatedAt: prDetail.CreatedAt,
			UpdatedAt: prDetail.UpdatedAt,
			State:     prDetail.State,
			Merged:    prDetail.Merged,
			Draft:     isDraft,
			Number:    prDetail.Number,
			Repository: struct {
				FullName string `json:"full_name"`
			}{
				FullName: repoPath,
			},
		})
	}

	// 各PRのコミット情報を確認し、自分のコミットがあるものだけをフィルタリング
	var filteredPRs []PullRequest
	for _, pr := range prs {
		hasMyCommit, err := c.hasMyCommitInRange(pr, since, until)
		if err != nil {
			continue // エラーの場合はスキップ
		}
		if pr.IsAuthor() || hasMyCommit {
			filteredPRs = append(filteredPRs, pr)
		}
	}

	return filteredPRs, nil
}

func extractRepoFullName(apiURL string) string {
	parts := strings.Split(apiURL, "/")
	if len(parts) >= 6 {
		return fmt.Sprintf("%s/%s", parts[4], parts[5])
	}
	return ""
}

// /issues URLを/pulls URLに変換する
func convertToPullsURL(issuesURL string) string {
	return strings.Replace(issuesURL, "/issues/", "/pulls/", 1)
}

func (c *PRClient) hasMyCommitInRange(pr PullRequest, since, until string) (bool, error) {
	// 自分のGitHubユーザー名を取得
	var user struct {
		Login string `json:"login"`
	}
	if err := c.client.Get("user", &user); err != nil {
		c.debugPrint("ユーザー情報取得エラー: %v\n", err)
		return false, err
	}

	// デバッグ出力
	c.debugPrint("ユーザー名: %s\n", user.Login)

	// 日付範囲の設定
	var sinceTime, untilTime time.Time
	var err1, err2 error
	if since != "" {
		sinceTime, err1 = time.Parse("2006-01-02", since)
	} else {
		sinceTime = timeNow().Truncate(24 * time.Hour) // 今日の0時
	}
	if until != "" {
		untilTime, err2 = time.Parse("2006-01-02", until)
		untilTime = untilTime.Add(24 * time.Hour) // 指定日の終わり
	} else {
		untilTime = timeNow().Add(24 * time.Hour) // 明日の0時
	}
	if err1 != nil || err2 != nil {
		return false, fmt.Errorf("日付の解析に失敗: %v, %v", err1, err2)
	}

	// デバッグ出力
	c.debugPrint("日付範囲: %s 〜 %s\n", sinceTime.Format("2006-01-02 15:04:05"), untilTime.Format("2006-01-02 15:04:05"))

	// PRの作成者が自分の場合はtrueを返す
	if pr.IsAuthor() {
		return true, nil
	}

	// PRのコミット情報を取得
	var commits []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Author struct {
				Name  string    `json:"name"`
				Email string    `json:"email"`
				Date  time.Time `json:"date"`
			} `json:"author"`
			Committer struct {
				Name  string    `json:"name"`
				Email string    `json:"email"`
				Date  time.Time `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		Committer struct {
			Login string `json:"login"`
		} `json:"committer"`
	}

	repoPath := pr.Repository.FullName
	commitPath := fmt.Sprintf("repos/%s/pulls/%d/commits", repoPath, pr.Number)

	// デバッグ出力
	c.debugPrint("コミット取得: %s\n", commitPath)

	err := c.client.Get(commitPath, &commits)
	if err != nil {
		// 404エラーの場合はfalseを返す（アクセス権限がない場合など）
		if strings.Contains(err.Error(), "404") {
			c.debugPrint("コミット取得スキップ（404）: %s\n", commitPath)
			return false, nil
		}
		c.debugPrint("コミット取得エラー: %v\n", err)
		return false, err
	}

	// コミットを確認
	for _, commit := range commits {
		// コミットの作者またはコミッターが自分の場合
		if (commit.Author.Login == user.Login || commit.Committer.Login == user.Login) &&
			commit.Commit.Author.Date.After(sinceTime) &&
			commit.Commit.Author.Date.Before(untilTime) {
			return true, nil
		}
	}

	return false, nil
}

func (pr PullRequest) IsAuthor() bool {
	// 自分がPRの作者かどうかを確認
	// Note: Search APIのauthor:@meで既にフィルタリングされているため、
	// ここではtrueを返すだけで十分
	return true
}

func buildSearchQuery(org, repo, since, until string) string {
	var dateRange string
	if since != "" && until != "" {
		dateRange = fmt.Sprintf("updated:%s..%s", since, until)
	} else if since != "" {
		dateRange = fmt.Sprintf("updated:>=%s", since)
	} else if until != "" {
		dateRange = fmt.Sprintf("updated:<=%s", until)
	} else {
		dateRange = fmt.Sprintf("updated:%s", timeNow().Format("2006-01-02"))
	}

	// 作者が自分のPRを検索（コミットは別途確認）
	// draft:trueとdraft:falseの両方を含めるためにis:prのみを使用
	query := fmt.Sprintf("is:pr %s author:@me", dateRange)

	if org != "" {
		query += fmt.Sprintf(" org:%s", org)
	}
	if repo != "" {
		query += fmt.Sprintf(" repo:%s", repo)
	}

	return query
}
