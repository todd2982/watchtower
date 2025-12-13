# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Watchtower is an automated Docker container base image update tool written in Go. It monitors running Docker containers and automatically updates them when their base images change in the registry. The tool is designed for homelabs, media centers, and local dev environments - NOT for production/commercial use (Kubernetes is recommended for that).

**Repository:** github.com/todd2982/watchtower
**Documentation:** https://todd2982.dev/watchtower

## Development Commands

### Building

```bash
# Local build with version injection
./build.sh                    # Creates 'watchtower' or 'watchtower.exe' binary

# Direct Go build (no version)
go build

# Docker builds
docker build -f dockerfiles/Dockerfile.dev-self-contained -t todd2982/watchtower .    # From local files
docker build -f dockerfiles/Dockerfile.self-contained -t todd2982/watchtower .       # From GitHub
```

### Testing

```bash
# Run all tests
go test ./... -v

# Run tests with coverage
go test ./... -coverprofile coverage.out

# View coverage
go tool cover -html=coverage.out
```

The project uses **Ginkgo** (BDD testing framework) and **Gomega** (matchers). Test files follow the `*_test.go` naming convention and are located alongside the code they test in `pkg/` and `internal/` directories.

### Running Watchtower

```bash
# Run locally built binary
./watchtower --run-once        # Single update run
./watchtower                   # Scheduled updates (default: every 5 minutes)

# With Docker
docker run --rm \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    todd2982/watchtower --run-once
```

## Architecture

### Directory Structure

- **`cmd/`** - CLI commands using Cobra framework
  - `root.go` - Main command with scheduler and Docker client setup
  - `notify-upgrade.go` - Notification upgrade command
- **`internal/`** - Private application logic (not importable by external packages)
  - `actions/` - Core business logic: update orchestration, container checking
  - `flags/` - Environment variable and flag parsing
  - `meta/` - Version metadata (injected at build time)
- **`pkg/`** - Public/reusable packages
  - `api/` - HTTP API endpoints for manual triggers and metrics
  - `container/` - Docker client abstraction and container operations
  - `filters/` - Container selection and filtering logic
  - `lifecycle/` - Pre/post update hook execution
  - `metrics/` - Prometheus metrics collection
  - `notifications/` - Multi-channel notification system (using Shoutrrr)
  - `registry/` - Docker registry authentication and operations
  - `session/` - Update session state tracking
  - `sorter/` - Dependency-based container start/stop ordering
  - `types/` - Shared interfaces and type definitions

### Execution Flow

1. **Entry:** `main.go` → `cmd.Execute()` → `cmd/root.go`
2. **Initialization:** Parse flags → Create Docker client → Setup notifiers
3. **Run Mode:**
   - **One-time (`--run-once`):** Execute updates immediately and exit
   - **Scheduled (default):** Start cron scheduler for periodic updates
   - **HTTP API (`--http-api-update`):** Expose `/v1/update` endpoint
4. **Update Process (`internal/actions/update.go`):**
   - Execute pre-check lifecycle hooks (if enabled)
   - List containers matching filters
   - Check each container for stale images
   - Sort by dependencies (using labels/networks)
   - Stop containers in reverse dependency order
   - Start containers in dependency order with same configuration
   - Cleanup old images (if `--cleanup` enabled)
   - Execute post-check lifecycle hooks
   - Send notifications with update report

### Key Design Patterns

- **Client Interface:** `pkg/container/Client` abstracts Docker API for testability
- **Filter Pattern:** Flexible container selection via label/name/scope filters in `pkg/filters/`
- **Strategy Pattern:** Pluggable notification backends, warning strategies
- **Dependency Injection:** Configuration passed through `UpdateParams` and `ClientOptions` structs

### Configuration

Configuration is loaded via **Viper** with the following precedence (highest to lowest):
1. Command-line flags
2. Environment variables (prefixed with `WATCHTOWER_`)
3. Config files (if specified)

All CLI flags have corresponding environment variables. See `internal/flags/` for mappings.

## Important Implementation Notes

### Docker Client

- The `pkg/container/Client` interface wraps the Docker SDK
- Always access Docker operations through this interface, not the SDK directly
- Mock implementations exist in `pkg/container/mocks/` for testing
- Test data fixtures are in `pkg/container/mocks/data/*.json`

### Container Lifecycle

- Containers are restarted with their original configuration (labels, env vars, volumes, networks)
- The `VerifyConfiguration()` method ensures containers can be safely updated
- Linked containers are deprecated; use networks instead

### Dependency Ordering

- Container start/stop order is determined by `pkg/sorter/` based on:
  - `com.centurylinklabs.watchtower.depends-on` labels
  - Shared networks
  - Container links (deprecated)
- Stopping happens in reverse dependency order to avoid breaking running services

### Version Injection

The version string is injected at build time via ldflags:
```bash
-ldflags "-X github.com/containrrr/watchtower/internal/meta.Version=$VERSION"
```

Access the version using `internal/meta.Version` - do not hardcode version strings elsewhere.

### Logging

- Uses **logrus** for structured logging
- Log level is configured via `--log-level` flag
- JSON formatting available with `--log-format json`

## CI/CD

### GitHub Actions Workflows

- **`.github/workflows/release.yml`** - Production releases on `v*.*.*` tags
  - Multi-platform builds (Linux, Windows: amd64, 386, arm, arm64)
  - Docker images published to Docker Hub and GHCR
  - Uses goreleaser for build orchestration
- **`.github/workflows/release-dev.yaml`** - Development releases
- **`.github/workflows/publish-docs.yml`** - MkDocs documentation deployment
- **`.github/workflows/pull-request.yml`** - PR validation and testing

### Release Process

Releases are automated via **goreleaser** (`goreleaser.yml`):
- Triggered by pushing version tags: `git tag v1.2.3 && git push --tags`
- Builds binaries for multiple platforms
- Creates Docker images with multi-arch manifests
- Publishes to GitHub releases, Docker Hub (`todd2982/watchtower`), and GHCR

## Testing Considerations

- **BDD Style:** Tests use Ginkgo's `Describe`/`Context`/`It` structure
- **Mocking:** Docker client is mocked for unit tests; see `pkg/container/mocks/`
- **Test Data:** Mock container data in `pkg/container/mocks/data/` as JSON fixtures
- **Coverage:** Tracked via Codecov; aim to maintain or improve coverage with changes

## Common Gotchas

1. **Windows Paths:** The codebase must work cross-platform; test path handling on Windows
2. **Docker Socket Permissions:** Watchtower requires access to `/var/run/docker.sock`
3. **Image Digests:** Updates are detected by comparing image digests, not tags
4. **Time Zones:** Container timezone data is included in the `FROM scratch` final image
5. **Health Checks:** The binary supports `--health-check` flag for container health probes
