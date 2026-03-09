#!/usr/bin/env bash

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

get_pr_json() {
  local users=${1}
  local orgs=${2}
  local repos=${3}
  local query="is:pr+is:open+draft:false"

  for name in ${users}; do
    query+="+author:${name}"
  done

  for org in ${orgs}; do
    query+="+org:${org}"
  done

  for repo in ${repos}; do
    query+="+repo:${repo}"
  done

  curl -s -H 'Accept: application/vnd.github.text-match+json' \
    "https://api.github.com/search/issues?per_page=100&q=${query}" | \
    jq '.items[]'
}

print_prs() {
  local title=${1}
  local pr_json=${2}
  local sort=${3:-".created_at"}

  if [[ "$(echo "${pr_json}" | jq -s 'length')" -eq 0 ]]; then
    return
  fi

  echo "* ${title} Pull Requests:"
  {
    printf "TITLE\tUSER\tDATE\tURL\n"
    echo "${pr_json}" | jq -sr 'sort_by('"${sort}"') | .[] | '"${format}"
  } | column -s "	" -t

  echo "${border}"
}

format='"\(if (.title | length) <= 40 then .title else (.title[0:37] + "...") end)\t'
format+='\(.user.login[0:10])\t'
format+='\(.created_at[0:10])\t'
format+='\(.html_url)"'

title="# GRC PR report for $(date) #"
# shellcheck disable=SC2001
border=$(echo "${title}" | sed 's/./#/g')

echo -e "${border}\n${title}\n${border}"

# Fetch PRs authored by the squad
users=$(yq '.reviewers[]' OWNERS)
orgs='
  open-cluster-management-io
  stolostron
  openshift
'

squad_pr_json=$(get_pr_json "${users}" "${orgs}")

repos=$(
  cat "${script_dir}/main-branch-sync/repo.txt"
  cat "${script_dir}/main-branch-sync/repo-extra.txt"
  cat "${script_dir}/main-branch-sync/repo-upstream.txt"
  echo "stolostron/magic-mirror"
)

# Fetch any PRs in our repos
repo_pr_json=$(get_pr_json "" "" "${repos}")

bot_users='
dependabot[bot]
red-hat-konflux[bot]
magic-mirror-bot[bot]
acm-grc-security
openshift-cherrypick-robot
'
bot_users=$(echo "${bot_users}" | jq -Rsc 'split("\n") | map(select(. != ""))')

# Omit the squad PRs and create buckets for PRs from bots and the community
filtered_pr_json=$(echo "${repo_pr_json}" | jq --argjson user_filter "$(yq -o json -I 0 '.reviewers' OWNERS)" 'select(.user.login | IN($user_filter[]) | not)')
bot_pr_json=$(echo "${filtered_pr_json}" | jq --argjson user_filter "${bot_users}" 'select(.user.login | IN($user_filter[]))')
community_pr_json=$(echo "${filtered_pr_json}" | jq --argjson user_filter "${bot_users}" 'select(.user.login | IN($user_filter[]) | not)')

# Print the PRs
print_prs "Community" "${community_pr_json}"
print_prs "Squad" "${squad_pr_json}"
print_prs "Bot" "${bot_pr_json}" ".user.login,.html_url"
