#! /bin/bash

set -e

if [[ -z "${REPO_PATH}" ]]; then
  echo "error: REPO_PATH is not set" >&2
  exit 1
fi

konflux_path="${REPO_PATH}/.tekton"

if [[ -d "${konflux_path}" ]]; then
  for konflux_file in "${konflux_path}"/"$(basename "${REPO_PATH}")"-*.yaml; do
    echo "Found Konflux file: ${konflux_file}"
    echo "konflux-update.sh: konflux update not implemented." >&2
    exit 1
  done
fi
