#! /bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${DIR}/common.sh

# Compare the .ci-operator.yaml file across the repos
CI_OPERATOR_FILE=".ci-operator.yaml"
CI_OP_PATH="${DIR}/../${CI_OPERATOR_FILE}"

ciopDiff() {
  repo="${1}"

	REPO_CI_OP_PATH="${COMPONENT_ORG}/${repo}/${CI_OPERATOR_FILE}"
	if [[ ! -f ${REPO_CI_OP_PATH} ]]; then
		echo "WARN: ${CI_OPERATOR_FILE} not found: ${REPO_CI_OP_PATH}"
		return 0
	fi

	CI_OP_DIFF=$(diff ${CI_OP_PATH} ${REPO_CI_OP_PATH})
	if [[ -n "${CI_OP_DIFF}" ]]; then
		echo "****"
		echo "ERROR: ${CI_OPERATOR_FILE} is not synced to $repo" | tee -a ${ERROR_FILE}
		echo "${CI_OP_DIFF}" | sed 's/^/   /' | tee -a ${ERROR_FILE}
		echo "***"
		return 1
	fi
}

# Compare the common Makefile across the repos
COMMON_MAKEFILE_NAME=Makefile.common.mk
COMMON_MAKEFILE_PATH=${DIR}/common/${COMMON_MAKEFILE_NAME}

makefileDiff() {
  repo="${1}"

	REPO_MAKEFILE_PATH="${COMPONENT_ORG}/${repo}/build/common/${COMMON_MAKEFILE_NAME}"
	if [[ ! -f ${REPO_MAKEFILE_PATH} ]]; then
		echo "WARN: Makefile not found: ${REPO_MAKEFILE_PATH}"
		return 0
	fi

	MAKEFILE_DIFF=$(diff ${COMMON_MAKEFILE_PATH} ${REPO_MAKEFILE_PATH})
	if [[ -n "${MAKEFILE_DIFF}" ]]; then
		echo "****"
		echo "ERROR: Common Makefile is not synced to $repo" | tee -a ${ERROR_FILE}
		echo "${MAKEFILE_DIFF}" | sed 's/^/   /' #| tee -a ${ERROR_FILE} # Commenting out until the Makefiles are synced since it's too noisy
		echo "***"
		return 1
	fi
}

# Compare the Dockerfile across the repos
COMMON_DOCKERFILE_NAME=Dockerfile
COMMON_DOCKERFILE_PATH=${DIR}/${COMMON_DOCKERFILE_NAME}.e2etest

dockerfileDiff() {
  repo="${1}"

	REPO_DOCKER_PATH="${COMPONENT_ORG}/${repo}/build/${COMMON_DOCKERFILE_NAME}"
	if [[ ! -f ${REPO_DOCKER_PATH} ]]; then
		echo "WARN: Dockerfile not found: ${REPO_DOCKER_PATH}"
		return 0
	fi

	DOCKERFILE_DIFF=$(diff <(grep "^FROM " ${COMMON_DOCKERFILE_PATH}) <(grep "^FROM " ${REPO_DOCKER_PATH}))
	if [[ -n "${DOCKERFILE_DIFF}" ]]; then
		echo "****"
		echo "ERROR: Dockerfile images are not synced to $repo" | tee -a ${ERROR_FILE}
		echo "${DOCKERFILE_DIFF}" | sed 's/^/   /' | tee -a ${ERROR_FILE}
		echo "***"
		rc=1
	fi
}

# Verify package versioning
packageVersioning() {
  repo="${1}"

	PACKAGES="^go 
		github.com/onsi/ginkgo"

	GOMOD_NAME="go.mod"
	REPO_GOMOD_PATH="${COMPONENT_ORG}/${repo}/${GOMOD_NAME}"
	if [[ ! -f ${REPO_GOMOD_PATH} ]]; then
		echo "WARN: ${GOMOD_NAME} not found: ${REPO_GOMOD_PATH}"
		return 0
	fi

	rcode=0
	for pkg in ${PACKAGES}; do
		FRAMEWORK_VERSION="$(awk '/'${pkg//\//\\\/}'/ {print $2}' ${DIR}/../${GOMOD_NAME})"
		REPO_VERSION="$(awk '/'${pkg//\//\\\/}'/ {print $2}' ${REPO_GOMOD_PATH})"

		# If the package wasn't found, assume it's not needed
		if [[ -z "${REPO_VERSION}" ]]; then
			return 0
		fi

		if [[ "${FRAMEWORK_VERSION}" != "${REPO_VERSION}" ]]; then
			echo "****"
			echo "ERROR: ${pkg/^/} version ${REPO_VERSION} in $repo does not match ${FRAMEWORK_VERSION}" | tee -a ${ERROR_FILE}
			echo "***"
			rcode=1
		fi
	done

	return ${rcode}
}

# Get the diff of CRDs across RHACM
crdDiff() {
	if [ "${1}" = "$DEFAULT_BRANCH" ]; then
		BRANCH="${1}"
	else
		BRANCH="release-${1}"
	fi
	propagator_repo="governance-policy-propagator"
	propagator_path="${COMPONENT_ORG}/${propagator_repo}/deploy/crds"
	mch_repo="multiclusterhub-operator"
	mch_path="${mch_repo}/pkg/templates/crds/grc"

	# Check out the target release branch in the repos
	git -C ${COMPONENT_ORG}/${propagator_repo}/ checkout --quiet $BRANCH
	git -C ${mch_repo}/ checkout --quiet $BRANCH

	echo "Checking that all CRDs are present in the MultiClusterHub GRC chart for ${BRANCH} ..."
	PROPAGATOR_CRD_FILES=$(ls -p -1 ${propagator_path} | grep -v /)
	CRD_LIST=$(diff <( echo "${PROPAGATOR_CRD_FILES}" ) <( ls -p -1 ${mch_path} | sed 's/_crd//' | grep -v OWNERS))
	if [[ -n "${CRD_LIST}" ]]; then
		echo "****"
		echo "ERROR: CRDs are not synced to ${mch_repo} for ${BRANCH}" | tee -a ${ERROR_FILE}
		echo "${CRD_LIST}" | sed 's/^/   /' | tee -a ${ERROR_FILE}
		echo "***"
		return 1
	fi

	rcode=0
	for crd_file in ${PROPAGATOR_CRD_FILES}; do
		CRD_DIFF="$(diff ${propagator_path}/${crd_file} ${mch_path}/${crd_file})"
		if [[ -n "${CRD_DIFF}" ]]; then
			echo "****"
			echo "ERROR: CRD $crd_file is not synced to $mch_repo for $BRANCH" | tee -a ${ERROR_FILE}
			echo "${CRD_DIFF}" | sed 's/^/   /' | tee -a ${ERROR_FILE}
			echo "***"
			rcode=1
		fi
	done
	
	return $rcode
}

# Check whether the crd-sync job in the addon controller is passing
crdSyncCheck() {
	echo "Checking the CRD sync GitHub Action in governance-policy-addon-controller ..."
	WORKFLOW_JSON=$(curl -s https://api.github.com/repos/stolostron/governance-policy-addon-controller/actions/workflows/crd-sync.yml/runs)
	WORKFLOW_CONCLUSION=$(echo "${WORKFLOW_JSON}" | jq -r '.workflow_runs[0].conclusion')
	WORKFLOW_URL=$(echo "${WORKFLOW_JSON}" | jq -r '.workflow_runs[0].html_url')
	if [[ "${WORKFLOW_CONCLUSION}" != "success" ]] && [[ "${WORKFLOW_URL}" != "null" ]]; then
		echo "WORKFLOW_CONCLUSION=${WORKFLOW_CONCLUSION}"
		echo "****"
		echo "ERROR: CRD sync action is failing in governance-policy-addon-controller" | tee -a ${ERROR_FILE}
		echo "   Link: ${WORKFLOW_URL}" | tee -a ${ERROR_FILE}
		echo "***"
		return 1
	fi
}

rc=0

ARTIFACT_DIR=${ARTIFACT_DIR:-${PWD}}
ERROR_FILE_NAME="codebase-errors.log"
ERROR_FILE="${ARTIFACT_DIR}/${ERROR_FILE_NAME}"

# Clean up error file if it exists
if [ -f ${ERROR_FILE} ]; then
	rm ${ERROR_FILE}
fi

# Check for consistency across repos
cloneRepos

REPOS=`ls "${COMPONENT_ORG}"`
for repo in ${REPOS}; do
	if !(git -C ${COMPONENT_ORG}/${repo} checkout --quiet ${DEFAULT_BRANCH}); then
		echo "WARN: ${repo} doesn't have a ${DEFAULT_BRANCH} branch. Skipping."

		continue
	fi

	# Verify that the common Makefile matches the framework
  makefileDiff "${repo}"
	if [ $? -eq 1 ]; then
		rc=1
	fi

	# Verify select packages are at the same version
	packageVersioning "${repo}"
	if [ $? -eq 1 ]; then
		rc=1
	fi

	# Verify .ci-operator.yaml file is up-to-date
	ciopDiff "${repo}"
	if [ $? -eq 1 ]; then
		rc=1
	fi

	# Verify that the Dockerfile images match the framework
	dockerfileDiff "${repo}"
	if [ $? -eq 1 ]; then
		rc=1
	fi
done

# Check CRDs for default branch and latest release
for release in $DEFAULT_BRANCH ${CHECK_RELEASES##* }; do
	crdDiff "${release}"
	if [ $? -eq 1 ]; then
		rc=2
	fi
done

crdSyncCheck
if [ $? -eq 1 ]; then
	rc=2
fi

cleanup

echo ""
echo "****"
echo "CODEBASE STATUS REPORT"
echo "***"
if [ -f ${ERROR_FILE} ]; then
	# Print the error log to stdout with duplicate lines removed
	awk '!a[$0]++' ${ERROR_FILE} | tee "${ARTIFACT_DIR}/summary-${ERROR_FILE_NAME}"
else
	echo "All checks PASSED!"
fi
echo "***"

exit ${rc}
