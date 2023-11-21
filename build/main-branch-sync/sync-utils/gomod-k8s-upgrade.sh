#! /bin/bash

set -e

if [[ -z "${REPO_PATH}" ]]; then
  echo "error: REPO_PATH is not set" >&2
  exit 1
fi

desired_k8s_version="${1}"
if [[ -z "${desired_k8s_version}" ]]; then
  echo "error: specifying a v0.Y.Z Kubernetes version positional argument is required" >&2
  echo "See https://github.com/kubernetes/kubernetes/tree/master/CHANGELOG for available versions" >&2
  exit 1
fi

if [[ ! -f ${REPO_PATH}/go.mod ]]; then
  echo "INFO: go.mod not found. Skipping Kubernetes version upgrade."
  exit 0
fi

cd "${REPO_PATH}" || exit 1

current_k8s_version=$(go mod edit -json go.mod | jq -r '.Require[] | select(.Path | startswith("k8s.io/api")) | .Version' | head -n 1)
if [[ -z "${current_k8s_version}" ]]; then
  echo "INFO: Failed to find Kubernetes version from packages in go.mod. Continuing..." >&2
  exit
fi

sigs_k8s_packages="
sigs.k8s.io/yaml
sigs.k8s.io/kustomize/api
sigs.k8s.io/kustomize/kyaml
k8s.io/klog/v2
"

for package in ${sigs_k8s_packages}; do
  echo "* Updating package ${package} to latest"
  go get "${package}@latest" || true
done

go mod tidy

packages=$(go list -m all | awk '/^k8s.io\/[^\/]* '"${current_k8s_version%.*}"/' { print $1 }')

for package in ${packages}; do
  echo "* Updating package ${package} to ${desired_k8s_version}"
  go get "${package}@${desired_k8s_version}" || true
done

go mod tidy

controller_runtime_versions="$(curl -s https://proxy.golang.org/sigs.k8s.io/controller-runtime/@v/list | { grep -v "+incompatible\|alpha.\|beta.\|rc." || true; } | sort --version-sort -r)"
for version in ${controller_runtime_versions}; do
  echo "* Looking up Controller Runtime version ${version}"
  gomod=$(go list -m -f '{{.GoMod}}' "sigs.k8s.io/controller-runtime@${version}")
  k8s_version=$(awk '/k8s.io\/api v[0-9]+\.[0-9]+\.[0-9]+/ { print $2 }' "${gomod}")
  echo "* Found Kubernetes version ${k8s_version}"
  if [[ "${k8s_version%.*}" == "${desired_k8s_version%.*}" ]]; then
    CONTROLLER_RUNTIME_VERSION="${version}"
    echo "INFO: Found Controller Runtime version ${CONTROLLER_RUNTIME_VERSION} for Kubernetes version ${desired_k8s_version}"
    break
  fi
done

if [[ -z "${CONTROLLER_RUNTIME_VERSION}" ]]; then
  echo "ERROR: Failed to find Controller Runtime version for Kubernetes version ${desired_k8s_version}" >&2
  exit 1
fi

go get "sigs.k8s.io/controller-runtime@${CONTROLLER_RUNTIME_VERSION}" || true

grep -o 'open-cluster-management.io/[^ ]* => github.com/stolostron/[^ ]*' go.mod | while IFS= read -r replacement; do
  echo "* Updating replacement ${replacement} to main"
  ${SED} -i -E 's%('"${replacement}"') .*%\1 main%' go.mod
done

go mod tidy
