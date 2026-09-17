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
check-vulnerabilities: govulncheck
	$(GOVULNCHECK) ./...

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