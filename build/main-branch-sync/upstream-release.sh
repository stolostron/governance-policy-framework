#! /bin/bash

set -e
set -o pipefail

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

echo "=== Review and submit the new releases (GitHub Actions may still be running to create them)"
for REPO in ${REPOS}; do
  echo "* https://github.com/${REPO}/releases/tag/${NEW_RELEASE}"
done

echo
echo '=== Next steps
* Create a new release for the policy generator if needed:
  - https://github.com/open-cluster-management-io/policy-generator-plugin

* If a new generator release was needed, update the POLICY_GENERATOR_TAG variable in all applicable AppSub Dockerfiles:
  - https://github.com/open-cluster-management-io/multicloud-operators-subscription/tree/main/build

* Verify the propagator and addon-controller manifests in clusteradm:
  - https://github.com/open-cluster-management-io/clusteradm/tree/main/pkg/cmd/install/hubaddon/scenario/addon/policy
  
  Reference manifests:
  - https://github.com/open-cluster-management-io/governance-policy-propagator/tree/main/deploy
  - https://github.com/open-cluster-management-io/governance-policy-addon-controller/tree/main/config'
