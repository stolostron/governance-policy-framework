#!/bin/bash
set -e

path="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
exit_code=0
CURRENT_VERSION=$(cat ${path}/../../CURRENT_VERSION)

while IFS="" read -r repo || [ -n "${repo}" ]
do
  printf '%s\n' "Updating ${repo} ...."
  p="${path}/${repo}"
  git clone https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com:${repo}.git ${p}
  GIT="git -C ${p}"
  ${GIT} checkout main
  ${GIT} pull
  ${GIT} checkout release-${CURRENT_VERSION}
  ${GIT} rebase main
  ${GIT} push || { exit_code=1; printf '%s\n' "Failed to fast forward ${p}"; }
done <${path}/repo.txt

exit ${exit_code}
