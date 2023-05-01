#!/bin/bash
set -e

path=$(pwd)
exit_code=0

printf '%s\n' "Updating stolostron/gatekeeper ...."
git clone git@github.com:stolostron/gatekeeper.git stolostron/gatekeeper
cd stolostron/gatekeeper
git status
git log --oneline | head -20
git config pull.ff only # set fast-forward only for this repo - other repos could have different settings
git remote add upstream git@github.com:open-policy-agent/gatekeeper.git
git remote -v
git fetch --all
git checkout master
git status
git log --oneline | head -20
git rebase upstream/master
git status
git log --oneline | head -20
git push || { exit_code=1; printf '%s\n' "Failed to fast forward stolostron/gatekeeper"; }
git push --tags || { exit_code=1; printf '%s\n' "Failed to fast forward stolostron/gatekeeper"; }
upstream_release_branches=$(git branch --list --remotes 'upstream/release-*' | sed 's|upstream/||g')
origin_branches=$(git branch --list --remotes 'origin/*' | grep -v HEAD | sed 's|origin/||g')
for branch in $upstream_release_branches; do
    if echo "$origin_branches" | grep -w -q "$branch"; then
        # branch already exists on our fork
        git checkout -b "$branch" "origin/$branch"
        git rebase "upstream/$branch"
        git push || { exit_code=1; printf '%s\n' "Failed to fast forward stolostron/gatekeeper $branch"; }
    else
        # new branch needs to be copied from upstream
        git checkout -b "$branch" "upstream/$branch"
        git push -u origin "$branch" || { exit_code=1; printf '%s\n' "Failed to fast forward stolostron/gatekeeper $branch"; }
    fi
done
cd $path

printf '%s\n' "Updating stolostron/gatekeeper-operator ...."
git clone git@github.com:stolostron/gatekeeper-operator.git stolostron/gatekeeper-operator
cd stolostron/gatekeeper-operator
git remote add upstream git@github.com:gatekeeper/gatekeeper-operator.git
git fetch upstream
git checkout main
git rebase upstream/main
git push || { exit_code=1; printf '%s\n' "Failed to fast forward stolostron/gatekeeper-operator"; }
git push --tags || { exit_code=1; printf '%s\n' "Failed to fast forward stolostron/gatekeeper-operator"; }
git checkout -b release-0.1 origin/release-0.1
git rebase upstream/release-0.1
git push || { exit_code=1; printf '%s\n' "Failed to fast forward stolostron/gatekeeper-operator"; }
cd $path

echo $exit_code
exit $exit_code
