package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/hiroyannnn/gh-pr-digest/client"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "pr-digest",
		Short:   "Show today's pull requests",
		Aliases: []string{"prd"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRoot(cmd)
		},
	}

	rootCmd.Flags().StringP("org", "o", "", "指定した組織のPRを表示")
	rootCmd.Flags().StringP("repo", "r", "", "指定したリポジトリのPRを表示")
	rootCmd.Flags().String("format", "text", "出力形式（text/json）")
	rootCmd.Flags().String("since", "", "指定した日付以降のPRを表示（YYYY-MM-DD形式）")
	rootCmd.Flags().String("until", "", "指定した日付までのPRを表示（YYYY-MM-DD形式）")
	rootCmd.Flags().Bool("debug", false, "デバッグ情報を表示")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command) error {
	org, _ := cmd.Flags().GetString("org")
	repo, _ := cmd.Flags().GetString("repo")
	format, _ := cmd.Flags().GetString("format")
	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")
	debug, _ := cmd.Flags().GetBool("debug")

	c, err := client.NewPRClient()
	if err != nil {
		return err
	}

	c.SetDebug(debug)
	prs, err := c.FetchTodaysPRs(org, repo, since, until)
	if err != nil {
		return err
	}

	switch format {
	case "json":
		return outputJSON(prs)
	default:
		return outputText(prs, since, until)
	}
}

func outputJSON(prs []client.PullRequest) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(prs)
}

func outputText(prs []client.PullRequest, since, until string) error {
	if len(prs) == 0 {
		if since != "" || until != "" {
			fmt.Printf("指定期間（%s 〜 %s）に作成または更新したPRはありません\n",
				since, until)
		} else {
			fmt.Println("今日作成または更新したPRはありません")
		}
		return nil
	}

	if since != "" || until != "" {
		fmt.Printf("Your Pull Requests (%s 〜 %s):\n\n",
			since, until)
	} else {
		fmt.Printf("Your Pull Requests Updated Today (%s):\n\n",
			time.Now().Format("2006-01-02"))
	}

	for _, pr := range prs {
		// ステータスに応じて表示を変更
		stateStr := ""
		if pr.Merged {
			stateStr = "🟣" // 紫：マージ済み
		} else if pr.State == "closed" {
			stateStr = "🔴" // 赤：クローズ
		} else if pr.Draft {
			stateStr = "⚪️" // 白：ドラフト
		} else {
			stateStr = "🟢" // 緑：オープン
		}

		// fmt.Printf("%s [%s] %s (#%d)\n", stateStr, pr.Repository.FullName, pr.Title, pr.Number)
		fmt.Printf("%s %s\n", stateStr, pr.Title)
		// fmt.Printf("Created: %s, Updated: %s\n",
		// 	pr.CreatedAt.Format("2006-01-02 15:04"),
		// 	pr.UpdatedAt.Format("2006-01-02 15:04"))
		fmt.Printf("%s\n\n", pr.HTMLURL)
	}

	return nil
}
