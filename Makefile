.PHONY: help release test build delete-tag

help: ## コマンド一覧を表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

release: test ## 新しいリリースを作成 (例: make release version=1.0.0)
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

test: ## テストを実行
	go test -v ./...
