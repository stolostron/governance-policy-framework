#!/bin/bash

set -eo pipefail

export SCRIPT_PATH
SCRIPT_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

# Fix sed issues on mac by using GSED
os=$(uname -s | tr '[:upper:]' '[:lower:]')
export SED
SED="sed"
if [ "${os}" == "darwin" ]; then
  SED="gsed"
  if [ ! -x "$(command -v ${SED})" ]; then
    echo "ERROR: ${SED} required, but not found."
    echo 'Perform "brew install gnu-sed" and try again.'
    exit 1
  fi
fi

# Set up script variables
urls=""
preserve_forks=false
upstream=false
target_branch="main"
branch=""
branch_suffix=""
delete_branch=false
commit_msg=""
commit_body=""
silent=false
dry_run=false
sync_util=""
sync_util_args=()

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
  --help)
    break
    ;;
  --preserve-forks)
    preserve_forks=true
    shift
    ;;
  --upstream)
    upstream=true
    shift
    ;;
  --target-branch)
    target_branch="$2"
    shift 2
    ;;
  --head-branch)
    head_branch="$2"
    shift 2
    ;;
  --delete-branch)
    delete_branch=true
    shift
    ;;
  --commit-msg)
    commit_msg="$2"
    shift 2
    ;;
  --commit-body)
    commit_body="$2"
    shift 2
    ;;
  --silent)
    silent=true
    shift
    ;;
  --dry-run)
    dry_run=true
    shift
    ;;
  --branch-suffix)
    branch_suffix="$2"
    shift 2
    ;;
  --)
    shift
    break
    ;;
  -*)
    echo "Unknown option: $1"
    exit 1
    ;;
  *)
    if [[ -n "${sync_util}" ]]; then
      sync_util_args+=("$1")
      shift
    else
      sync_util="$1"
      shift
    fi
    ;;
  esac
done

sync_util_list=$(cd "${SCRIPT_PATH}/sync-utils" && for f in *.sh; do basename "$f" .sh; done)

if [[ "$1" == "--help" || "$1" == "-h" || -z "${sync_util}" ]]; then
  cat <<EOF
This script automates the process of bulk updating the GRC repositories.

Usage: $(basename "$0") [OPTIONS] SYNC_UTIL [SYNC_ARGS...]

Arguments:
  SYNC_UTIL                    Name of sync utility from sync-utils/ (without .sh extension)
                               Available: $(echo "${sync_util_list}" | paste -s -d, -)
  SYNC_ARGS                    Arguments to pass to the sync utility

Options:
  --preserve-forks             Preserve downstream forks in repository list (default: false)
  --upstream                   Use upstream repositories (default: false)
  --target-branch BRANCH       Target branch to update (default: main)
  --head-branch BRANCH         Name for the working branch
  --branch-suffix SUFFIX       Suffix string for head branch name
  --delete-branch              Delete existing head branch before updating (default: false)
  --commit-msg MSG             Commit message (required unless --dry-run is passed)
  --commit-body BODY           Commit body (optional)
  --silent                     Do not ask for confirmation when committing (default: false)
  --dry-run                    Show what changes would be made without committing/pushing (default: false)
  --help, -h                   Show this help and exit

Examples:
  $(basename "$0") --commit-msg "sync main pipeline" go-upgrade
  $(basename "$0") --branch release-2.8 --commit-msg "chore: update release" custom-sync arg1 arg2
  $(basename "$0") --dry-run --branch-suffix test go-upgrade
EOF
  exit 0
fi

# Validate sync utility
if [[ ! -f "${SCRIPT_PATH}/sync-utils/${sync_util}.sh" ]]; then
  echo "error: Sync utility '${sync_util}' not found in sync-utils directory." >&2
  echo "Available utilities:" >&2
  echo "${sync_util_list}" | ${SED} 's/^/ - /g' >&2
  exit 1
fi

if [[ -z "${commit_msg}" ]] && ! ${dry_run}; then
  echo "error: Must specify --commit-msg (not required in dry-run mode)." >&2
  exit 1
fi

if [[ ${head_branch} == "release-"* ]] || [[ ${head_branch} == "main" ]]; then
  echo "error: Branch name cannot be main or a release branch. Did you mean to use --target-branch?" >&2
  exit 1
fi

if [[ -z "${branch_suffix}" ]] && [[ -z "${head_branch}" ]]; then
  branch_suffix=${sync_util}
fi

upstream_script="cat ${SCRIPT_PATH}/repo-upstream.txt"
if [[ ${target_branch} == "release-"* ]] || ! ${upstream}; then
  upstream_script=":"
fi

if [[ ${target_branch} == "release-"* ]]; then
  commit_msg="[${target_branch}] ${commit_msg}"
fi

# shellcheck disable=SC2016
deduplication=(-F'/' '!a[$2]++')
if ${preserve_forks}; then
  deduplication=('{print}')
fi

# Collect repositories to handle
# The default value here collects stolostron and OCM-io repos and
# deduplicates based on the name of the repo, keeping the OCM-io one
repos=$(
  {
    ${upstream_script} || exit 1
    cat "${SCRIPT_PATH}/repo.txt"
    cat "${SCRIPT_PATH}/repo-extra.txt"
  } | awk "${deduplication[@]}"
)

# Preflight check for existing git directories
for repo in ${repos}; do
  repo_path="${SCRIPT_PATH}/${repo}"
  if [[ -d "${repo_path}" ]]; then
    echo "ERROR: Repository directory already exists: ${repo}"
    echo "Run 'make clean' before continuing."
    exit 1
  fi
done

head_branch="${head_branch:-"$(id -un)-${target_branch}-${branch_suffix}"}"

echo "Running repo update with:
  Sync utility:              ${sync_util} ${sync_util_args[*]}
  Target branch:             ${target_branch}
  Head branch:               ${head_branch}
  Delete existing branch:    ${delete_branch}
  Preserve downstream repos: ${preserve_forks}
  Dry run mode:              ${dry_run}
  Commit title: ${commit_msg}
  Commit body: ${commit_body}

Repositories
------------
${repos}
"

proceed=false
while read -r -p "Continue with updates? (n - exit, y - continue) " response; do
  case "${response}" in
  Y | y)
    proceed=true
    break
    ;;
  N | n)
    proceed=false
    break
    ;;
  *)
    echo "Invalid response. Enter 'n' or 'y'."
    ;;
  esac
done
if ! ${proceed}; then
  exit 0
fi

echo "=== Handling commits for target branch ${target_branch}"

for repo in ${repos}; do
  printf '\n=== %s\n' "Updating ${repo}"
  export REPO_PATH
  REPO_PATH="${SCRIPT_PATH}/${repo}"
  git clone --depth=1 --single-branch --branch="${target_branch}" --quiet "https://github.com/${repo}" "${REPO_PATH}" || continue
  git="git -C ${REPO_PATH}"

  if ! ${dry_run}; then
    if [[ -n "$(${git} ls-remote --heads origin "${head_branch}")" ]]; then
      if ${delete_branch}; then
        ${git} push --delete origin "${head_branch}"
      else
        ${git} fetch origin "${head_branch}:${head_branch}"
        ${git} checkout "${head_branch}"
      fi
    fi
  fi

  if [[ $(${git} branch --show-current) != "${head_branch}" ]]; then
    ${git} checkout --quiet "${target_branch}" || continue
    ${git} checkout -b "${head_branch}"
  fi

  # Execute the specified sync utility with provided arguments
  "${SCRIPT_PATH}/sync-utils/${sync_util}.sh" "${sync_util_args[@]}"

  ${git} add .

  if (${git} diff --staged --exit-code >/dev/null); then
    echo "INFO: No changes to commit."
  else
    if ${dry_run}; then
      echo "[DRY RUN] Changes that would be committed:"
      ${git} --no-pager diff --staged
      echo "[DRY RUN] Would commit with message: '${commit_msg}'"
      if [[ -n "${commit_body}" ]]; then
        echo "[DRY RUN] Would include commit body: '${commit_body}'"
      fi
      echo "[DRY RUN] Would push to origin/${head_branch}"
      urls="${urls}
    [DRY RUN] https://github.com/${repo}/compare/${target_branch}...${head_branch}"
    else
      ${git} --no-pager diff --staged
      if ! ${silent}; then
        proceed=false
        while read -r -p "Continue with commit? (n - skip, y - proceed) " response; do
          case "${response}" in
          Y | y)
            proceed=true
            break
            ;;
          N | n)
            proceed=false
            break
            ;;
          *)
            echo "Invalid response. Enter 'n' or 'y'."
            ;;
          esac
        done
        if ! ${proceed}; then
          continue
        fi
      fi

      # shellcheck disable=SC2086 # amend contains multiple arguments
      ${git} commit -s -S -a -m "${commit_msg}" -m "${commit_body}"
      output=$(${git} push origin "${head_branch}" 2>&1) || {
        printf "ERROR $?: %s\n" "${output}"
        exit 1
      }
      [[ -n "${output}" ]] && echo "${output}"
      urls="${urls}
    https://github.com/${repo}/compare/${target_branch}...${head_branch}"
    fi
  fi

  # Clean up cloned repository in dry-run mode
  if ${dry_run}; then
    rm -rf "${REPO_PATH}"
  fi
done

if ${dry_run}; then
  echo "[DRY RUN] Summary - Pull Requests that would be created: ${urls}"
else
  echo "Create Pull Requests: ${urls}"
fi
