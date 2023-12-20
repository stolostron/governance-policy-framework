#!/bin/bash
set -e

path="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
exit_code=0
CURRENT_VERSION=$(cat ${path}/../../CURRENT_VERSION)
COMMIT_TIME="${COMMIT_TIME:-"now"}"

echo "* Fast-forwarding repos using commit time '${COMMIT_TIME}' ..."

while IFS="" read -r repo || [ -n "${repo}" ]
do
  echo "::group::${repo}"
  echo "* Updating ${repo} ..."
  p="${path}/${repo}"
  git clone https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/${repo}.git ${p}
  GIT="git -C ${p}"
  TARGET_COMMIT="$(${GIT} rev-list -1 --before="${COMMIT_TIME}" main)"
  if [[ -z "${TARGET_COMMIT}" ]]; then
    echo "* ERROR: Failed to fetch commit for ${p}"
    exit_code=1
    echo "::endgroup::"
    continue
  fi
  echo "* Checking out 'main@{${COMMIT_TIME}}' ..."
  ${GIT} -c advice.detachedHead=false checkout ${TARGET_COMMIT}
  echo "* Checking out 'release-${CURRENT_VERSION}' ..."
  ${GIT} checkout release-${CURRENT_VERSION} || ${GIT} checkout -b release-${CURRENT_VERSION}
  echo "* Fast-forward to 'main@{${COMMIT_TIME}}' ..."
  ${GIT} merge --ff-only ${TARGET_COMMIT}
  echo "* Push to 'release-${CURRENT_VERSION}'"
  ${GIT} push origin release-${CURRENT_VERSION} || { exit_code=1; echo "* ERROR: Failed to fast forward ${p}"; }
  echo "::endgroup::"
done <${path}/repo.txt

exit ${exit_code}
