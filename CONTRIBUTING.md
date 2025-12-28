## Prerequisites

To contribute code changes to this project you will need the following development kits.

 * [Go](https://golang.org/doc/install)
 * [Docker](https://docs.docker.com/engine/installation/)
 
As watchtower utilizes go modules for vendor locking, you'll need at least Go 1.11.
You can check your current version of the go language as follows:

```bash
  ~ $ go version
  go version go1.12.1 darwin/amd64
```


## Checking out the code

Do not place your code in the go source path.

```bash
git clone git@github.com:<yourfork>/watchtower.git
cd watchtower
```

## Building and testing

watchtower is a go application and is built with go commands. The following commands assume that you are at the root level of your repo.

```bash
go build                               # compiles and packages an executable binary, watchtower
go test ./... -v                       # runs tests with verbose output
./watchtower                           # runs the application (outside of a container)
```

If you dont have it enabled, you'll either have to prefix each command with `GO111MODULE=on` or run `export GO111MODULE=on` before running the commands. [You can read more about modules here.](https://github.com/golang/go/wiki/Modules)

To build a Watchtower image of your own, use the self-contained Dockerfiles. As the main Dockerfile, they can be found in `dockerfiles/`:

* `dockerfiles/Dockerfile.dev-self-contained` will build an image based on your current local Watchtower files.
* `dockerfiles/Dockerfile.self-contained` will build an image based on current Watchtower's repository on GitHub.

e.g.:

```bash
sudo docker build . -f dockerfiles/Dockerfile.dev-self-contained -t todd2982/watchtower # to build an image from local files
```

## Commit Messages

This project follows [Conventional Commits](https://www.conventionalcommits.org/) for automated changelog generation. Using semantic commit messages helps create clear, user-friendly changelogs that are automatically published to GitHub Releases.

**Commit Message Format:**

```text
<type>: <description>

[optional body]
```

**Common Types:**

* `feat:` - New features (e.g., `feat: add support for custom registry certificates`)
* `fix:` - Bug fixes (e.g., `fix: resolve race condition in container restart logic`)
* `docs:` - Documentation changes (e.g., `docs: clarify --cleanup flag behavior`)
* `refactor:` - Code refactoring (e.g., `refactor: simplify container sorting logic`)
* `test:` - Test additions or changes (e.g., `test: add coverage for notification handlers`)
* `chore:` - Maintenance tasks (e.g., `chore: update dependencies`)
* `security:` - Security updates (e.g., `security: update Alpine base image to patch CVE-2024-XXXX`)

**Examples:**

```bash
feat: add support for webhook notifications
fix: prevent duplicate container updates
docs: add troubleshooting section to README
security: upgrade Go dependencies to address vulnerabilities
```

Good commit messages help generate clear changelogs that explain what changed and why, making it easier for users to understand each release.
