#!/bin/sh

# Validate the pipeline is up to date and that no failed prow jobs exist

COMPONENT_ORG=stolostron
CHECK_RELEASES="2.3 2.4 2.5"

# Clone the repositories needed for this script to work
cloneRepos() {
	if [ ! -d "pipeline" ]; then
		git clone https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/$COMPONENT_ORG/policy-grc-squad.git
	fi
	# Only clone pipeline if it doesn't already exist.
	if [ ! -d "pipeline" ]; then
		git clone https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/$COMPONENT_ORG/pipeline.git
	fi
	REPOS=$(cat policy-grc-squad/main-branch-sync/repo.txt | grep policy | grep -v framework)
	for repo in $REPOS; do
		printf '%s\n' "Updating $repo ...."
		git clone https://github.com/$repo.git $repo
	done
}

# return the most recent git sha for a repository's release branch
getGitSha() {
	component=$1
	release=release-$2
	cd $COMPONENT_ORG/$component
	co=`git checkout $release`
	GITSHA=`git log -n 1 --no-decorate --pretty=oneline | awk '{print $1}'`
	echo "$GITSHA"
	cd $BASEDIR
}

getPipelineValue() {
	component="$1"
	release="${2}-integration"
	key="$3"

	cd pipeline
	co=$(git checkout "$release")
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
			echo "****"
			echo "ERROR!!!  Prow job failure: $repo $release."
			LINK=https://prow.ci.openshift.org$(echo "$JSON" | jq '.SpyglassLink' | sed 's/"//g')
			echo "   Link: $LINK"
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

# Limit repositories to our repositories that create images which means they have prow jobs

cloneRepos
REPOS=`ls "$COMPONENT_ORG"`
for repo in $REPOS; do
	for release in $CHECK_RELEASES; do

		# for each release check the SHA, and image
		gitsha=$(getGitSha "$repo" "$release")
		imagetag=$(getPipelineValue "$repo" "$release" "image-tag")
		pipelinesha=$(getPipelineValue "$repo" "$release" "git-sha256")

		if [ "$gitsha" != "$pipelinesha" ]; then
			echo "****"
			echo "ERROR!!!  SHA mismatch in pipeline and $repo $RELEASE repositories."
        		echo "   pipeline: $pipelinesha"
        		echo "   $repo: $gitsha"
			echo "***"
        		rc=1
		fi

		# make sure the docker image is available
		FOUND=$(curl -s "https://quay.io/api/v1/repository/${COMPONENT_ORG}/${repo}/tag/?onlyActiveTags=true&specificTag=${imagetag}" | jq '.tags | length')
		if [ "${FOUND}" != "1" ]; then
			echo "****"
			echo "ERROR!!!  Tag not found for image in quay: $repo:${imagetag}"
			echo "***"
			rc=1
		fi

		# check the prow job history
		checkProwJob "$repo" "$release"
		if [ $? -eq 1 ]; then
			rc=1
		fi
	done
done

cleanup

exit $rc
