.PHONY: help release test build delete-tag install-local reinstall-local

help: ## コマンド一覧を表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## バイナリをビルド
	go build

install-local: build ## ローカルの拡張機能をインストール
	gh extension install .

reinstall-local: ## ローカルの拡張機能を再インストール（更新用）
	@echo "拡張機能を再インストールします..."
	@gh extension remove gh-pr-digest 2>/dev/null || true
	@$(MAKE) install-local
	@echo "再インストールが完了しました"

reinstall-prod: ## プロダクションの拡張機能を再インストール（更新用）
	@echo "拡張機能を再インストールします..."
	@gh extension remove gh-pr-digest 2>/dev/null || true
	@gh extension install hiroyannnn/gh-pr-digest
	@echo "再インストールが完了しました"

test: ## テストを実行
	go test ./...

test-v: ## テストを詳細なログ付きで実行
	go test -v ./...

release: ## 新しいリリースを作成 (例: make release version=1.0.0)
	@if [ -z "$(version)" ]; then \
		echo "バージョンを指定してください (例: make release version=1.0.0)"; \
		exit 1; \
	fi
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "コミットされていない変更があります"; \
		exit 1; \
	fi
	@if git rev-parse "v$(version)" >/dev/null 2>&1; then \
		echo "タグ v$(version) は既に存在します"; \
		echo "新しいバージョン番号を指定してください"; \
		echo "または 'make delete-tag version=$(version)' を実行してタグを削除してください"; \
		exit 1; \
	fi
	@echo "テストを実行中..."
	@if ! make test > /dev/null; then \
		echo "テストが失敗しました"; \
		echo "詳細を確認するには 'make test-v' を実行してください"; \
		exit 1; \
	fi
	@echo "v$(version) をリリースします..."
	@git tag "v$(version)"
	@git push origin "v$(version)"
	@echo "GitHub Actionsでリリースビルドが開始されます"
	@echo "https://github.com/hiroyannnn/gh-pr-digest/actions でビルドの進行状況を確認してください"

delete-tag: ## タグを削除 (例: make delete-tag version=1.0.0)
	@if [ -z "$(version)" ]; then \
		echo "バージョンを指定してください (例: make delete-tag version=1.0.0)"; \
		exit 1; \
	fi
	@if ! git rev-parse "v$(version)" >/dev/null 2>&1; then \
		echo "タグ v$(version) は存在しません"; \
		exit 1; \
	fi
	@echo "タグ v$(version) を削除します..."
	@git push origin --delete "v$(version)"
	@git tag -d "v$(version)"
	@echo "タグを削除しました"
