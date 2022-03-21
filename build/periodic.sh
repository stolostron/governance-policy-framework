#!/bin/sh

# Validate the pipeline is up to date and that no failed prow jobs exist

COMPONENT_ORG=stolostron
DEFAULT_BRANCH=${DEFAULT_BRANCH:-"main"}
CHECK_RELEASES="2.3 2.4 2.5"
# This list can include all postsubmit jobs for all repos--if a job doesn't exist it's filtered to empty and skipped
CHECK_JOBS=${CHECK_JOBS:-"publish images latest-image-mirror latest-test-image-mirror"}

# Clone the repositories needed for this script to work
cloneRepos() {
	if [ ! -d "policy-grc-squad" ]; then
		echo "Cloning policy-grc-squad ..."
		git clone --quiet https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/$COMPONENT_ORG/policy-grc-squad.git
	fi
	if [ ! -d "pipeline" ]; then
		echo "Cloning pipeline ..."
		git clone --quiet https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/$COMPONENT_ORG/pipeline.git
	fi
	if [ ! -d "${COMPONENT_ORG}" ]; then
		# Collect repos from https://github.com/stolostron/policy-grc-squad/blob/master/main-branch-sync/repo.txt
		REPOS=$(cat policy-grc-squad/main-branch-sync/repo.txt)
		for repo in $REPOS; do
			echo "Cloning $repo ...."
			git clone --quiet https://github.com/$repo.git $repo
		done
	fi
}

# return the most recent git sha for a repository's release branch
# Inputs: getGitSha "component" "version"
getGitSha() {
	component=$1
	release=release-$2
	cd $COMPONENT_ORG/$component
	co=`git checkout --quiet $release`
	GITSHA=`git log -n 1 --no-decorate --pretty=oneline | awk '{print $1}'`
	echo "$GITSHA"
	cd $BASEDIR
}

# Fetch a value from the Pipeline manifest
# Inputs: getPipelineValue "component" "version" "manifest-json-key"
getPipelineValue() {
	component="$1"
	release="${2}-integration"
	key="$3"

	cd pipeline
	co=$(git checkout --quiet "$release")
	value=`jq -r '.[] |select(.["image-name"] == "'$component'") | .["'$key'"]' manifest.json`
	echo "$value"
	cd $BASEDIR
}

# Check for Prow job failures
# Inputs: checkProwJob "component" "version" "published-branches"
checkProwJob() {
	component="$1"

	if [ "$release" = "$DEFAULT_BRANCH" ]; then
		BRANCH="$2"
	else
		BRANCH="release-$2"
	fi

	rcode=0
	# This is a hack to get data from the openshift ci prow
	# sample curl: 
	# 	curl -s https://prow.ci.openshift.org/job-history/gs/origin-ci-test/logs/branch-ci-open-cluster-management-iam-policy-controller-release-2.5-publish
				# which contains this:
				# var allBuilds = [
				# 	{"SpyglassLink":"/view/gs/origin-ci-test/logs/branch-ci-open-cluster-management-iam-policy-controller-release-2.5-publish/1468625788839399424",
				# 	 "ID":"1468625788839399424","Started":"2021-12-08T16:57:31Z","Duration":103000000000,"Result":"FAILURE",
				# 	 "Refs":
				# 		{"org":"open-cluster-management","repo":"iam-policy-controller",
				# 		 "repo_link":"https://github.com/open-cluster-management/iam-policy-controller",
				# 		 "base_ref":"release-2.5","base_sha":"567b3597e8324a4c56ec8f1d717ae15d9671e4a8",
				# 		 "base_link":"https://github.com/open-cluster-management/iam-policy-controller/compare/d544db4214b4...567b3597e832"
				# 		}}];
	for job in $CHECK_JOBS; do
		OUTPUT=$(curl -s https://prow.ci.openshift.org/job-history/gs/origin-ci-test/logs/branch-ci-${COMPONENT_ORG}-${component}-${BRANCH}-${job})
		JSON=$(echo "$OUTPUT" | grep "var allBuilds" | sed 's/  var allBuilds =//' | sed 's/;$//' | jq '.[0]')
		STATUS=$(echo "$JSON" | jq -r '.Result')
		if [ "$STATUS" = "FAILURE" ] || [ "$STATUS" = "ABORTED" ]; then
			LINK=https://prow.ci.openshift.org$(echo "$JSON" | jq -r '.SpyglassLink')
			echo "****"
			echo "ERROR: Prow job failure: $repo $release" | tee -a ${ERROR_FILE}
			echo "   Link: $LINK" | tee -a ${ERROR_FILE}
			echo "***"
			rcode=1
		fi
	done
	return $rcode
}

cleanup() {
	cd "$BASEDIR"
	rm -rf pipeline
	rm -rf policy-grc-squad
	rm -rf "$COMPONENT_ORG"
}

BASEDIR=$(pwd)
rc=0

ARTIFACT_DIR=${ARTIFACT_DIR:-${BASEDIR}}
ERROR_FILE="${ARTIFACT_DIR}/errors.log"

# Clean up error file if it exists
if [ -f ${ERROR_FILE} ]; then
	rm ${ERROR_FILE}
fi

# Limit repositories to our repositories that create images which means they have prow jobs

cloneRepos
REPOS=`ls "$COMPONENT_ORG"`
for repo in $REPOS; do
	# Special handling if repo name differs from image name or repo has more than one image
	case $repo in
		governance-policy-framework)
			IMAGES="grc-policy-framework-tests";;
		grc-ui)
			IMAGES="$repo $repo-tests";;
		*)
			IMAGES=$repo;;
	esac
	for release in $DEFAULT_BRANCH $CHECK_RELEASES; do
		echo "Checking for failures with component $repo $release ..."
		# check the prow job history
		checkProwJob "$repo" "$release"
		if [ $? -eq 1 ]; then
			rc=1
		fi

		# Don't check the SHA for the default branch since it's not a release
		if [ "$release" = "$DEFAULT_BRANCH" ]; then
			continue
		fi
		# for each release and each image name, check the Git SHA
		gitsha=$(getGitSha "$repo" "$release")
		for imageName in ${IMAGES}; do
			pipelinesha=$(getPipelineValue "$imageName" "$release" "git-sha256")

			if [ -z "$pipelinesha" ]; then
				echo "WARN: Pipeline SHA not found for $repo $release repository for $imageName. Continuing."
			elif [ "$gitsha" != "$pipelinesha" ]; then
				echo "****"
				echo "ERROR: SHA mismatch in pipeline and $repo $release repositories." | tee -a ${ERROR_FILE}
							echo "   imageName: $imageName" | tee -a ${ERROR_FILE}
							echo "   pipeline: $pipelinesha" | tee -a ${ERROR_FILE}
							echo "   $repo: $gitsha" | tee -a ${ERROR_FILE}
				echo "***"
				rc=1
			fi
		done

		# for each release and each image name, check the tag in Quay
		for imageName in ${IMAGES}; do
			imagetag=$(getPipelineValue "$imageName" "$release" "image-tag")
			# If the tag wasn't found in pipeline, skip checking for it in Quay
			if [ -z "$imagetag" ]; then
				continue
			fi
			# make sure the quay image is available
			QUAY_RESPONSE=$(curl -s "https://quay.io/api/v1/repository/${COMPONENT_ORG}/${imageName}/tag/?onlyActiveTags=true&specificTag=${imagetag}")
			QUAY_STATUS=$(echo "${QUAY_RESPONSE}" | jq -r '.error_message')
			FOUND=$(echo "${QUAY_RESPONSE}" | jq '.tags | length')
			if [ "${QUAY_STATUS}" != "null" ]; then
				echo "****"
				echo "ERROR: Error '${QUAY_STATUS}' querying $repo $release image in quay: $imageName:${imagetag}" | tee -a ${ERROR_FILE}
				echo "***"
				rc=1
			elif [ "${FOUND}" != "1" ]; then
				echo "****"
				echo "ERROR: Tag not found for image in quay: $repo:${imagetag}" | tee -a ${ERROR_FILE}
				echo "***"
				rc=1
			fi
		done
	done
done

cleanup

echo ""
echo "****"
echo "PROW STATUS REPORT:"
echo "***"
if [ -f ${ERROR_FILE} ]; then
	# Print the error log to stdout with duplicate lines removed
	awk '!a[$0]++' ${ERROR_FILE}
else
	echo "All checks PASSED!"
fi
echo "***"

exit $rc
