#!/usr/bin/env bash

USERS=(dhaiducek gparvin JustinKuli mprahl yiraeChristineKim JeffeyL zyjjay)
ORGS=(open-cluster-management-io stolostron openshift)

query="is:pr+is:open+draft:false"

for name in "${USERS[@]}"; do
  query+="+author:${name}"
done

for org in "${ORGS[@]}"; do
  query+="+org:${org}"
done

format='"\(if (.title | length) <= 40 then .title else (.title[0:37] + "...") end)\t'
format+='\(.user.login[0:10])\t'
format+='\(.created_at[0:10])\t'
format+='\(.html_url)"'

title="# GRC PR report for $(date) #"
border=$(echo "${title}" | sed 's/./#/g')

echo -e "${border}\n${title}\n${border}"

{
  printf "TITLE\tUSER\tDATE\tURL\n"
  curl -s -H 'Accept: application/vnd.github.text-match+json' \
    "https://api.github.com/search/issues?q=${query}" \
    | jq -r '.items | reverse | .[] | '"${format}"
} | column -s "	" -t
