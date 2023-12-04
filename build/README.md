# Build scripts and resources

# Testing scripts

| File                                                                                 | Runs In                                                 | Description                                                                                                 |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| [run-e2e-tests.sh](./run-e2e-tests.sh)                                               | [GRC Integration](../.github/workflows/integration.yml) | Run Framework E2E tests on Kind using Makefile commands                                                     |
| [run-e2e-tests-policy-framework.sh](./run-e2e-tests-policy-framework.sh)             |                                                         | Run the containerized Framework E2E tests via `docker run`                                                  |
| [run-e2e-tests-policy-framework-prow.sh](./run-e2e-tests-policy-framework-prow.sh)   | Prow                                                    | Run Framework E2E tests tailored to Prow (i.e. using Prow env vars)                                         |
| [run-test-image.sh](./run-test-image.sh)                                             | Canaries                                                | Script run in the Framework Docker container (commands similar to `run-e2e-tests-policy-framework-prow.sh`) |
| [periodic.sh](./periodic.sh)                                                         | [GRC CI Check](../.github/workflows/repo-config.yml)    | Verify that all essential CI jobs are passing and syncing is unblocked                                      |
| [codebase-check.sh](./codebase-check.sh)                                             | [GRC CI Check](../.github/workflows/repo-config.yml)    | Verify that repos are using consistent versions, CI, and CRDs                                               |

# Utility scripts

| Folder                                    | Description                                                                                          |
| ----------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| [branch-create/](./branch-create/)        | Scripts for cutting new release branches and configurations                                          |
| [main-branch-sync/](./main-branch-sync/)  | Scripts for maintaining cross-repo `main` branches and fast-forwarding to the current release branch |
| [detect-modules.sh](./detect-modules.sh) | Script to build binaries and check for Go modules in the built binary across all repos               |
