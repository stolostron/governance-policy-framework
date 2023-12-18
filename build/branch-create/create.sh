#!/bin/bash
set -e

NEW_VERSION=$(cat ../CURRENT_VERSION)

path="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
SED="sed"
if [ "${OS}" == "darwin" ]; then
    SED="gsed"
    if [ ! -x "$(command -v ${SED})"  ]; then
       echo "ERROR: $SED required, but not found."
       echo "Perform \"brew install gnu-sed\" and try again."
       exit 1
    fi
fi

echo "* Creating branch 'release-${NEW_VERSION}' in all repos..."

while IFS="" read -r repo || [ -n "${repo}" ]
do
  printf '%s\n' "* Updating ${repo} ...."
  p="${path}/${repo}"
  git clone git@github.com:${repo}.git ${p}
  GIT="git -C ${repo}"
  ${GIT} checkout main
  ${GIT} pull
  ${GIT} checkout -b release-${NEW_VERSION}
  ${GIT} push origin release-${NEW_VERSION}
done <${path}/repo.txt
