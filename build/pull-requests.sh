#!/usr/bin/env bash

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

get_prs() {
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

  {
    printf "TITLE\tUSER\tDATE\tURL\n"
    curl -s -H 'Accept: application/vnd.github.text-match+json' \
      "https://api.github.com/search/issues?q=${query}" |
      jq -r '.items | reverse | .[] | '"${format}"
  } | column -s "	" -t
}

format='
  \(if (.title | length) <= 40 then .title else (.title[0:37] + "...") end)\t
  \(.user.login[0:10])\t
  \(.created_at[0:10])\t
  \(.html_url)
'

title="# GRC PR report for $(date) #"
# shellcheck disable=SC2001
border=$(echo "${title}" | sed 's/./#/g')

echo -e "${border}\n${title}\n${border}"

users=$(yq '.reviewers[]' OWNERS)
orgs='
  open-cluster-management-io
  stolostron
  openshift
'

get_prs "${users}" "${orgs}"

echo "${border}"

users='
  app/dependabot
  app/red-hat-konflux
  openshift-cherrypick-robot
'
repos=$(
  cat "${script_dir}/main-branch-sync/repo.txt"
  cat "${script_dir}/main-branch-sync/repo-extra.txt"
)

get_prs "${users}" "" "${repos}"
