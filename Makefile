.PHONY: migrate run test test-short vet sqlc swag check

migrate: ## Apply pending schema migrations against $DATABASE_URL (never run on app boot)
	go run ./cmd/migrate

run: ## Start the Pinalert web server (created in plan 01-03)
	go run ./cmd/server

test: ## Full suite, requires DATABASE_URL pointing at a real Postgres
	go test ./... -v

test-short: ## Skips any test that needs DATABASE_URL
	go test ./... -short

vet:
	go vet ./...

sqlc: ## Regenerate internal/store/sqlc/ from internal/store/queries — commit the result
	sqlc generate

swag: ## Regenerate docs/ (Swagger UI + doc.json) from handler doc-comments
	swag init -g internal/api/router.go -o docs

check: vet test-short
