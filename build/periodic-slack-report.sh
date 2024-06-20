#!/usr/bin/env bash

set -euxo pipefail

# Setup / verify required variables
: "${ARTIFACTS_PATH:?ARTIFACTS_PATH must be set}"
: "${WF_LINK:?WF_LINK must be set}"
: "${WF_CONCLUSION:?WF_CONCLUSION must be set}"
: "${GH_NEEDS_CTX:?GH_NEEDS_CTX must be set}"
: "${ORIGIN:?ORIGIN must be set}"

path="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
CURRENT_VERSION=$(cat ${path}/../CURRENT_VERSION)

cd "${ARTIFACTS_PATH}"

echo "::group::Initial Header"
echo '{"blocks":[{"type":"section","text":{"type":"mrkdwn","text":":good_question:"}}]}' > ./slack-payload.json

if [[ "${WF_CONCLUSION}" == "failed" ]]; then
  yq -i -pj -oj -I=0 '.blocks[0].text.text = ":failed: *Periodic GRC Tests Failed*\n '"${WF_LINK}"'"' ./slack-payload.json
elif [[ "${WF_CONCLUSION}" == "succeeded" ]]; then
  yq -i -pj -oj -I=0 '.blocks[0].text.text = ":tada: *Periodic GRC Tests Succeeded*\n '"${WF_LINK}"'"' ./slack-payload.json
else
  yq -i -pj -oj -I=0 '.blocks[0].text.text = ":warning: *Periodic GRC Tests Cancelled*\n '"${WF_LINK}"'"' ./slack-payload.json
fi

cat ./slack-payload.json
echo "::endgroup::"

echo "::group:: Tests Section"

yq -i -pj -oj -I=0 '.blocks += {"type":"section","text":{"type":"mrkdwn","text":""}}' ./slack-payload.json

prefixs=("./fw-kind-report-latest-false/" \
         "./fw-kind-report-latest-true/" \
         "./fw-kind-report-latest-false-hosted/" \
         "./fw-kind-report-minimum-false/" \
         "./fw-kind-report-minimum-true/" \
         "./integration-report/" \
         "./integration-report/etcd-")

for prefix in "${prefixs[@]}"; do
  report="${prefix}report.json"
  echo "${report}"

  name="Integration Tests (normal)"
  if [[ "${prefix}" == *"etcd"* ]]; then
    name="Integration Tests (etcd)"
  elif [[ "${prefix}" == *"fw-kind"* ]]; then
    testkind="${prefix#./fw-kind-report-}"
    testkind="${testkind%?}" # remove extra "/" from the end

    name="KinD Tests (${testkind})"
  fi

  if [[ -e "${report}" ]]; then
    success="$(yq -pj -oy '.[0].SuiteSucceeded' "${report}")"
    if [[ "${success}" == "true" ]]; then
      yq -i -pj -oj -I=0 '.blocks[-1].text.text += ":white_check_mark: '"${name}"'\n"' ./slack-payload.json
    else
      yq -pj -oj -I=0 '.[0].SpecReports | filter(.State == "failed")' "${report}" > "${report}-fails"
      failcount="$(yq -pj -oy 'length' "${report}-fails")"
      yq -i -pj -oj -I=0 '.blocks[-1].text.text += ":failed: '"${name}"' failure count: '"${failcount}"'\n"' ./slack-payload.json

      # Add failure message
      yq -i -pj -oj -I=0 '.blocks += {"type": "rich_text", "elements": [{"type": "rich_text_preformatted", "elements":[{"type":"text", "text":"- "}]}]}' ./slack-payload.json
      yq -i -pj -oj -I=0 '.blocks[-1].elements[0].elements[0].text += (load("'"${report}-fails"'") | [.[] | (.ContainerHierarchyTexts[0] // "[" + .LeafNodeType + "]") + "\n  " + .LeafNodeText + "\n  " + (.Failure.Message | sub("\n", "\n    "))] | join("\n- "))' ./slack-payload.json
      yq -i -pj -oj -I=0 '.blocks += {"type":"section","text":{"type":"mrkdwn","text":""}}' ./slack-payload.json
    fi
  else
    yq -i -pj -oj -I=0 '.blocks[-1].text.text += ":warning: '"${name}"' has no test report, it might have failed before the tests\n"' ./slack-payload.json
  fi

  cat ./slack-payload.json
done

echo "::endgroup::"

echo "::group::Fast Forwarding Section"

ff_status="$(echo "${GH_NEEDS_CTX}" | yq -pj -oy '.ff.result')"
if [[ "${ff_status}" == "success" ]]; then
  yq -i -pj -oj -I=0 '.blocks += {"type":"section","text":{"type":"mrkdwn","text":":fast_forward: Fast Forwarded to '"${CURRENT_VERSION}"'"}}' ./slack-payload.json
elif [[ "${ff_status}" == "skipped" ]]; then
  yq -i -pj -oj -I=0 '.blocks += {"type":"section","text":{"type":"mrkdwn","text":":dotted_line_face: Fast Forwarding Skipped"}}' ./slack-payload.json
else
  yq -i -pj -oj -I=0 '.blocks += {"type":"section","text":{"type":"mrkdwn","text":":failed: Fast Forwarding Failed"}}' ./slack-payload.json
fi

cat ./slack-payload.json
echo "::endgroup::"

echo "::group::Context Section"

yq -i -pj -oj -I=0 '.blocks += {"type":"context","elements":[{"type":"mrkdwn","text":"'"${ORIGIN}"'"}]}' ./slack-payload.json

cat ./slack-payload.json
echo "::endgroup::"
