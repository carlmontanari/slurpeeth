.DEFAULT_GOAL := help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

fmt: ## Run formatters
	GOARCH=amd64 GOOS=linux gofumpt -w .
	GOARCH=amd64 GOOS=linux goimports -w .
	GOARCH=amd64 GOOS=linux golines -w .

lint: fmt ## Run linters; runs with GOOS env var for linting on darwin
	GOARCH=amd64 GOOS=linux golangci-lint run

test: ## Run unit tests
	GOARCH=amd64 GOOS=linux gotestsum --format testname --hide-summary=skipped -- -coverprofile=cover.out ./...

test-race: ## Run unit tests with race flag
	GOARCH=amd64 GOOS=linux gotestsum --format testname --hide-summary=skipped -- -race -coverprofile=cover.out ./...

cov:  ## Produce html coverage report
	go tool cover -html=cover.out
