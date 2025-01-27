# gh-pr-digest

GitHubの今日作成されたPull Requestsを簡単に確認できるGitHub CLI拡張機能です。

## 概要

`gh pr-digest`（または短縮コマンド `gh prd`）は、GitHub CLIの拡張として動作し、指定した組織やリポジトリの今日作成されたPull Requestsの一覧を表示します。

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

# 出力形式を指定（テキスト/JSON）
gh prd --format json
```

### 出力例

```
Today's Pull Requests (2024-03-21):

[user/repo] Fix bug in login process (#123)
https://github.com/user/repo/pull/123

[user/repo] Add new feature (#124)
https://github.com/user/repo/pull/124
```

## 必要条件

- GitHub CLI (gh) がインストールされていること
- GitHub APIのアクセストークンが設定されていること

## ライセンス

MIT
