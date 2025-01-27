# gh-pr-digest

GitHubの今日作成されたPull Requestsを簡単に確認できるGitHub CLI拡張機能です。

## 概要

`gh pr-digest`（または短縮コマンド `gh prd`）は、GitHub CLIの拡張として動作し、指定した組織やリポジトリのPull Requestsの一覧を表示します。

## インストール

```bash
gh extension install <your-username>/gh-pr-digest
```

## 使い方

### 基本的な使用方法

```bash
gh pr-digest  # 通常コマンド
gh prd        # 短縮コマンド
```

現在のリポジトリの今日作成されたPRを表示します。

### オプション

```bash
# 特定の組織のPRを表示
gh prd -o <organization>

# 特定のリポジトリのPRを表示
gh prd -r <owner>/<repository>

# 日付範囲を指定して表示
gh prd --since 2024-01-25 --until 2024-01-25

# 出力形式を指定（テキスト/JSON）
gh prd --format json
```

### 出力例

```
Your Pull Requests (2024-01-25 〜 2024-01-25):

🟣 新機能の追加
https://github.com/owner/repo/pull/562
```

PRのステータスは絵文字で表示されます：

- 🟢：オープン
- 🔴：クローズ
- 🟣：マージ済み

## 必要条件

- GitHub CLI (gh) がインストールされていること
- GitHub APIのアクセストークンが設定されていること

## ライセンス

MIT
