.PHONY: test-ginko
test-ginko: manifests generate fmt vet envtest ginkgo ## Run tests.
	@echo "Running tests..."
	@KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
	ginkgo -p -r --compilers=4 --timeout=5m --fail-fast --race --trace --randomize-all \
		--output-interceptor-mode=none \
		--no-color=false \
		--show-node-events \
		--json-report=report.json \
		--junit-report=junit.xml || exit 1
	@echo "All tests passed!"

.PHONY: test-verbose
test-verbose: manifests generate fmt vet envtest ginkgo ## Run tests with verbose output.
	@echo "Running tests in verbose mode..."
	@KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
	ginkgo -p -r -v --compilers=4 --timeout=5m --fail-fast --race --trace --randomize-all \
		--output-interceptor-mode=none \
		--no-color=false \
		--show-node-events \
		--json-report=report.json \
		--junit-report=junit.xml || exit 1
	@echo "All tests passed!"

.PHONY: test-watch
test-watch: manifests generate fmt vet envtest ginkgo ## Run tests in watch mode.
	@echo "Running tests in watch mode..."
	@KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
	ginkgo watch -p -r --compilers=4 --timeout=5m --fail-fast --race --trace --randomize-all \
		--output-interceptor-mode=none \
		--no-color=false \
		--show-node-events \
		--json-report=report.json \
		--junit-report=junit.xml || exit 1

.PHONY: test-coverage
test-coverage: manifests generate fmt vet envtest ginkgo ## Run tests with coverage report.
	@echo "Running tests with coverage..."
	@KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
	ginkgo -p -r --compilers=4 --timeout=5m --fail-fast --race --trace --randomize-all \
		--output-interceptor-mode=none \
		--no-color=false \
		--show-node-events \
		--json-report=report.json \
		--junit-report=junit.xml \
		--coverprofile=coverage.out || exit 1
	@echo "Generating coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"
