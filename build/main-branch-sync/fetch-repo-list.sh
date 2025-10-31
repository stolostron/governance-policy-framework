#! /bin/bash

########################
#
# Usage:
# - export GITHUB_TOKEN
#
# Description:
# - Fetch upstream repos
#
########################

if [[ -z "${GITHUB_TOKEN}" ]]; then
  echo "error: Exporting GITHUB_TOKEN is required to fetch repos." >&2
  exit 1
fi

GITHUB_ORG=${GITHUB_ORG:-"open-cluster-management-io"}
GITHUB_TEAM=${GITHUB_TEAM:-"sig-policy"}
GITHUB_API_URL="https://api.github.com/orgs/${GITHUB_ORG}/teams/${GITHUB_TEAM}/repos?per_page=100"

REPOS_RESPONSE=$(curl -s -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GITHUB_TOKEN}" "${GITHUB_API_URL}")

if [[ "$(echo "${REPOS_RESPONSE}" | jq -r '.message' 2>/dev/null)" == "Not Found" ]]; then
  echo "* Team '${GITHUB_TEAM}' in org '${GITHUB_ORG}' returned a 'Not Found' message" >&2
  exit 1
fi

echo "${REPOS_RESPONSE}"  | jq -r '.[] | select(.archived == false and .role_name == "write") | .full_name'
