#!/bin/bash
set -e

path="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
exit_code=0
URLS=""
branch="${BRANCH:-"main"}"
refresh_branch="refresh-build-${branch}"
commit_msg="Build refresh"
if [[ "${branch}" == "release-"* ]]; then
  commit_msg="[${BRANCH}] ${commit_msg}" 
fi

# Fix sed issues on mac by using GSED
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
SED="sed"
if [ "${OS}" == "darwin" ]; then
    SED="gsed"
    if [ ! -x "$(command -v ${SED})"  ]; then
       echo "ERROR: ${SED} required, but not found."
       echo 'Perform "brew install gnu-sed" and try again.'
       exit 1
    fi
fi

echo "Refreshing builds on branch ${branch}..."

while IFS="" read -r repo || [ -n "${repo}" ]
do
  printf '%s\n' "Updating ${repo} ...."
  p="${path}/${repo}"
  git clone --quiet https://github.com/${repo} ${p}
  GIT="git -C ${p}"
  if ( ${GIT} branch --remotes | grep "origin/${refresh_branch}" ); then
    ${GIT} push --delete origin ${refresh_branch}
  fi
  ${GIT} checkout --quiet ${branch}
  ${GIT} checkout -b ${refresh_branch}
  newdate=`date +"%m/%d/%Y"`
  ${SED} -i "s,^Date: .*,Date: $newdate," ${p}/README.md
  ${GIT} add ${p}/README.md
  ${GIT} commit -s -m "${commit_msg}" -m "Update date in README.md"  &&
    OUTPUT=`${GIT} push origin ${refresh_branch} 2>&1` || { echo "${OUTPUT}"; exit 1; }
  [[ -n "${OUTPUT}" ]] && echo "${OUTPUT}"
  PR_URL="$(echo "${OUTPUT}" | grep "remote:.*https.*${refresh_branch}" | sed 's/^remote: *//')"
  if [[ -z "${PR_URL}" ]]; then
    PR_URL="${repo} : Failed to update README.md"
  else
    PR_URL="https://github.com/${repo}/compare/${branch}...${refresh_branch}"
  fi
  URLS="${URLS}
  ${PR_URL}"
done <${path}/repo.txt

echo "Create Pull Requests: ${URLS}"

exit ${exit_code}
