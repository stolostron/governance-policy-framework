#!/bin/sh

# Validate the pipeline is up to date and that no failed prow jobs exist

COMPONENT_ORG=stolostron
CHECK_RELEASES="2.3 2.4 2.5"

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
getGitSha() {
	component=$1
	release=release-$2
	cd $COMPONENT_ORG/$component
	co=`git checkout --quiet $release`
	GITSHA=`git log -n 1 --no-decorate --pretty=oneline | awk '{print $1}'`
	echo "$GITSHA"
	cd $BASEDIR
}

getPipelineValue() {
	component="$1"
	release="${2}-integration"
	key="$3"

	cd pipeline
	co=$(git checkout --quiet "$release")
	value=`jq '.[] |select(.["image-name"] == "'$component'") | .["'$key'"]' manifest.json | sed 's/"//g'`
	echo "$value"
	cd $BASEDIR
}

checkProwJob() {
	component="$1"
	release="$2"

	rcode=0
	# This is a hack to get data from the openshift ci prow
	# sample curl: curl -H "content-type: application/xml" https://prow.ci.openshift.org/job-history/gs/origin-ci-test/logs/branch-ci-open-cluster-management-iam-policy-controller-release-2.5-publish
        # which contains this:   var allBuilds = [{"SpyglassLink":"/view/gs/origin-ci-test/logs/branch-ci-open-cluster-management-iam-policy-controller-release-2.5-publish/1468625788839399424","ID":"1468625788839399424","Started":"2021-12-08T16:57:31Z","Duration":103000000000,"Result":"FAILURE","Refs":{"org":"open-cluster-management","repo":"iam-policy-controller","repo_link":"https://github.com/open-cluster-management/iam-policy-controller","base_ref":"release-2.5","base_sha":"567b3597e8324a4c56ec8f1d717ae15d9671e4a8","base_link":"https://github.com/open-cluster-management/iam-policy-controller/compare/d544db4214b4...567b3597e832"}}];
	echo "Checking prow jobs for a failure with component $component."
	jobs="publish images latest-image-mirror"
	for job in $jobs; do
		OUTPUT=$(curl -s https://prow.ci.openshift.org/job-history/gs/origin-ci-test/logs/branch-ci-${COMPONENT_ORG}-${component}-release-${release}-${job})
		JSON=$(echo "$OUTPUT" | grep "var allBuilds" | sed 's/  var allBuilds =//' | sed 's/;$//' | jq '.[0]')
		STATUS=$(echo "$JSON" | jq '.Result')
		if [ "$STATUS" = \"FAILURE\" ]; then
			LINK=https://prow.ci.openshift.org$(echo "$JSON" | jq '.SpyglassLink' | sed 's/"//g')
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
	for release in $CHECK_RELEASES; do

		# for each release check the SHA, and image
		gitsha=$(getGitSha "$repo" "$release")
		imagetag=$(getPipelineValue "$repo" "$release" "image-tag")
		pipelinesha=$(getPipelineValue "$repo" "$release" "git-sha256")

		if [ -z "$pipelinesha" ]; then
			echo "****"
			echo "WARN: Pipeline SHA not found for $repo $release repository. Continuing."
			echo "***"
		elif [ "$gitsha" != "$pipelinesha" ]; then
			echo "****"
				echo "ERROR: SHA mismatch in pipeline and $repo $release repositories." | tee -a ${ERROR_FILE}
							echo "   imageName: $imageName" | tee -a ${ERROR_FILE}
							echo "   pipeline: $pipelinesha" | tee -a ${ERROR_FILE}
							echo "   $repo: $gitsha" | tee -a ${ERROR_FILE}
				echo "***"

		# make sure the quay image is available (only if we found it in pipeline)
		if [ -n "$imagetag" ]; then
			QUAY_RESPONSE=$(curl -s "https://quay.io/api/v1/repository/${COMPONENT_ORG}/${repo}/tag/?onlyActiveTags=true&specificTag=${imagetag}")
			QUAY_STATUS=$(echo "${QUAY_RESPONSE}" | jq -r '.error_message')
			FOUND=$(echo "${QUAY_RESPONSE}" | jq '.tags | length')
			if [ "${QUAY_STATUS}" != "null" ]; then
				echo "****"
					echo "ERROR: Error '${QUAY_STATUS}' querying $repo $release image in quay: $imageName:${imagetag}" | tee -a ${ERROR_FILE}
					echo "***"
					echo "ERROR: Tag not found for image in quay: $repo:${imagetag}" | tee -a ${ERROR_FILE}
					echo "***"

		# check the prow job history
		checkProwJob "$repo" "$release"
		if [ $? -eq 1 ]; then
			rc=1
		fi
	done
done

cleanup

echo ""
echo "****"
echo "PROW STATUS REPORT:"
echo "****"
if [ -f ${ERROR_FILE} ]; then
	# Print the error log to stdout with duplicate lines removed
	awk '!a[$0]++' ${ERROR_FILE}
else
	echo "All checks PASSED!"
fi
echo "****"

exit $rc
