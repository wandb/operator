# VERSION defaults to the Helm chart appVersion (no leading v).
VERSION ?= $(shell sed -n 's/^appVersion: *"\(.*\)"/\1/p' deploy/operator/Chart.yaml)
CHANNELS ?= stable
DEFAULT_CHANNEL ?= stable
BUNDLE_METADATA_OPTS ?= --channels=$(CHANNELS) --default-channel=$(DEFAULT_CHANNEL)

IMAGE_TAG_BASE ?= us-docker.pkg.dev/wandb-production/public/wandb/operator
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)
# Root Makefile defaults IMG to controller:latest; use the public image unless overridden.
ifeq ($(origin IMG),file)
BUNDLE_CONTROLLER_IMG := $(IMAGE_TAG_BASE):$(VERSION)
else ifeq ($(IMG),controller:latest)
BUNDLE_CONTROLLER_IMG := $(IMAGE_TAG_BASE):$(VERSION)
else
BUNDLE_CONTROLLER_IMG := $(IMG)
endif

BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) --package wandb-operator $(BUNDLE_METADATA_OPTS)

# OpenShift floor from CRC 2.60.1 (OpenShift 4.21.8).
OPENSHIFT_VERSIONS ?= v4.21
BUNDLE_REPLACES ?= wandb-operator.v1.22.0
COMMUNITY_BUNDLE_DIR ?= dist/community-operators/operators/wandb-operator/$(VERSION)

.PHONY: bundle
bundle: manifests kustomize ## Generate bundle manifests and metadata, then validate generated files.
	@command -v operator-sdk >/dev/null 2>&1 || { \
		echo "Error: operator-sdk is required. Install it from https://sdk.operatorframework.io/docs/installation/"; \
		exit 1; \
	}
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(BUNDLE_CONTROLLER_IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle $(BUNDLE_GEN_FLAGS)
	printf 'resources:\n- manager.yaml\n' > config/manager/kustomization.yaml
	operator-sdk bundle validate ./bundle

.PHONY: bundle-community
bundle-community: bundle ## Stamp community-operators-prod metadata and stage the version directory.
	@command -v python3 >/dev/null 2>&1 || { echo "Error: python3 is required."; exit 1; }
	python3 hack/scripts/prepare-community-bundle.py \
		--bundle-dir ./bundle \
		--dependencies config/olm/community/dependencies.yaml \
		--openshift-versions $(OPENSHIFT_VERSIONS) \
		--replaces $(BUNDLE_REPLACES) \
		--container-image $(BUNDLE_CONTROLLER_IMG) \
		--stage-dir $(COMMUNITY_BUNDLE_DIR)
	operator-sdk bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)
