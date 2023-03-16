#! /bin/bash

COMPONENT_ORG=stolostron
DEFAULT_BRANCH=${DEFAULT_BRANCH:-"main"}
UTIL_REPOS="policy-grc-squad pipeline multiclusterhub-operator"
SKIP_CLONING="${SKIP_CLONING:-"false"}"
SKIP_CLEANUP="${SKIP_CLEANUP:-"false"}"

# Clone the repositories needed for this script to work
cloneRepos() {
  if [[ "${SKIP_CLONING}" == "true" ]]; then
    return 0
  fi

	for prereqrepo in ${UTIL_REPOS}; do
		if [ ! -d ${prereqrepo} ]; then
			echo "Cloning ${prereqrepo} ..."
			git clone --quiet https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/${COMPONENT_ORG}/${prereqrepo}.git || exit 1
		fi
	done
	if [ ! -d "${COMPONENT_ORG}" ]; then
		# Collect repos from https://github.com/stolostron/policy-grc-squad/blob/master/main-branch-sync/repo.txt
		REPOS=$(cat policy-grc-squad/main-branch-sync/repo.txt)
		# Manually append deprecated repos
		REPOS="${REPOS}
			stolostron/governance-policy-spec-sync
			stolostron/governance-policy-status-sync
			stolostron/governance-policy-template-sync
			stolostron/policy-collection
			stolostron/policy-generator-plugin"
		for repo in $REPOS; do
			echo "Cloning $repo ...."
			git clone --quiet https://github.com/$repo.git $repo || exit 1
		done
	fi
}

cleanup() {
  if [[ "${SKIP_CLEANUP}" == "true" ]]; then
    return 0
  fi

	for repo_dir in ${UTIL_REPOS}; do
		rm -rf ${repo_dir}
	done
	rm -rf "$COMPONENT_ORG"
}