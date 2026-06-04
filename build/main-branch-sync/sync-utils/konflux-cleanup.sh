#! /bin/bash

################################################################################
#
# Description: This is a repo-bulk-update subcommand script to clean up
# the unused Konflux pipeline files in a repository.
#   - It will clean up the unused Konflux pipeline files in the repository at
#     REPO_PATH based on the base branch.
################################################################################

set -e

if [[ -z "${REPO_PATH}" ]]; then
  echo "error: REPO_PATH is not set" >&2
  exit 1
fi

base_branch="$1"

if [[ -z "${base_branch}" ]]; then
  echo "error: missing base branch positional argument" >&2
  exit 1
fi

konflux_path="${REPO_PATH}/.tekton"

function handle_konflux_file_cel() {
  local konflux_file="$1"
  local version="$2"
  local branch=""
  local cel=""

  if [[ ${konflux_file} == *"-acm-${version//./}-"* ]]; then
    branch="release-${version}"
  elif [[ ${konflux_file} == *"-mce-${version//./}-"* ]]; then
    branch="backplane-${version}"
  else
    echo "WARN: unrecognized file pattern: ${konflux_file}" >&2
    return
  fi

  current_cel=$(yq -e '.metadata.annotations["pipelinesascode.tekton.dev/on-cel-expression"]' "${konflux_file}" 2>/dev/null || echo "")
  if [[ -n "${current_cel}" ]]; then
    if [[ ${current_cel} == *"${branch}"* ]]; then
      return
    fi
  fi

  if [[ ${konflux_file} == *"-pull-request.yaml" ]]; then
    cel='event == "pull_request" && target_branch in ["main", "'"${branch}"'"]'
  fi

  if [[ ${konflux_file} == *"-push.yaml" ]]; then
    cel='event == "push" && target_branch == "'"${branch}"'"'
  fi

  cel=${cel} yq -i '.metadata.annotations["pipelinesascode.tekton.dev/on-cel-expression"] = strenv(cel)' "${konflux_file}"
}

if [[ ${base_branch} == "main" ]]; then
  versions="main $(cat "${SCRIPT_PATH}/../../CURRENT_VERSION")"
elif [[ ${base_branch} == release-* ]] || [[ ${base_branch} == backplane-* ]]; then
  versions=${base_branch#*-}
 else
  echo "ERROR: unsupported base branch '${base_branch}' (expected main, release-*, or backplane-*)" >&2
  exit 1
fi

if [[ -d "${konflux_path}" ]]; then
  for konflux_file in "${konflux_path}"/*-pull-request.yaml "${konflux_path}"/*-push.yaml; do
    if [[ ! -f "${konflux_file}" ]]; then
      echo "INFO: Konflux file not found: ${konflux_file}"
      continue
    fi

    remove=true

    for version in ${versions}; do
      if [[ ${konflux_file} == *"-${version//./}-"* ]]; then
        remove=false

        if [[ ${version} != "main" ]]; then
          handle_konflux_file_cel "${konflux_file}" "${version}"
        fi

        break
      fi
    done

    if ${remove}; then
      echo "INFO: Removing unused Konflux file: ${konflux_file}"
      rm "${konflux_file}"
    fi
  done
fi
