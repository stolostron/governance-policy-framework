#! /bin/bash

################################################################################
#
# Description: This repo-bulk-update subcommand updates the OWNERS file in a
# repository.
#   - If the OWNERS file exists in the repository at REPO_PATH, it will
#     overwrite it with the local OWNERS file.
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

if [[ -f "${REPO_PATH}/OWNERS" ]]; then
  cp "${SCRIPT_PATH}/../../OWNERS" "${REPO_PATH}/OWNERS"
fi
