#! /bin/bash

################################################################################
#
# Description: This is a repo-bulk-update subcommand template script to update
# the Konflux pipeline files in a repository.
#   - It will update the Konflux pipeline files in the repository at REPO_PATH
#     using the commands in this script.
#   - If the boilerplate commands are not updated, the script will exit with an
#     error.
# 
################################################################################

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
