---
name: cvt-ci
description: Integrate CVT contract validation into CI/CD pipelines
sdk_version: "0.6.1"
---

# CVT CI/CD Integration

## Prerequisites

- Contract tests already written and passing locally (run `/cvt-write-contract` first)
- Access to a CI/CD platform (GitHub Actions, GitLab CI, or Jenkins)
- Either a shared CVT server address or Docker available in CI for a sidecar

## Goal

Add CVT contract validation to the CI/CD pipeline so that contract tests run on every push or pull request, and optionally gate deployments with `can-i-deploy` checks.

## Reference Templates

The CVT repository includes ready-to-use CI templates in the `ci-templates/` directory at the repository root. These are the source of truth for CI configuration:

| File | Platform | Purpose |
|---|---|---|
| `ci-templates/demo-consumer.yml` | GitHub Actions | Consumer contract test workflow |
| `ci-templates/demo-producer.yml` | GitHub Actions | Producer validation workflow |
| `ci-templates/demo-can-i-deploy.yml` | GitHub Actions | Deployment gate workflow |
| `ci-templates/Jenkinsfile` | Jenkins | Full pipeline example |
| `ci-templates/README.md` | All | Detailed integration guide |

Always refer to `ci-templates/` for the latest patterns. Copy and adapt rather than writing from scratch.

## Steps

1. **Choose a server strategy**: shared CVT server or Docker sidecar (see below).
2. **Copy the appropriate template** from `ci-templates/` into your project.
3. **Add the contract test step** to run your existing contract tests.
4. **Configure secrets** for API key authentication if the server requires it.
5. **Optionally add a `can-i-deploy` gate** before production deployments.
6. **Test the pipeline** with a pull request.

## Server Strategy

### Option A: Shared CVT Server

If your team runs a shared CVT server, configure the SDK to connect to it:

```yaml
env:
  CVT_SERVER: cvt.internal.example.com:9550
```

### Option B: Docker Sidecar (No Shared Server)

For teams without a shared server, run CVT as a sidecar service in CI:

```yaml
# GitHub Actions example
services:
  cvt:
    image: ghcr.io/sahina/cvt:latest
    ports:
      - 9550:9550
```

Before running tests, wait for the server to be ready:

```bash
# Install and use cvt wait
go install github.com/sahina/cvt/cmd/cvt@latest
cvt wait --server localhost:9550 --timeout 60

# Or use a simple health check loop
for i in $(seq 1 30); do
  grpc_health_probe -addr=localhost:9550 && break
  sleep 2
done
```

## SDK-Specific Instructions

### Node.js

```yaml
# GitHub Actions
- name: Install dependencies
  run: npm ci

- name: Run contract tests
  run: npm test -- --testPathPattern=contract
  env:
    CVT_SERVER: localhost:9550
```

### Python

```yaml
# GitHub Actions
- name: Install dependencies
  run: pip install -e ".[test]"

- name: Run contract tests
  run: pytest tests/contract/ -v
  env:
    CVT_SERVER: localhost:9550
```

### Go

```yaml
# GitHub Actions
- name: Run contract tests
  run: go test ./contract/... -v
  env:
    CVT_SERVER: localhost:9550
```

### Java

```yaml
# GitHub Actions
- name: Run contract tests
  run: mvn test -Dtest="*Contract*"
  env:
    CVT_SERVER: localhost:9550
```

## API Key Authentication in CI

If the CVT server requires API key authentication, store the key as a CI secret:

**GitHub Actions:**
```yaml
- name: Run contract tests
  run: npm test -- --testPathPattern=contract
  env:
    CVT_API_KEY: ${{ secrets.CVT_API_KEY }}
```

**GitLab CI:**
```yaml
contract_tests:
  script:
    - npm test -- --testPathPattern=contract
  variables:
    CVT_API_KEY: $CVT_API_KEY  # Set in GitLab CI/CD Settings > Variables
```

**Jenkins:**
```groovy
withCredentials([string(credentialsId: 'cvt-api-key', variable: 'CVT_API_KEY')]) {
    sh 'npm test -- --testPathPattern=contract'
}
```

Never hard-code API keys in workflow files or commit them to the repository.

## Deployment Gate with can-i-deploy

Add a deployment gate that checks whether a schema change will break registered consumers:

```yaml
# GitHub Actions
- name: Check deployment safety
  run: |
    cvt can-i-deploy \
      --schema my-api \
      --version ${{ github.sha }} \
      --env production \
      --server ${{ vars.CVT_SERVER }}
```

For JSON output in scripting contexts:
```bash
cvt can-i-deploy --schema my-api --version 2.0.0 --env prod --json
```

The `can-i-deploy` command exits with code 1 if deployment is unsafe, which will fail the CI step.

## Full Pipeline Example (GitHub Actions)

```yaml
name: Contract Validation

on: [push, pull_request]

jobs:
  contract-tests:
    runs-on: ubuntu-latest
    services:
      cvt:
        image: ghcr.io/sahina/cvt:latest
        ports:
          - 9550:9550
    steps:
      - uses: actions/checkout@v4

      - name: Wait for CVT server
        run: |
          go install github.com/sahina/cvt/cmd/cvt@latest
          cvt wait --server localhost:9550 --timeout 60

      - name: Run contract tests
        run: npm ci && npm test -- --testPathPattern=contract
        env:
          CVT_SERVER: localhost:9550

  deploy-gate:
    runs-on: ubuntu-latest
    needs: contract-tests
    if: github.ref == 'refs/heads/main'
    steps:
      - name: Check can-i-deploy
        run: |
          cvt can-i-deploy \
            --schema my-api \
            --version ${{ github.sha }} \
            --env production \
            --server ${{ vars.CVT_SERVER }}
```

## Common Errors

| Error | Cause | Fix |
|---|---|---|
| `connection refused` in CI | CVT sidecar not ready | Add `cvt wait` step before running tests |
| `authentication required` | Server has API key auth enabled | Add `CVT_API_KEY` secret to CI platform |
| `image not found: ghcr.io/sahina/cvt` | Docker image not accessible | Ensure CI runner can pull from GHCR; add `GITHUB_TOKEN` if needed |
| `can-i-deploy` exits 1 | Breaking changes detected | Review the output; fix the schema or coordinate with consumers |
| `schema not registered` in `can-i-deploy` | Schema was never registered on the shared server | Register the schema first using `cvt register-schema` or the SDK |

## Success Criteria

- CI pipeline runs contract tests on every push or pull request
- Tests connect to a CVT server (shared or sidecar) and pass
- API keys are stored as CI secrets, not in code
- Optionally: `can-i-deploy` gate blocks unsafe deployments
- Pipeline configuration references patterns from `ci-templates/`
