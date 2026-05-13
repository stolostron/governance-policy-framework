#! /bin/bash

################################################################################
#
# Description: This repo-bulk-update subcommand syncs the common build files in
# a repository.
#   - It will copy the common build and repo configuration files from the local
#     repository to the repository at REPO_PATH.
#   - After copying, it runs build and generation commands.
# 
################################################################################

set -e

if [[ -z "${SCRIPT_PATH}" ]]; then
  echo "error: SCRIPT_PATH is not set" >&2
  exit 1
fi

if [[ -z "${REPO_PATH}" ]]; then
  echo "error: REPO_PATH is not set" >&2
  exit 1
fi

SED="sed"
if [ "${OS}" == "darwin" ]; then
  SED="gsed"
  if [ ! -x "$(command -v ${SED})" ]; then
    echo "ERROR: ${SED} required, but not found."
    echo 'Perform "brew install gnu-sed" and try again.'
    exit 1
  fi
fi

common_files=(
  build/common/config/.golangci.yml
  build/common/Makefile.common.mk
  .github/dependabot.yml
  .github/renovate.json
)

for filepath in "${common_files[@]}"; do
  if [[ -f "${REPO_PATH}/${filepath}" ]]; then
    cp "${SCRIPT_PATH}/../../${filepath}" "${REPO_PATH}/${filepath}"
  fi
done

cd "${REPO_PATH}" || exit 1

if [[ -f "${REPO_PATH}/go.mod" ]]; then
  ${SED} -i "s%- prefix(.*)%- prefix($(go list -m))%" "${REPO_PATH}/build/common/config/.golangci.yml"
  make generate || true
  make manifests || true
  make generate-operator-yaml || true
  make fmt || true
  make lint || true
fi
