#! /bin/bash

set -e

SCRIPT_PATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

if [[ -z "${NEW_RELEASE}" ]]; then
  echo "error: Export NEW_RELEASE with the new vX.Y.Z version before running this script."
  exit 1
fi

echo "=== Tagging upstream repos with ${NEW_RELEASE}"
REPOS="$(${SCRIPT_PATH}/fetch-repo-list.sh | grep -v 'collection\|generator\|nucleus')"
for REPO in ${REPOS}; do
  echo "* Handling ${REPO} ..."
  git clone --quiet https://github.com/${REPO}.git ${SCRIPT_PATH}/${REPO}
  GIT="git -C ${SCRIPT_PATH}/${REPO}"
  ${GIT} tag -a ${NEW_RELEASE} -m "${NEW_RELEASE}"
  ${GIT} push origin ${NEW_RELEASE}
done

echo "=== View the new releases"
for REPO in ${REPOS}; do
  echo "* https://github.com/${REPO}/releases/tag/${NEW_RELEASE}"
done
