#! /bin/bash

################################################################################
#
# Description: This repo-bulk-update subcommand upgrades the Go version in a
# repository.
#   - It will upgrade the Go version in the repository at REPO_PATH to the
#     desired Go version in the go.mod file and container files.
#
################################################################################

set -e

if [[ -z "${REPO_PATH}" ]]; then
  echo "error: REPO_PATH is not set" >&2
  exit 1
fi

go_version="$1"

if [[ -z "${go_version}" ]]; then
  echo "error: specifying a X.Y Go version positional argument is required" >&2
  exit 1
fi

if [[ -f ${REPO_PATH}/go.mod ]]; then
  GO="go -C ${REPO_PATH}"
  ${GO} get "go@${go_version}.0"

  ${GO} mod tidy || {
    echo "error: Failed to tidy go.mod in ${REPO_PATH}" >&2
    read -r -p "Take care of upgrade errors. Press enter to continue"
  }

  if [[ -d ${REPO_PATH}/vendor ]]; then
    ${GO} mod vendor
  fi
fi

if [[ -f ${REPO_PATH}/.ci-operator.yaml ]]; then
  yq '.build_root_image.tag = "go'"${go_version}"'-linux"' -i "${REPO_PATH}/.ci-operator.yaml"
fi

konflux_build_files=""
if [[ -d ${REPO_PATH}/.tekton ]]; then
  #shellcheck disable=SC2016
  konflux_build_files=$(
    yq '. as $pipeline ireduce(""; . + ($pipeline.spec.params[] | select(.name == "dockerfile") | .value) + "\n")' "${REPO_PATH}/.tekton"/*.yaml |
      sort -u |
      sed "s|^|${REPO_PATH}/|"
  )
  konflux_build_files_base=${konflux_build_files//.rhtap/}
fi

for file in ${konflux_build_files} ${konflux_build_files_base} "${REPO_PATH}"/build/Dockerfile* "${REPO_PATH}"/Dockerfile*; do
  if [[ ! -f "${file}" ]] || [[ -L "${file}" ]]; then
    echo "WARN: Skipping check for ${file} because it wasn't found"
    continue
  fi
  echo "INFO: Checking version in ${file}"
  ${SED} -i -E "s/(golang:|go|rhel_[0-9]+_)[0-9]+\.[0-9]+/\1${go_version}/g" "${file}"
done
