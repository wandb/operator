# Multi-Binary Project Structure

This project supports multiple binaries through a structured approach.

## Directory Structure

```
operator/
├── cmd/
│   ├── controller/      # Main controller binary
│   │   └── main.go
│   ├── canary/         # Canary connectivity tester
│   │   └── main.go
│   ├── cli/            # Future: CLI tool (example)
│   │   └── main.go
│   └── webhook/        # Future: Webhook server (example)
│       └── main.go
├── Dockerfile.controller # Dockerfile for controller binary
├── Dockerfile.canary    # Dockerfile for canary binary
├── Dockerfile.cli       # Future: Dockerfile for CLI binary
├── Dockerfile.webhook   # Future: Dockerfile for webhook binary
├── Dockerfile.template  # Template for new Dockerfiles
└── Makefile
```

## Adding a New Binary

### 1. Create the Binary Source

Create a new directory under `cmd/` with your binary name:

```bash
mkdir -p cmd/mybinary
```

Create `cmd/mybinary/main.go`:

```go
package main

import (
    "fmt"
    "os"
)

func main() {
    fmt.Println("My Binary")
    os.Exit(0)
}
```

### 2. Create the Dockerfile

Copy the template:

```bash
cp Dockerfile.template Dockerfile.mybinary
```

Edit `Dockerfile.mybinary` and replace:
- `binary-name` with `mybinary`
- `cmd/binary-name/main.go` with `cmd/mybinary/main.go`

### 3. Update the Makefile

Add build targets for your binary:

```makefile
# Add to the build section
.PHONY: build-mybinary
build-mybinary: ## Build mybinary binary.
	go build -o bin/mybinary cmd/mybinary/main.go

# Update the main build target to include your binary
.PHONY: build
build: build-controller build-mybinary ## Build all binaries.
```

Add docker targets:

```makefile
# Add image variable at the top of Makefile
MYBINARY_IMG ?= mybinary:latest

# Add docker build target
.PHONY: docker-build-mybinary
docker-build-mybinary: ## Build mybinary docker image.
	$(CONTAINER_TOOL) build -t ${MYBINARY_IMG} -f Dockerfile.mybinary .

# Update main docker-build to include your binary
.PHONY: docker-build
docker-build: docker-build-controller docker-build-mybinary ## Build all docker images.

# Add docker push target
.PHONY: docker-push-mybinary
docker-push-mybinary: ## Push mybinary docker image.
	$(CONTAINER_TOOL) push ${MYBINARY_IMG}

# Update main docker-push to include your binary
.PHONY: docker-push
docker-push: docker-push-controller docker-push-mybinary ## Push all docker images.
```

### 4. Build and Test

Build the binary:
```bash
make build-mybinary
./bin/mybinary
```

Build the Docker image:
```bash
make docker-build-mybinary
docker run --rm ${MYBINARY_IMG}
```

## Current Binaries

### Controller (`cmd/controller/main.go`)
- **Purpose**: Main Kubernetes operator controller
- **Binary**: `bin/manager`
- **Image**: Controlled by `CONTROLLER_IMG` variable (default: `controller:latest`)
- **Dockerfile**: `Dockerfile.controller`
- **Build**: `make build-controller` or `make build`
- **Docker Build**: `make docker-build-controller` or `make docker-build`

### Canary (`cmd/canary/main.go`)
- **Purpose**: Infrastructure connectivity testing (MySQL, Redis, MinIO, Kafka, ClickHouse)
- **Binary**: `bin/canary`
- **Image**: Controlled by `CANARY_IMG` variable (default: `wandb-canary:latest`)
- **Dockerfile**: `Dockerfile.canary`
- **Build**: `make build-canary` or `make build`
- **Docker Build**: `make docker-build-canary` or `make docker-build`

## Future Binaries (Examples)

### CLI Tool (`cmd/cli/main.go`)
- **Purpose**: Command-line interface for managing the operator
- **Binary**: `bin/wandb-cli`
- **Image**: `CLI_IMG` variable
- **Dockerfile**: `Dockerfile.cli`

### Webhook Server (`cmd/webhook/main.go`)
- **Purpose**: Admission webhook server for validation/mutation
- **Binary**: `bin/webhook`
- **Image**: `WEBHOOK_IMG` variable
- **Dockerfile**: `Dockerfile.webhook`

## Why This Pattern?

1. **Standard Practice**: Follows Kubernetes ecosystem conventions
2. **Clear Separation**: Each binary is self-contained
3. **Easy Discovery**: All Dockerfiles visible at project root
4. **Tool Compatibility**: Works with Docker, Make, Tilt, and CI/CD
5. **Scalable**: Easy to add new binaries without conflicts

## Tilt Development

For local development with Tilt, you can add custom binaries by:

1. Creating a new local_resource in `Tiltfile`
2. Using docker_build() with the appropriate Dockerfile

Example:
```python
docker_build('mybinary:latest', '.',
    dockerfile='Dockerfile.mybinary',
    only=['./bin/mybinary'])
```
