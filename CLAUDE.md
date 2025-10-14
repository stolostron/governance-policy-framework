# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this
repository.

## Repository Overview

This is the **Governance Policy Framework** for Open Cluster Management (OCM). It is a test-only
repository that validates policy distribution and enforcement across hub and managed clusters. The
framework distributes `Policy` Custom Resources from a hub cluster to managed clusters and collects
compliance results.

**Key architectural components:**

- **Hub cluster**: Where policies are created and compliance results are aggregated
- **Managed cluster(s)**: Where policies are distributed and evaluated by policy controllers
- **Policy propagator**: Replicates policies from hub user namespaces to cluster namespaces, then to
  managed clusters
- **Policy framework addon**: Synchronizes policies between hub and managed clusters
- **Policy controllers**: Run on managed clusters to evaluate policies (config-policy-controller,
  cert-policy-controller)

## Development Commands

### Setting Up Local KinD Environment

Create two KinD clusters (hub and managed) with the full policy framework:

```bash
make kind-bootstrap-cluster
```

Create clusters without deploying controllers (for development):

```bash
make kind-bootstrap-cluster-dev
```

Delete KinD clusters:

```bash
make kind-delete-cluster
```

### Testing

Run e2e tests (requires KinD clusters):

```bash
make e2e-test
```

Run integration tests (requires actual OCP cluster with credentials):

```bash
make integration-test
```

Run a specific test file:

```bash
TEST_FILE=<filename> make e2e-test
# Example: TEST_FILE=cert_policy_test.go make e2e-test
```

Run tests in hosted mode (hub and managed on same cluster):

```bash
make e2e-test-hosted
```

### Linting and Formatting

Format Go code:

```bash
make fmt
```

Lint code:

```bash
make lint
```

### Debugging

Collect debug logs from hub cluster:

```bash
make e2e-debug-hub
```

Collect debug logs from managed cluster:

```bash
make e2e-debug-managed
```

Collect all debug logs (hub + managed):

```bash
make e2e-debug
```

For KinD clusters specifically:

```bash
make e2e-debug-kind
```

Debug output is stored in `test-output/debug/` directory.

### Cleaning

Remove generated files and KinD clusters:

```bash
make clean
```

## Architecture & Code Structure

### Test Organization

- **test/e2e/**: End-to-end tests that verify the policy framework works across hub and managed
  clusters
- **test/integration/**: Integration tests that validate policy scenarios from policy-collection
- **test/common/**: Shared test utilities and helpers used by both e2e and integration tests
- **test/resources/**: YAML manifests and test fixtures

### Key Test Patterns

Tests use Ginkgo/Gomega and follow this pattern:

1. **Client setup**: Tests use multiple Kubernetes clients

   - `ClientHub`: Kubernetes client for hub cluster
   - `ClientManaged`: Kubernetes client for managed cluster
   - `ClientHubDynamic`: Dynamic client for hub cluster (for CRDs)
   - `ClientManagedDynamic`: Dynamic client for managed cluster (for CRDs)
   - `ClientHosting/ClientHostingDynamic`: Points to hub in hosted mode, managed otherwise

2. **Namespace organization**:

   - `UserNamespace` (default: `policy-test`): Hub namespace where root policies are created
   - `ClusterNamespace` (default: `managed`): Represents the managed cluster name
   - Policies are replicated from user namespace → cluster namespace on hub → managed cluster

3. **Policy creation flow**:
   - Create `Policy` CR in user namespace on hub
   - Create `Placement` CR to select target clusters
   - Create `PlacementBinding` CR to bind policy to placement
   - Policy propagator replicates to cluster namespace
   - Framework addon distributes to managed cluster
   - Policy controller on managed cluster evaluates and reports status back to hub

### KinD Cluster Configuration

The Makefile sets up two KinD clusters with these defaults:

- Hub cluster: `kind-hub` (namespace: `open-cluster-management`)
- Managed cluster: `kind-managed` (namespace: `open-cluster-management-agent-addon`)

Kubeconfig files are generated:

- `kubeconfig_hub` / `kubeconfig_hub_e2e`: For hub cluster access
- `kubeconfig_managed` / `kubeconfig_managed_e2e`: For managed cluster access
- `kubeconfig_hub_internal`: Internal hub address for managed→hub communication

### Important Environment Variables

- `MANAGED_CLUSTER_NAME`: Name of the managed cluster (default: `managed`)
- `HUB_CLUSTER_NAME`: Name of the hub cluster (default: `hub`)
- `KIND_HUB_NAMESPACE`: Namespace for hub components (default: `open-cluster-management`)
- `KIND_MANAGED_NAMESPACE`: Namespace for managed components (default:
  `open-cluster-management-agent-addon`)
- `deployOnHub`: Set to `true` to run managed cluster on hub (hosted mode)
- `RELEASE_BRANCH`: Branch to use for component images (default: `main`)
- `VERSION_TAG`: Image tag to use (default: `latest`)

### Test Utilities (test/common/)

Key functions in `test/common/common.go`:

- `InitInterfaces(hubConfig, managedConfig, isHosted)`: Initialize Kubernetes clients
- `OcHub(args...)`: Run kubectl/oc commands against hub cluster
- `OcManaged(args...)`: Run kubectl/oc commands against managed cluster
- `IsAtLeastVersion(minVersion)`: Check if OCP cluster meets minimum version
- `ApplyManagedClusterSetBinding(ctx)`: Create ManagedClusterSetBinding for tests

Key functions in `test/common/policy_utils.go`:

- Policy creation, deletion, and status checking utilities
- Placement and PlacementBinding helpers

### Hosted Mode

Hosted mode runs a managed cluster whose multi-cluster control plane components (i.e. the
Klusterlet) run on a separate hosting cluster (for the tests run here, the hub cluster also acts as
the hosting cluster). This is used for:

- Testing hub-as-managed scenarios
- Resource-constrained environments

To set up hosted mode for testing:

```bash
make kind-bootstrap-hosted
```

### Common Build Files

The repository uses shared Makefiles from `build/common/`:

- `Makefile.common.mk`: Provides common targets like linting, formatting, dependency installation
- Versions for tools (golangci-lint, ginkgo, kustomize, etc.) are defined here

## Policy Custom Resources

The framework works with these CRDs:

1. **Policy** (`policy.open-cluster-management.io/v1`): Container for policy templates
2. **PlacementBinding**: Binds policies to placements
3. **Placement** (`cluster.open-cluster-management.io/v1beta1`): Selects target clusters
4. **PolicySet**: Groups related policies
5. **PolicyAutomation**: Defines automated remediation actions

Policy templates wrapped by Policy CR:

- **ConfigurationPolicy**: Ensures Kubernetes resources match desired state
- **CertificatePolicy**: Checks certificate expiration
- **OperatorPolicy**: Manages OLM operators

## Git Workflow

All commits must be signed off with DCO:

```bash
git commit --signoff -m "Your commit message"
```

This repository uses fast-forward merging to the main branch when integration tests pass (controlled
by `FAST_FORWARD` variable).
