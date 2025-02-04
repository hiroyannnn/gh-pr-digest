package client

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
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
	// キャッシュの追加
	userCache      *string
	userCacheMux   sync.RWMutex
	commitCache    map[string]bool
	commitCacheMux sync.RWMutex
}

func NewPRClient() (*PRClient, error) {
	client, err := gh.RESTClient(nil)
	if err != nil {
		return nil, fmt.Errorf("GitHub クライアントの作成に失敗: %w", err)
	}
	return &PRClient{
		client:      client,
		commitCache: make(map[string]bool),
	}, nil
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
		"q":        []string{query},
		"sort":     []string{"updated"},
		"order":    []string{"desc"},
		"per_page": []string{"100"}, // 一度に取得するPR数を増やす
	}.Encode())

	// デバッグ出力
	c.debugPrint("APIパス: %s\n", path)

	err := c.client.Get(path, &response)
	if err != nil {
		return nil, fmt.Errorf("PRの取得に失敗: %w", err)
	}

	// 並列処理用のチャネルとエラーチャネルを作成
	prChan := make(chan PullRequest, len(response.Items))
	errChan := make(chan error, len(response.Items))
	semaphore := make(chan struct{}, 10) // 同時実行数を制限

	// 各PRの詳細情報を並列で取得
	var wg sync.WaitGroup
	for _, item := range response.Items {
		wg.Add(1)
		go func(item struct {
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
		}) {
			defer wg.Done()
			semaphore <- struct{}{}        // セマフォ取得
			defer func() { <-semaphore }() // セマフォ解放

			repoPath := extractRepoFullName(item.URL)
			prPath := fmt.Sprintf("repos/%s/pulls/%d", repoPath, item.Number)

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

			err := c.client.Get(prPath, &prDetail)
			if err != nil {
				errChan <- fmt.Errorf("PR詳細の取得に失敗: %w", err)
				return
			}

			// コミット情報を取得（必要な場合のみ）
			hasMyCommit, err := c.hasMyCommitInRange(PullRequest{
				Title:      prDetail.Title,
				URL:        convertToPullsURL(item.URL),
				HTMLURL:    prDetail.HTMLURL,
				CreatedAt:  prDetail.CreatedAt,
				UpdatedAt:  prDetail.UpdatedAt,
				State:      prDetail.State,
				Merged:     prDetail.Merged,
				Draft:      prDetail.Draft || item.Draft,
				Number:     prDetail.Number,
				Repository: prDetail.Repository,
			}, since, until)
			if err != nil {
				errChan <- fmt.Errorf("コミット情報の取得に失敗: %w", err)
				return
			}

			if prDetail.Draft || item.Draft || hasMyCommit {
				prChan <- PullRequest{
					Title:      prDetail.Title,
					URL:        convertToPullsURL(item.URL),
					HTMLURL:    prDetail.HTMLURL,
					CreatedAt:  prDetail.CreatedAt,
					UpdatedAt:  prDetail.UpdatedAt,
					State:      prDetail.State,
					Merged:     prDetail.Merged,
					Draft:      prDetail.Draft || item.Draft,
					Number:     prDetail.Number,
					Repository: prDetail.Repository,
				}
			}
		}(item)
	}

	// 完了を待つためのゴルーチン
	go func() {
		wg.Wait()
		close(prChan)
		close(errChan)
	}()

	// 結果の収集
	var prs []PullRequest
	var errors []error
	for {
		select {
		case pr, ok := <-prChan:
			if !ok {
				// エラーがあれば最初のエラーを返す
				if len(errors) > 0 {
					return nil, errors[0]
				}
				return prs, nil
			}
			prs = append(prs, pr)
		case err, ok := <-errChan:
			if !ok {
				continue
			}
			errors = append(errors, err)
		}
	}
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

func (c *PRClient) getUser() (string, error) {
	c.userCacheMux.RLock()
	if c.userCache != nil {
		defer c.userCacheMux.RUnlock()
		return *c.userCache, nil
	}
	c.userCacheMux.RUnlock()

	c.userCacheMux.Lock()
	defer c.userCacheMux.Unlock()

	// ダブルチェック
	if c.userCache != nil {
		return *c.userCache, nil
	}

	var user struct {
		Login string `json:"login"`
	}
	if err := c.client.Get("user", &user); err != nil {
		return "", fmt.Errorf("ユーザー情報の取得に失敗: %w", err)
	}

	c.userCache = &user.Login
	return user.Login, nil
}

func (c *PRClient) hasMyCommitInRange(pr PullRequest, since, until string) (bool, error) {
	// キャッシュキーの生成
	cacheKey := fmt.Sprintf("%s-%d-%s-%s", pr.Repository.FullName, pr.Number, since, until)

	// キャッシュチェック
	c.commitCacheMux.RLock()
	if result, ok := c.commitCache[cacheKey]; ok {
		c.commitCacheMux.RUnlock()
		return result, nil
	}
	c.commitCacheMux.RUnlock()

	// 日付範囲の設定
	var sinceTime, untilTime time.Time
	var err1, err2 error
	if since != "" {
		sinceTime, err1 = time.Parse("2006-01-02", since)
	} else {
		sinceTime = timeNow().Truncate(24 * time.Hour)
	}
	if until != "" {
		untilTime, err2 = time.Parse("2006-01-02", until)
		untilTime = untilTime.Add(24 * time.Hour)
	} else {
		untilTime = timeNow().Add(24 * time.Hour)
	}
	if err1 != nil || err2 != nil {
		return false, fmt.Errorf("日付の解析に失敗: %v, %v", err1, err2)
	}

	// PRの作成者が自分の場合はtrueを返す
	if pr.IsAuthor() {
		c.commitCacheMux.Lock()
		c.commitCache[cacheKey] = true
		c.commitCacheMux.Unlock()
		return true, nil
	}

	// ユーザー名の取得（キャッシュ使用）
	username, err := c.getUser()
	if err != nil {
		return false, err
	}

	// PRのコミット情報を取得
	var commits []struct {
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

	commitPath := fmt.Sprintf("repos/%s/pulls/%d/commits", pr.Repository.FullName, pr.Number)
	err = c.client.Get(commitPath, &commits)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			c.commitCacheMux.Lock()
			c.commitCache[cacheKey] = false
			c.commitCacheMux.Unlock()
			return false, nil
		}
		return false, err
	}

	// コミットを確認
	for _, commit := range commits {
		if (commit.Author.Login == username || commit.Committer.Login == username) &&
			commit.Commit.Author.Date.After(sinceTime) &&
			commit.Commit.Author.Date.Before(untilTime) {
			c.commitCacheMux.Lock()
			c.commitCache[cacheKey] = true
			c.commitCacheMux.Unlock()
			return true, nil
		}
	}

	c.commitCacheMux.Lock()
	c.commitCache[cacheKey] = false
	c.commitCacheMux.Unlock()
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
