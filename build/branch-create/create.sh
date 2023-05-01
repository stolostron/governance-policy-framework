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

# Special handling
for repo in "stolostron/governance-policy-addon-controller" "stolostron/governance-policy-framework"; do
  echo "* Updating image tag in Makefile for ${repo} (this will require a pull request) ..."
  p="${path}/${repo}"
  git clone git@github.com:${repo}.git ${p}
  GIT="git -C ${repo}"
  ${GIT} checkout -b version-tag-${NEW_VERSION}
  ${SED} -i "s/\(TAG ?= latest-\).*/\1${NEW_VERSION}/" ${p}/Makefile
  ${GIT} diff
  ${GIT} commit --signoff -am "Update Makefile image tag to ${NEW_VERSION}"
  ${GIT} push origin version-tag-${NEW_VERSION}
  echo "^^^ Please open a pull request to update the tag."
done
