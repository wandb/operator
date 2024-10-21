# Makefile for building and pushing the WandB Operator image

# Define variables
IMAGE ?= wandb-operator                       # Name of the Docker image
QUAY_DOCKER_REGISTRY ?= quay.io                # Docker registry URL
QUAY_PROJECT ?= wandb_tools                   # Project name in the registry
VERSION ?= latest                              # Default version for the image
BRANCH ?= main                                 # Default branch for tagging

# Targets

# Build the Docker image
build-image:
	docker build -t $(QUAY_DOCKER_REGISTRY)/$(QUAY_PROJECT)/$(IMAGE):$(VERSION) \
	-f olm-catalog/bundle.Dockerfile olm-catalog/
	
	docker tag $(QUAY_DOCKER_REGISTRY)/$(QUAY_PROJECT)/$(IMAGE):$(VERSION) \
	$(QUAY_DOCKER_REGISTRY)/$(QUAY_PROJECT)/$(IMAGE):$(BRANCH)

# Build and push the Docker image to the registry
build-push:
	docker push $(QUAY_DOCKER_REGISTRY)/$(QUAY_PROJECT)/$(IMAGE):$(VERSION)
	docker push $(QUAY_DOCKER_REGISTRY)/$(QUAY_PROJECT)/$(IMAGE):$(BRANCH)

# Specify that these targets do not represent files
.PHONY: build-image build-push
