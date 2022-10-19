#! /bin/bash

# Validate the pipeline is up to date and that no failed prow jobs exist

COMPONENT_ORG=stolostron
DEFAULT_BRANCH=${DEFAULT_BRANCH:-"main"}
CHECK_RELEASES="2.4 2.5 2.6 2.7"
# This list can include all postsubmit jobs for all repos--if a job doesn't exist it's filtered to empty and skipped
CHECK_JOBS=${CHECK_JOBS:-"publish publish-test images latest-image-mirror latest-test-image-mirror"}
UTIL_REPOS="policy-grc-squad pipeline multiclusterhub-operator"

# Clone the repositories needed for this script to work
cloneRepos() {
	for prereqrepo in ${UTIL_REPOS}; do
		if [ ! -d ${prereqrepo} ]; then
			echo "Cloning ${prereqrepo} ..."
			git clone --quiet https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/${COMPONENT_ORG}/${prereqrepo}.git
		fi
	done
	if [ ! -d "${COMPONENT_ORG}" ]; then
		# Collect repos from https://github.com/stolostron/policy-grc-squad/blob/master/main-branch-sync/repo.txt
		REPOS=$(cat policy-grc-squad/main-branch-sync/repo.txt)
		# Manually append deprecated repos
		REPOS="${REPOS}
			stolostron/grc-ui
			stolostron/grc-ui-api
			stolostron/governance-policy-spec-sync
			stolostron/governance-policy-status-sync
			stolostron/governance-policy-template-sync"
		for repo in $REPOS; do
			echo "Cloning $repo ...."
			git clone --quiet https://github.com/$repo.git $repo
		done
	fi
}

# return URL of open sync issues (uses authenticated API to prevent rate limiting)
getSyncIssues() {
	component=$1
	issues="$(curl -s -H "Authorization: token ${GITHUB_TOKEN}" https://api.github.com/repos/${COMPONENT_ORG}/${component}/issues \
	| jq -r '.[] | select(.pull_request == null) | select(.title|match(".*Failed to sync the upstream PR.*")) | .html_url')"
	echo "${issues}"
}

# return the most recent git sha for a repository's release branch
# Inputs: getGitSha "component" "version"
getGitSha() {
	component=$1
	release=release-$2
	co=`git -C $COMPONENT_ORG/$component checkout --quiet $release`
	GITSHA=`git -C $COMPONENT_ORG/$component log -n 1 --no-decorate --pretty=oneline | awk '{print $1}'`
	echo "$GITSHA"
}

# Fetch a value from the Pipeline manifest
# Inputs: getPipelineValue "component" "version" "manifest-json-key"
getPipelineValue() {
	component="$1"
	release="${2}-integration"
	key="$3"

	co=$(git -C pipeline/ checkout --quiet "$release")
	value=`jq -r '.[] |select(.["image-name"] == "'$component'") | .["'$key'"]' pipeline/manifest.json`
	echo "$value"
}

# Check for Prow job failures
# Inputs: checkProwJob "component" "version" "published-branches"
checkProwJob() {
	component="$1"
	check_publish="$3"

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
		# Skip checking the publish job(s) if there are no images in case it was removed mid-release and the most recent job failed
		if [ "$BRANCH" != "$DEFAULT_BRANCH" ] && [[ "$check_publish" != *":$release:"* ]] && [[ "$job" == "publish"* ]]; then
			echo "WARN: Not checking job $job for $BRANCH"
			continue
		fi
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
	for repo_dir in ${UTIL_REPOS}; do
		rm -rf ${repo_dir}
	done
	rm -rf "$COMPONENT_ORG"
}

rc=0

ARTIFACT_DIR=${ARTIFACT_DIR:-$(pwd)}
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
	HAS_IMAGE=""
	for release in $DEFAULT_BRANCH $CHECK_RELEASES; do
		echo "Checking for failures with component $repo $release ..."

		# Don't check the SHA for the default branch since it's not a release
		if [ "$release" != "$DEFAULT_BRANCH" ]; then
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
				HAS_IMAGE="$HAS_IMAGE:$release:"
			done
		else
			syncIssue=$(getSyncIssues "$repo")
			if [[ -n "${syncIssue}" ]]; then
				echo "****"
				echo "ERROR: Syncing is paused for $repo" | tee -a ${ERROR_FILE}
				echo "   Issue: ${syncIssue}" | tee -a ${ERROR_FILE}
				echo "****"
				rc=1
			fi
		fi

		# check the prow job history
		checkProwJob "$repo" "$release" "$HAS_IMAGE"
		if [ $? -eq 1 ]; then
			rc=1
		fi
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
