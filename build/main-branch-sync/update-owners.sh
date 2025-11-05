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

set -eo pipefail

script_path="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

OWNERS_FILE_NAME="OWNERS"
REPOS="${REPOS:-"$(
  cat "${script_path}/repo-upstream.txt"
  cat "${script_path}/repo.txt"
  cat "${script_path}/repo-extra.txt"
  cat "${script_path}/repo-owners.txt"
)"}"

# Store repositories that exist upstream
DUPLICATE_REPOS=$(echo "${REPOS}" | awk -F'/' '!/,/ {print $2}' | sort | uniq -d)

UPDATED_REPOS=()

for repo in ${REPOS}; do
  VERSIONS="main"

  if [[ "${repo%/*}" == "stolostron" ]]; then
    VERSIONS="${VERSIONS} $(cat "${script_path}/../../CURRENT_SUPPORTED_VERSIONS")"
  fi

  # Handle custom paths
  ignore_clone_err=false
  OWNERS_PATH="${OWNERS_FILE_NAME}"
  if [[ "${repo}" == *","* ]]; then
    ignore_clone_err=true
    OWNERS_PATH="${repo##*,}/${OWNERS_FILE_NAME}"
    repo="${repo%%,*}"
  fi

  # Clone repo
  printf '%s\n' "* Updating ${repo} ${OWNERS_PATH} ..."
  MESSAGE=$(curl -s "https://api.github.com/repos/${repo}" | jq .message)
  if [[ "${MESSAGE}" == '"Not Found"' ]]; then
    continue
  fi

  if ! git clone --quiet "https://github.com/${repo}.git" "${script_path}/${repo}" && [[ ${ignore_clone_err} == false ]]; then
    exit 1
  fi

  silent=false

  for version in ${VERSIONS}; do
    # Checkout target branch if specified
    GIT="git -C ${script_path}/${repo}"
    REPO_OWNERS_PATH="${script_path}/${repo}/${OWNERS_PATH}"
    BRANCH_NAME="update-owners"
    COMMIT_MSG="Update ${OWNERS_FILE_NAME}"
    CHECKOUT_BRANCH="main"

    if [[ "${version}" != "main" ]]; then
      CHECKOUT_BRANCH="release-${version}"
      ${GIT} checkout "${CHECKOUT_BRANCH}" || continue
      BRANCH_NAME="${BRANCH_NAME}-${CHECKOUT_BRANCH}"
      COMMIT_MSG="[${CHECKOUT_BRANCH}] ${COMMIT_MSG}"
    fi

    # Update OWNERS file
    if [[ -f "${REPO_OWNERS_PATH}" ]]; then
      branch_update=false
      ${GIT} checkout -b "${BRANCH_NAME}" || branch_update=true

      if [[ -n "${NEW_OWNERS}" ]]; then
        for owner_name in ${NEW_OWNERS}; do
          new_owner="${owner_name}" yq e '(.approvers, .reviewers) |= (. + env(new_owner) | unique)' -i "${REPO_OWNERS_PATH}"
        done
      fi

      if [[ -n "${DELETE_OWNERS}" ]]; then
        for owner_name in ${DELETE_OWNERS}; do
          delete_owner="${owner_name}" yq e '(.approvers[], .reviewers[]) |= del(select(. == env(delete_owner)))' -i "${REPO_OWNERS_PATH}"
        done
      fi

      sed -i '' 's/^  //g' "${REPO_OWNERS_PATH}"

      if ! ${GIT} diff --exit-code; then
        if (echo "${DUPLICATE_REPOS}" | grep -q "${repo}") && [[ ${CHECKOUT_BRANCH} == "main" ]] && [[ ${repo} == "stolostron/"* ]]; then
          echo "  Skipping ${repo} - found in DUPLICATE_REPOS"
          continue
        fi

        if [[ ${silent} == false ]]; then
          read -r -p "Continue (y/n)? " response
          case "${response}" in
          n)
            break
            ;;
          *)
            silent=true
            ;;
          esac
        fi

        force=""
        if [[ ${branch_update} == "false" ]]; then
          ${GIT} commit -S --signoff -am "${COMMIT_MSG}"
        else
          ${GIT} commit -S --signoff -a --amend --no-edit
          force="--force"
        fi

        ${GIT} push ${force} --set-upstream origin "${BRANCH_NAME}" || continue
        gh pr create --repo "${repo}" --head "${BRANCH_NAME}" --base "${CHECKOUT_BRANCH}" --title "${COMMIT_MSG}" --body "" || true
        UPDATED_REPOS+=("${script_path}/${repo}")
      else
        echo "  No changes detected. Skipping update."
      fi
    else
      echo "  File not found: ${REPO_OWNERS_PATH}"
    fi
  done
done

echo "==="
if [[ "${#UPDATED_REPOS[@]}" != "0" ]]; then
  echo "URLs to open PRs:"
  for repo in "${UPDATED_REPOS[@]}"; do
    PULL_URL="$(git -C "${repo}" remote get-url --push origin | sed 's/.git$//')"
    echo "* ${PULL_URL}/pull/new/${BRANCH_NAME}"
  done
else
  echo "No PRs opened for updates."
fi
