package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuildSearchQuery(t *testing.T) {
	// 固定の日付を使用してテスト
	fixedDate := "2024-02-04"
	tests := []struct {
		name     string
		org      string
		repo     string
		since    string
		until    string
		expected string
	}{
		{
			name:     "基本的なクエリ",
			org:      "",
			repo:     "",
			since:    "",
			until:    "",
			expected: "is:pr updated:" + fixedDate + " author:@me draft:*",
		},
		{
			name:     "組織指定",
			org:      "testorg",
			repo:     "",
			since:    "",
			until:    "",
			expected: "is:pr updated:" + fixedDate + " author:@me draft:* org:testorg",
		},
		{
			name:     "リポジトリ指定",
			org:      "",
			repo:     "owner/repo",
			since:    "",
			until:    "",
			expected: "is:pr updated:" + fixedDate + " author:@me draft:* repo:owner/repo",
		},
		{
			name:     "日付範囲指定",
			org:      "",
			repo:     "",
			since:    "2024-01-01",
			until:    "2024-01-31",
			expected: "is:pr updated:2024-01-01..2024-01-31 author:@me draft:*",
		},
		{
			name:     "すべての条件指定",
			org:      "testorg",
			repo:     "owner/repo",
			since:    "2024-01-01",
			until:    "2024-01-31",
			expected: "is:pr updated:2024-01-01..2024-01-31 author:@me draft:* org:testorg repo:owner/repo",
		},
	}

	// time.Now()をモック
	now := time.Date(2024, 2, 4, 0, 0, 0, 0, time.UTC)
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSearchQuery(tt.org, tt.repo, tt.since, tt.until)
			if got != tt.expected {
				t.Errorf("buildSearchQuery() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExtractRepoFullName(t *testing.T) {
	tests := []struct {
		name     string
		apiURL   string
		expected string
	}{
		{
			name:     "正常なURL",
			apiURL:   "https://api.github.com/repos/owner/repo/issues/1",
			expected: "owner/repo",
		},
		{
			name:     "不正なURL",
			apiURL:   "invalid-url",
			expected: "",
		},
		{
			name:     "空のURL",
			apiURL:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRepoFullName(tt.apiURL)
			if got != tt.expected {
				t.Errorf("extractRepoFullName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPullRequest_IsAuthor(t *testing.T) {
	pr := PullRequest{}
	if !pr.IsAuthor() {
		t.Error("IsAuthor() = false, want true")
	}
}

// モックサーバーを作成するヘルパー関数
func setupMockServer(t *testing.T, responses map[string]interface{}) (*httptest.Server, *PRClient) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// パスとクエリを結合
		fullPath := r.URL.Path
		if r.URL.RawQuery != "" {
			fullPath = fmt.Sprintf("%s?%s", r.URL.Path, r.URL.RawQuery)
		}

		// デバッグ出力
		t.Logf("Received request: %s", fullPath)
		for path := range responses {
			t.Logf("Available path: %s", path)
		}

		response, exists := responses[fullPath]
		if !exists {
			t.Errorf("Unexpected request to %s", fullPath)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	client := &PRClient{
		client: &mockRESTClient{
			baseURL: server.URL,
			t:       t,
		},
		debug: true,
	}

	return server, client
}

// モックのRESTクライアント
type mockRESTClient struct {
	baseURL string
	t       *testing.T
}

func (c *mockRESTClient) Do(method string, path string, body io.Reader, response interface{}) error {
	return c.DoWithContext(context.Background(), method, path, body, response)
}

func (c *mockRESTClient) DoWithContext(ctx context.Context, method string, path string, body io.Reader, response interface{}) error {
	if method != "GET" {
		return fmt.Errorf("method %s not implemented", method)
	}
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s/%s", c.baseURL, path), body)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(response)
}

func (c *mockRESTClient) Get(path string, response interface{}) error {
	return c.Do("GET", path, nil, response)
}

func (c *mockRESTClient) Delete(path string, response interface{}) error {
	return fmt.Errorf("not implemented")
}

func (c *mockRESTClient) Post(path string, body io.Reader, response interface{}) error {
	return fmt.Errorf("not implemented")
}

func (c *mockRESTClient) Put(path string, body io.Reader, response interface{}) error {
	return fmt.Errorf("not implemented")
}

func (c *mockRESTClient) Patch(path string, body io.Reader, response interface{}) error {
	return fmt.Errorf("not implemented")
}

func (c *mockRESTClient) Request(method string, path string, body io.Reader) (*http.Response, error) {
	return c.RequestWithContext(context.Background(), method, path, body)
}

func (c *mockRESTClient) RequestWithContext(ctx context.Context, method string, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s/%s", c.baseURL, path), body)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

func TestPRClient_FetchTodaysPRs(t *testing.T) {
	// 固定の日付を使用してテスト
	fixedDate := "2024-02-04"
	now := time.Date(2024, 2, 4, 0, 0, 0, 0, time.UTC)
	timeNow = func() time.Time { return now }
	defer func() { timeNow = time.Now }()

	searchPath := fmt.Sprintf("/search/issues?order=desc&q=is%%3Apr+updated%%3A%s+author%%3A%%40me+draft%%3A%%2A&sort=updated", fixedDate)

	// モックレスポンスの準備
	searchResponse := struct {
		Items []struct {
			Title      string    `json:"title"`
			URL        string    `json:"url"`
			HTMLURL    string    `json:"html_url"`
			CreatedAt  time.Time `json:"created_at"`
			UpdatedAt  time.Time `json:"updated_at"`
			State      string    `json:"state"`
			Number     int       `json:"number"`
			Repository struct {
				FullName string `json:"full_name"`
				HTMLURL  string `json:"html_url"`
			} `json:"repository"`
		} `json:"items"`
		Total int `json:"total_count"`
	}{
		Items: []struct {
			Title      string    `json:"title"`
			URL        string    `json:"url"`
			HTMLURL    string    `json:"html_url"`
			CreatedAt  time.Time `json:"created_at"`
			UpdatedAt  time.Time `json:"updated_at"`
			State      string    `json:"state"`
			Number     int       `json:"number"`
			Repository struct {
				FullName string `json:"full_name"`
				HTMLURL  string `json:"html_url"`
			} `json:"repository"`
		}{
			{
				Title:     "Test PR",
				URL:       "https://api.github.com/repos/owner/repo/pulls/1",
				HTMLURL:   "https://github.com/owner/repo/pull/1",
				CreatedAt: now,
				UpdatedAt: now,
				State:     "open",
				Number:    1,
				Repository: struct {
					FullName string `json:"full_name"`
					HTMLURL  string `json:"html_url"`
				}{
					FullName: "owner/repo",
				},
			},
		},
		Total: 1,
	}

	prResponse := struct {
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
	}{
		Title:     "Test PR",
		URL:       "https://api.github.com/repos/owner/repo/pulls/1",
		HTMLURL:   "https://github.com/owner/repo/pull/1",
		CreatedAt: now,
		UpdatedAt: now,
		State:     "open",
		Merged:    false,
		Draft:     true,
		Number:    1,
		Repository: struct {
			FullName string `json:"full_name"`
		}{
			FullName: "owner/repo",
		},
	}

	userResponse := struct {
		Login string `json:"login"`
	}{
		Login: "testuser",
	}

	responses := map[string]interface{}{
		searchPath:                          searchResponse,
		"/repos/owner/repo/pulls/1":         prResponse,
		"/user":                             userResponse,
		"/repos/owner/repo/pulls/1/commits": []struct{}{},
	}

	server, client := setupMockServer(t, responses)
	defer server.Close()

	prs, err := client.FetchTodaysPRs("", "", "", "")
	if err != nil {
		t.Fatalf("FetchTodaysPRs() error = %v", err)
	}

	if len(prs) != 1 {
		t.Errorf("FetchTodaysPRs() returned %d PRs, want 1", len(prs))
	}

	pr := prs[0]
	if pr.Title != "Test PR" {
		t.Errorf("PR.Title = %v, want Test PR", pr.Title)
	}
	if !pr.Draft {
		t.Errorf("PR.Draft = false, want true")
	}
}
