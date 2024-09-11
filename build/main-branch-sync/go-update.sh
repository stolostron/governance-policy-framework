#!/bin/bash

########################
#
# Usage:
# - export JIRA_ISSUE and GITHUB_TOKEN
# - Run the script and use the produced URLs to open PRs
#
# Description:
# - Update Go version across all repos and supported ACM versions
#
########################

set -e

path="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
RELEASES="$(cat ${path}/../../CURRENT_SUPPORTED_VERSIONS)"
source ${path}/../common.sh
exit_code=0
URLS=""

if [[ -z "${JIRA_ISSUE}" ]]; then
  echo "error: Setting JIRA_ISSUE is required."
  exit 1
fi

UPSTREAM_REPOS="$(${BUILD_DIR}/main-branch-sync/fetch-repo-list.sh)"
if [[ -z "${UPSTREAM_REPOS}" ]]; then
  echo "error: Failed to retrieve upstream repos."
  exit 1
fi

RELEASE_BRANCHES="$(for BRANCH in ${RELEASES}; do echo release-${BRANCH}; done)"
DEFAULT_BRANCH="${DEFAULT_BRANCH:-"main"}"

# Fetch and parse lastest major Go version
GO_VERSION="${GO_VERSION:-"$(curl -s https://go.dev/VERSION?m=text | head -1)"}"
GO_VERSION="${GO_VERSION#go}"
GO_VERSION="${GO_VERSION%\.[0-9]}"
if ! [[ "${GO_VERSION}" =~ ^[0-9]+\.[0-9]+$ ]]; then
  echo "Failed to parse Go version. Found '${GO_VERSION}'."
  exit 1
fi
GO_PATTERN="(go( ?)|(golang-builder:rhel_.*))[0-9]\.[0-9]{2}"

# Fix sed issues on mac by using GSED
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
SED="sed"
if [ "${OS}" == "darwin" ]; then
  SED="gsed"
  if [ ! -x "$(command -v ${SED})" ]; then
    echo "ERROR: ${SED} required, but not found."
    echo 'Perform "brew install gnu-sed" and try again.'
    exit 1
  fi
fi

echo "* Updating Golang to ${GO_VERSION} for ACM ${RELEASES}"

for repo in $(
  echo ${UPSTREAM_REPOS}
  cat ${REPO_PATH}
  cat ${EXTRA_REPO_PATH}
); do
  # Clone repo
  echo "* Handling ${repo}"
  p="${path}/${repo}"
  git clone --quiet https://github.com/${repo} ${p}
  GIT="git -C ${p}"

  # Iterate over release branches
  for branch in ${DEFAULT_BRANCH} ${RELEASE_BRANCHES}; do
    commit_msg="Update to Go v${GO_VERSION}"
    echo "* Checking out branch ${branch}"
    ${GIT} checkout --quiet ${branch} || continue

    # Check relevant files for Go version
    for FILE in ${p}/go.mod ${p}/.ci-operator.yaml "${p}"/build/Dockerfile* "${p}"/Dockerfile*; do
      if [[ ! -f "${FILE}" ]] || [[ -L "${FILE}" ]]; then
        echo "WARN: Skipping check for ${FILE} because it wasn't found"
        continue
      fi
      echo "INFO: Checking version in ${FILE}"
      ${SED} -i -E "s/${GO_PATTERN}/\1${GO_VERSION}/" ${FILE}
    done

    # Continue if no updates were made
    if (${GIT} diff --exit-code 1>/dev/null); then
      echo "INFO: Skipping branch ${branch} because it's already using ${GO_VERSION}"
      continue
    fi

    # Lint code
    echo "INFO: Files modified with Go version. Linting code"
    cd ${p}
    go mod tidy &>/dev/null
    make fmt &>/dev/null
    LINT_ERRORS=""
    make lint &>/dev/null || LINT_ERRORS="LINT--> "
    cd ${path}

    # Create commit and push branch
    echo "INFO: Pushing updates to git"
    refresh_branch="go-update-${branch}"
    if (${GIT} branch --remotes | grep "origin/${refresh_branch}" 1>/dev/null); then
      ${GIT} push --delete origin ${refresh_branch}
    fi
    ${GIT} checkout -b ${refresh_branch}
    if [[ "${branch}" == "release-"* ]]; then
      commit_msg="[${branch}] ${commit_msg}"
    fi
    ${GIT} commit -s -am "${commit_msg}" -m "ref: ${JIRA_ISSUE}" &&
      OUTPUT=$(${GIT} push origin ${refresh_branch} 2>&1) || {
      echo "${OUTPUT}"
      exit 1
    }
    [[ -n "${OUTPUT}" ]] && echo "${OUTPUT}"
    PR_URL="$(echo "${OUTPUT}" | grep "remote:.*https://.*${refresh_branch}" | sed 's/^remote: *//')"
    if [[ -z "${PR_URL}" ]]; then
      PR_URL="${repo} ${branch} : Failed to push update"
    else
      PR_URL="https://github.com/${repo}/compare/${branch}...${refresh_branch}"
    fi
    URLS="${URLS}
    ${LINT_ERRORS}${PR_URL}"
  done
done

echo "Create Pull Requests: ${URLS}"

exit ${exit_code}
