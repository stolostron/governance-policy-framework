# Running tests in this repository

## Test folders

 - `e2e`: Tests for functionality across all components
 - `policy-collection`: Tests for policies in the [policy-collection](https://github.com/stolostron/policy-collection) repository

## Cluster configuration

**Prerequisites**
- `go`
- `kubectl`
- `kind` and `docker` (if you're running the tests on Kind)

**NOTES**
- The tests look for `kubeconfig` files in the root of the local repository directory to authenticate to a Hub and Managed cluster:
  - `kubeconfig_hub`
  - `kubeconfig_managed`
- To use a self-managed Hub for the tests, export `deployOnHub=true` and use the same file for both kubeconfigs.

### On Local Kind Clusters

For hub-only:
```shell
export deployOnHub=true
make kind-bootstrap-cluster
```

For a hub with a managed cluster attached:
```shell
make kind-bootstrap-cluster
```

To delete the clusters and the `kubeconfig` files:
```shell
make kind-delete-cluster
```

### On external clusters with Open Cluster Management already installed

For hub-only:
```shell
cp <path/to/hub/kubeconfig> ./kubeconfig_hub
cp ./kubeconfig_hub ./kubeconfig_managed
export deployOnHub=true
export MANAGED_CLUSTER_NAME=local-cluster
```

For a hub with a managed cluster attached:
```shell
cp <path/to/hub/kubeconfig> ./kubeconfig_hub
cp <path/to/managed/kubeconfig> ./kubeconfig_managed
export MANAGED_CLUSTER_NAME=<managed_cluster_name_on_hub>
```

## Test Runs

If there's a particular test file you want to run, export the name of the test file to `TEST_FILE` (with or without the path):
```shell
export TEST_FILE=<filename_test.go>
```

### E2E

```shell
make e2e-test
```

### Integration (including Policy Collection)

```shell
make integration-test
```
