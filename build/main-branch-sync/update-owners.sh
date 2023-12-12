#!/bin/bash

########################
#
# Usage:
# - export NEW_OWNERS to add to the owners file
# - export DELETE_OWNERS to delete from the owners file
#
# Description:
# - Add or remove space delimited owners from repos listed in repos.txt
#
########################

CHECKOUT_BRANCH=${CHECKOUT_BRANCH:-""}
OWNERS_FILE_NAME="OWNERS"
GITHUB_ORG=${GITHUB_ORG:-"stolostron"}
REPOS="${REPOS:-$(cat repo.txt && cat repo-extra.txt)}"
UPDATED_REPOS=()
if [[ "${GITHUB_ORG}" != "stolostron" ]]; then
  REPOS=$(echo "${REPOS}" | sed "s/stolostron/${GITHUB_ORG}/g")
fi

for repo in ${REPOS}; do
  # Handle custom paths
  OWNERS_PATH="."
  if [[ "$repo" == *","* ]]; then
    OWNERS_PATH="${OWNERS_PATH}/${repo##*,}"
    repo="${repo%%,*}"
  fi
  OWNERS_PATH="${OWNERS_PATH}/${OWNERS_FILE_NAME}"

  # Clone repo
  echo "==="
  printf '%s\n' "Updating $repo ${OWNERS_PATH} ..."
  MESSAGE=$(curl -s https://api.github.com/repos/$repo | jq .message)
  if [[ "${MESSAGE}" == '"Not Found"' ]]; then
    continue
  fi
  git clone --quiet https://github.com/$repo.git $repo

  # Checkout target branch if specified
  GIT="git -C ${repo}"
  OWNERS_PATH="${repo}/${OWNERS_PATH}"
  BRANCH_NAME="update-owners"
  COMMIT_MSG="Update ${OWNERS_FILE_NAME}"
  if [[ -n "${CHECKOUT_BRANCH}" ]]; then
    ${GIT} checkout ${CHECKOUT_BRANCH} || continue
    BRANCH_NAME="${BRANCH_NAME}-${CHECKOUT_BRANCH}"
    COMMIT_MSG="[${CHECKOUT_BRANCH}] ${COMMIT_MSG}"
  fi
  # Update OWNERS file
  if [[ -f "${OWNERS_PATH}" ]]; then
    ${GIT} checkout -b ${BRANCH_NAME}
    if [[ -n "${NEW_OWNERS}" ]]; then
      for owner_name in $NEW_OWNERS; do
        new_owner="${owner_name}" yq e '(.approvers, .reviewers) |= (. + env(new_owner) | unique)' -i ${OWNERS_PATH}
      done
    fi
    if [[ -n "${DELETE_OWNERS}" ]]; then
      for owner_name in $DELETE_OWNERS; do
        delete_owner="${owner_name}" yq e '(.approvers[], .reviewers[]) |= del(select(. == env(delete_owner)))' -i ${OWNERS_PATH}
      done
    fi
    sed -i '' 's/^  //g' ${OWNERS_PATH}
    if ! git diff --exit-code &>/dev/null; then
      ${GIT} commit --signoff -am "${COMMIT_MSG}"
      ${GIT} push --set-upstream origin ${BRANCH_NAME} && UPDATED_REPOS+=("$repo")
    else
      echo "No changes detected. Skipping update."
    fi
  else
    echo "File not found: ${OWNERS_PATH}"
  fi
done


echo "==="
if [[ "${#UPDATED_REPOS[@]}" != "0" ]]; then
  echo "URLs to open PRs:"
  for repo in "${UPDATED_REPOS[@]}"; do
    PULL_URL="$(git -C ${repo} remote get-url --push origin | sed 's/.git$//')"
    echo "* ${PULL_URL}/pull/new/${BRANCH_NAME}"
  done
else
  echo "No PRs opened for updates."
fi

rm -rf ${GITHUB_ORG}
