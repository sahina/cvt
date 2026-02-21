# Releasing

This guide covers the CVT release process, including stable releases and pre-releases.

## Overview

CVT uses [Semantic Versioning](https://semver.org/) for releases:

- **MAJOR.MINOR.PATCH** (e.g., `1.2.3`) for stable releases
- **MAJOR.MINOR.PATCH-PRERELEASE** (e.g., `1.2.3-rc.1`) for pre-releases

When a version tag is pushed, the release workflow automatically:

1. Builds multi-platform Docker images (`linux/amd64`, `linux/arm64`)
2. Pushes to GitHub Container Registry (`ghcr.io/sahina/cvt`)
3. Creates a GitHub Release with auto-generated release notes

## Stable Releases

Stable releases update the `:latest` Docker tag and are marked as the latest release on GitHub.

```bash
# Create and push a stable release
make release TAG=1.0.0
```

This will:

- Create git tag `v1.0.0`
- Push the tag to origin
- Trigger the release workflow
- Push Docker images: `:1.0.0` and `:latest`
- Create a GitHub Release

## Pre-Releases

Pre-releases are for alpha, beta, and release candidate versions. They do **not** update the `:latest` Docker tag and are marked as pre-release on GitHub.

### Pre-Release Types

| Type              | Example         | Use Case                         |
| ----------------- | --------------- | -------------------------------- |
| Alpha             | `1.0.0-alpha.1` | Early development, unstable      |
| Beta              | `1.0.0-beta.1`  | Feature complete, testing        |
| Release Candidate | `1.0.0-rc.1`    | Ready for release, final testing |

### Creating Pre-Releases

```bash
# Alpha release
make prerelease TAG=1.0.0-alpha.1

# Beta release
make prerelease TAG=1.0.0-beta.1

# Release candidate
make prerelease TAG=1.0.0-rc.1
```

This will:

- Create git tag `v1.0.0-rc.1`
- Push the tag to origin
- Trigger the release workflow
- Push Docker image: `:1.0.0-rc.1` only (no `:latest` update)
- Create a GitHub Pre-Release

## Release Workflow

### Prerequisites

1. Ensure all tests pass on `main` branch
2. Ensure the changes you want to release are merged to `main`
3. Pull the latest `main` branch locally

### Step-by-Step Process

```bash
# 1. Switch to main and pull latest
git checkout main
git pull origin main

# 2. Create the release
make release TAG=1.0.0

# 3. Monitor the release workflow
# Visit: https://github.com/sahina/cvt/actions/workflows/release.yml
```

### Typical Release Cycle

```bash
# Development phase - alpha releases
make prerelease TAG=1.0.0-alpha.1
make prerelease TAG=1.0.0-alpha.2

# Testing phase - beta releases
make prerelease TAG=1.0.0-beta.1
make prerelease TAG=1.0.0-beta.2

# Final testing - release candidates
make prerelease TAG=1.0.0-rc.1
make prerelease TAG=1.0.0-rc.2

# Stable release
make release TAG=1.0.0
```

## SDK Publishing

The release workflow automatically publishes all SDKs to their public registries:

| SDK     | Registry                                                                        | Mechanism                                 |
| ------- | ------------------------------------------------------------------------------- | ----------------------------------------- |
| Node.js | [npmjs](https://www.npmjs.com/package/@sahina/cvt-sdk)                          | `npm publish` with `NPM_TOKEN` secret     |
| Java    | [Maven Central](https://central.sonatype.com/artifact/io.github.sahina/cvt-sdk) | `mvn deploy -Prelease` with GPG signing   |
| Python  | [PyPI](https://pypi.org/project/cvt-sdk/)                                       | Trusted publisher (OIDC, no token needed) |
| Go      | `go get`                                                                        | Git tag `sdks/go/vX.Y.Z` pushed to origin |

All SDK jobs run in parallel after `validate`, so they don't block each other.

## Docker Images

Released images are available at:

```text
ghcr.io/sahina/cvt:latest     # Latest stable release
ghcr.io/sahina/cvt:1.0.0      # Specific version
ghcr.io/sahina/cvt:1.0.0-rc.1 # Pre-release version
```

### Pulling Images

```bash
# Latest stable
docker pull ghcr.io/sahina/cvt:latest

# Specific version
docker pull ghcr.io/sahina/cvt:1.0.0

# Pre-release
docker pull ghcr.io/sahina/cvt:1.0.0-rc.1
```

### Platform Support

All images are built for multiple platforms:

- `linux/amd64` - Standard x86-64 servers, CI runners
- `linux/arm64` - ARM servers (AWS Graviton, Apple Silicon)

## Makefile Commands

| Command                            | Description                       |
| ---------------------------------- | --------------------------------- |
| `make tag TAG=x.y.z`               | Create a local git tag            |
| `make tag-push TAG=x.y.z`          | Create and push a git tag         |
| `make release TAG=x.y.z`           | Alias for `tag-push`              |
| `make prerelease TAG=x.y.z-suffix` | Create and push a pre-release tag |
| `make check-release`               | Show the current release version  |

## Troubleshooting

### Release workflow failed

1. Check the [Actions tab](https://github.com/sahina/cvt/actions/workflows/release.yml) for error details
2. Common issues:
   - Docker build failure: Check Dockerfile syntax
   - Permission denied: Ensure `GITHUB_TOKEN` has `packages:write` permission

### Tag already exists

If you need to re-release the same version:

```bash
# Delete local tag
git tag -d v1.0.0

# Delete remote tag
git push origin :refs/tags/v1.0.0

# Re-create the release
make release TAG=1.0.0
```

### Verify image was pushed

```bash
# Check if image exists
docker manifest inspect ghcr.io/sahina/cvt:1.0.0
```
