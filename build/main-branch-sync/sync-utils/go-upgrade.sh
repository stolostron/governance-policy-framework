#! /bin/bash

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

if [[ -f ${REPO_PATH}/.ci-operator.yaml ]]; then
  yq '.build_root_image.tag = "go'"${go_version}"'-linux"' -i "${REPO_PATH}/.ci-operator.yaml"
fi

for file in "${REPO_PATH}"/build/Dockerfile* "${REPO_PATH}"/Dockerfile*; do
  if [[ ! -f "${file}" ]] || [[ -L "${file}" ]]; then
    echo "WARN: Skipping check for ${file} because it wasn't found"
    continue
  fi
  echo "INFO: Checking version in ${file}"
  ${SED} -i -E "s/(go|rhel_9_)[0-9]+\.[0-9]+/\1${go_version}/g" "${file}"
done
