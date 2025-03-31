.PHONY: safe-bump-deps
safe-bump-deps:
	go get -u ./...
	go mod tidy

.PHONY: major-bump-deps
major-bump-deps:
	go get $(shell go list -m all | grep -v "indirect" | grep -v "github.com/wandb/operator" | cut -d' ' -f1)@latest
	go mod tidy

.PHONY: find-deprecated
find-deprecated:
	@echo "Checking for deprecation warnings in dependencies..."
	@go list -u -m -json all | jq -r 'select(.Deprecated != null) | "\(.Path) - \(.Deprecated)"'
	@echo "Checking for deprecation notices in source code comments..."
	@grep -r --include="*.go" "Deprecated" . || echo "No deprecation notices found in comments."
	@echo "Checking build with extra deprecation warnings..."
	@go build -gcflags='-m -d=deprecation' ./... 2>&1 | grep -i "deprecated" || echo "No deprecation warnings during build."

.PHONY: check-vulnerabilities
check-vulnerabilities:
	@echo "Checking Go version compatibility..."
	@GO_VERSION=$$(go version | awk '{print $$3}' | sed 's/go//'); \
	GO_VERSION_REQUIRED=$$(grep -E "^go [0-9]+\.[0-9]+(\.[0-9]+)?" go.mod | awk '{print $$2}'); \
	if [ "$$(printf '%s\n' "$$GO_VERSION_REQUIRED" "$$GO_VERSION" | sort -V | head -n1)" != "$$GO_VERSION_REQUIRED" ]; then \
		echo "Error: This project requires Go $$GO_VERSION_REQUIRED but you have Go $$GO_VERSION"; \
		echo "Please upgrade your Go installation to at least $$GO_VERSION_REQUIRED before running this command."; \
		exit 1; \
	fi; \
	echo "Go version OK ($$GO_VERSION)"
	@echo "Installing/updating govulncheck..."
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "Checking for vulnerable dependencies..."
	@govulncheck ./...

.PHONY: list-outdated
list-outdated:
	@echo "Listing outdated direct dependencies..."
	@go list -u -m -json all | jq -r 'select(.Update != null and .Main != true and .Indirect != true) | "\(.Path): \(.Version) -> \(.Update.Version)"'

.PHONY: list-all-outdated
list-all-outdated:
	@echo "Listing ALL outdated dependencies (including transitive dependencies)..."
	@go list -u -m -json all | jq -r 'select(.Update != null) | "\(.Path): \(.Version) -> \(.Update.Version) \(if .Indirect then "(indirect)" else "" end)"'

.PHONY: clean-mod-cache
clean-mod-cache:
	go clean -modcache