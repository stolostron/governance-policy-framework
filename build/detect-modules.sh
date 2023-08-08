#! /bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

RELEASES="2.6 2.7 2.8 2.9"

if [[ -z "${1}" ]]; then
  echo "Error: A module query argument is required to run this script."
  exit 1
fi

MODULE_QUERY="${1}"
echo "Checking releases: ${RELEASES}"
echo "Go module query: '${MODULE_QUERY}'"

# Collect repos from main-branch-sync/repo.txt
REPOS=$(cat ${DIR}/main-branch-sync/repo.txt)
# Manually append deprecated repos
REPOS="${REPOS}
  stolostron/governance-policy-spec-sync
  stolostron/governance-policy-status-sync
  stolostron/governance-policy-template-sync"
SUMMARY="Modules matching '${MODULE_QUERY}':"
FOUND="false"
GO_LIST_TMPL='
{{- define "M" }}{{ .Path }}@{{ .Version }}{{ end -}}
{{- with .Module -}}
  {{- if not .Main -}}
    {{- if .Replace -}}
      {{ template "M" .Replace }}
    {{- else -}}
      {{ template "M" . }}
    {{- end -}}
  {{- end -}}
{{- end }}'

for REPO in ${REPOS}; do
  echo "* Checking ${REPO}"
  git clone --quiet git@github.com:${REPO}.git ${DIR}/${REPO}
  cd ${DIR}/${REPO}
  for RELEASE in ${RELEASES}; do
    git fetch origin release-${RELEASE} &>/dev/null && \
      git checkout --force origin/release-${RELEASE} &>/dev/null || \
      { echo "${REPO} ${RELEASE}: branch could not be checked out"; continue; }
    OUTPUT=""
    go mod download &>/dev/null
    MAIN_MODULES="$(go list -deps -f "${GO_LIST_TMPL}" all | sort -u)"
    OUTPUT="$(echo "${MAIN_MODULES}" | grep -i "${MODULE_QUERY}")"
    if [[ -n "${OUTPUT}" ]]; then
      echo "${OUTPUT}"
      echo "^^^ Found in ${REPO} ${RELEASE}"
      FOUND="true"
      OUTPUT="$(echo "${OUTPUT}" | sed 's/^/  /')"
      SUMMARY="${SUMMARY}\n${REPO} ${RELEASE}:\n${OUTPUT}"
    else
      echo "=== ${REPO} ${RELEASE}: '${MODULE_QUERY}' not found"
    fi
  done
  cd ${DIR}
done

if [[ "${FOUND}" == "false" ]]; then
  SUMMARY="${SUMMARY}\n  Module not found."
fi

echo -e "\n${SUMMARY}"
