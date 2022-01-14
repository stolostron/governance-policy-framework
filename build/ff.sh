#!/bin/bash

set -e

body='{
"request": {
"branch":"master"
}}'

if [ $FF == 'true' ]; then
   curl -s -X POST \
      -H "Content-Type: application/json" \
      -H "Accept: application/json" \
      -H "Travis-API-Version: 3" \
      -H "Authorization: token $TRAVIS_TOKEN" \
      -d "$body" \
      https://api.travis-ci.com/repo/stolostron%2Fpolicy-grc-squad/requests
else
  echo 'skipping fast forward'
fi
