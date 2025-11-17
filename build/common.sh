#! /bin/bash

BUILD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

export CHECK_RELEASES EXTRA_REPO_PATH UPSTREAM_REPO_PATH

CHECK_RELEASES="$(cat "${BUILD_DIR}/../CURRENT_VERSION"; echo; cat "${BUILD_DIR}/../CURRENT_SUPPORTED_VERSIONS")"
COMPONENT_ORG=stolostron
DEFAULT_BRANCH=${DEFAULT_BRANCH:-"main"}
UTIL_REPOS="pipeline multiclusterhub-operator"
SKIP_CLONING="${SKIP_CLONING:-"false"}"
SKIP_CLEANUP="${SKIP_CLEANUP:-"false"}"
REPO_PATH="${BUILD_DIR}/main-branch-sync/repo.txt"
EXTRA_REPO_PATH="${BUILD_DIR}/main-branch-sync/repo-extra.txt"
UPSTREAM_REPO_PATH="${BUILD_DIR}/main-branch-sync/repo-upstream.txt"

# Clone the repositories needed for this script to work
cloneRepos() {
	if [[ "${SKIP_CLONING}" == "true" ]]; then
		return 0
	fi

	: "${GITHUB_USER:?GITHUB_USER must be set}"
	: "${GITHUB_TOKEN:?GITHUB_TOKEN must be set}"

	for prereqrepo in ${UTIL_REPOS}; do
		if [ ! -d "${prereqrepo}" ]; then
			echo "Cloning ${prereqrepo} ..."
			git clone --quiet "https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/${COMPONENT_ORG}/${prereqrepo}.git" "${prereqrepo}" || exit 1
		fi
	done
	if [ ! -d "${COMPONENT_ORG}" ]; then
		# Collect repos from main-branch-sync/repo.txt
		REPOS=$(cat "${REPO_PATH}")
		for repo in ${REPOS}; do
			echo "Cloning ${repo} ...."
			git clone --quiet "https://github.com/${repo}.git" "${repo}" || exit 1
		done
	fi
}

cleanup() {
	if [[ "${SKIP_CLEANUP}" == "true" ]]; then
		return 0
	fi

	for repo_dir in ${UTIL_REPOS}; do
		rm -rf "${repo_dir}"
	done
	rm -rf "${COMPONENT_ORG}"
}
