# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Watchtower is an automated Docker container base image update tool written in Go. It monitors running Docker containers and automatically updates them when their base images change in the registry. The tool is designed for homelabs, media centers, and local dev environments - NOT for production/commercial use (Kubernetes is recommended for that).

**Repository:** github.com/todd2982/watchtower

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
-ldflags "-X github.com/todd2982/watchtower/internal/meta.Version=$VERSION"
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
  - Docker images published to Docker Hub only
  - Uses goreleaser for build orchestration and automatic manifest creation
- **`.github/workflows/release-dev.yaml`** - Development releases on every push to main
  - Publishes single-arch `latest-dev` tag to Docker Hub
  - Fast builds for rapid iteration
- **`.github/workflows/pull-request.yml`** - PR validation and testing
  - Runs tests on all platforms (Linux, macOS, Windows)
  - Builds binaries and Docker images locally (without publishing)
  - Validates commit messages follow conventional commits format
  - Generates and posts changelog preview as PR comment

### Release Process

Releases are automated via **goreleaser** (`goreleaser.yml`):

- Triggered by pushing version tags: `git tag v1.2.3 && git push --tags`
- Builds binaries for multiple platforms (Linux, Windows: amd64, 386, arm, arm64)
- Creates 4 architecture-specific Docker images (amd64, i386, armhf, arm64v8)
- Automatically creates multi-arch manifests via `docker_manifests` section
- Publishes to GitHub releases and Docker Hub (`todd2982/watchtower`)
- Tags created: `{version}`, `latest`, `{arch}-{version}`, `{arch}-latest`

### Changelog Generation

Changelogs are automatically generated from semantic commit messages and published to GitHub Releases:

**Semantic Commit Prefixes:**

- `feat:` - New features (→ "Features" section)
- `fix:` - Bug fixes (→ "Bug Fixes" section)
- `security:` - Security updates (→ "Security Updates" section)
- `docs:` - Documentation (→ "Documentation" section)
- `chore(deps):` - Dependency updates (excluded from changelog)
- `test:` - Test changes (excluded from changelog)

**Automated Release Flow:**

When a version tag is pushed, goreleaser automatically:

1. Collects commits since the last tag
2. Groups commits by semantic type
3. Generates formatted changelog
4. Publishes changelog to GitHub Releases
5. Dependabot discovers changelog via `org.opencontainers.image.source` Docker label

**Example commit messages:**

```bash
feat: add support for custom registry certificates
fix: resolve race condition in container restart logic
security: update Alpine base image to patch CVE-2024-XXXX
docs: clarify --cleanup flag behavior in README
test: add security tests for execute command function
```

Good commit messages help generate clear, user-friendly changelogs.

### Commit Message Validation

The project uses **commitlint** to enforce conventional commit message format:

**Configuration:** `.commitlintrc.yml`

**Validation Rules:**

- Automated validation runs on all pull requests via GitHub Actions
- Checks commit messages follow conventional commits specification
- Ensures proper type prefixes (feat, fix, docs, security, test, etc.)
- Validates header length (max 100 characters)
- **IMPORTANT: Subject line must be lowercase/sentence-case** - capitalize only the first word after the type prefix
  - ✅ Correct: `test: add security tests for execute command function`
  - ❌ Wrong: `test: add security tests for ExecuteCommand function` (capitalized function name fails validation)
  - ✅ Correct: `fix: resolve issue in docker client`
  - ❌ Wrong: `fix: resolve issue in Docker Client` (capitalized words fail validation)
- Body paragraphs can use normal capitalization and proper nouns
- **IMPORTANT: Footer must have a leading blank line** - add a blank line before footers like Co-Authored-By or Generated with tags

**Local validation (optional):**

```bash
# Install commitlint and config
npm install --save-dev @commitlint/{cli,config-conventional}

# Validate last commit
npx commitlint --from HEAD~1 --to HEAD --verbose
```

**Changelog Preview:**

Pull requests automatically receive a comment showing what the changelog will look like when the PR is merged. This preview:

- Generates from commits in the PR compared to main branch
- Shows how commits will be categorized in the release changelog
- Updates automatically when new commits are pushed
- Helps reviewers verify changelog quality before merging

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
6. **Label Namespace:** All watchtower labels use the `com.centurylinklabs.watchtower` namespace from the original project for backward compatibility. This is intentional and should NOT be changed, as it would break existing user configurations.

## Fork Compatibility Notes

This fork (todd2982/watchtower) maintains API compatibility with the original watchtower project:

- **Docker Labels:** All configuration labels (`com.centurylinklabs.watchtower.*`) use the original namespace for backward compatibility
- **Breaking Changes:** Changes that would break compatibility with existing user configurations should be avoided
- **Documentation:** When adding features, maintain consistency with the original project's label conventions
