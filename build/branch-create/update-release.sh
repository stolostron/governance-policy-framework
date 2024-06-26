#! /bin/bash

########################
#
# Usage:
#   From the `release` repo folder, execute the script.
#   Export the following variables beforehand:
#   - OLD_VERSION: The previous version of the release (Be sure to update the CURRENT_VERSION file
#        at the root of this repo, which will be used as NEW_VERSION)
#   - USERNAME: (Optional) Use a GitHub user present in the OWNERS file if a 'repo.txt' file is not
#     present
#
# Description:
#   NOTE: Prior to execution, review and update the logic to fit the latest
#         configuration instructions from CICD and your squad's needs. If files
#         are not created as you expect, the script can be updated and re-run
#         and the files will be overwritten.
#   This script will:
#   - Copy the Prow configuration files from the version given in OLD_VERSION
#     to the version given in NEW_VERSION (if the new version is not given,
#     it will calculate the next version for you by adding 0.1)
#   - Update configuration files (old, new, and main) to point to the NEW_VERSION
#   - Run `make update` (after prompting)
#   - Cut new branches and display URLs to open PRs for each component
#
########################

SCRIPT_DIR=$(dirname "${BASH_SOURCE[0]}")

# Check for dependencies
if [[ -z "$(which yq)" ]]; then
  echo "ERROR: You must have 'yq' installed."
  exit 1
fi
# Check for dependencies
if [[ -z "$(which docker)" ]] || [[ "$(
  docker ps &>/dev/null
  echo $?
)" != "0" ]]; then
  echo "WARNING: You must have 'docker' installed and running to run 'make update'."
fi

# Check for 'release' repo
if [[ "${PWD##*/}" != "release" ]]; then
  echo "ERROR: Run this script from the base of the 'release' repo directory."
  exit 1
fi

# Check Git status for current changes
if [[ -n "$(git diff)" ]]; then
  echo "ERROR: Stash or commit the current changes to prevent losing your work."
  exit 1
fi

RELEASE_ROOT_PATH="${PWD}"
RELEASE_CONFIG_PATH="${RELEASE_ROOT_PATH}/ci-operator/config"
RELEASE_RULES_PATH="${RELEASE_ROOT_PATH}/core-services/prow/02_config"

echo "===== Checking for environment variables ====="
# Version checks
NEW_VERSION=$(cat ${SCRIPT_DIR}/../../CURRENT_VERSION)
OLD_VERSION=$(head -1 ${SCRIPT_DIR}/../../CURRENT_SUPPORTED_VERSIONS)

if [ -z "${OLD_VERSION}" ]; then
  echo "error: OLD_VERSION is empty"
  exit 1
fi
if [ -z "${NEW_VERSION}" ]; then
  echo "error: NEW_VERSION is empty"
  exit 1
fi
if [ "${NEW_VERSION}" == "${OLD_VERSION}" ]; then
  echo "error: NEW_VERSION ('${NEW_VERSION}') is equal to OLD_VERSION ('${OLD_VERSION}')"
  exit 1
fi

# Generate component list
if [ -f "${SCRIPT_DIR}/repo.txt" ]; then
  echo "* Using 'repo.txt' file found in script directory."
  COMPONENT_LIST=$(cat "${SCRIPT_DIR}/repo.txt" | sed 's/^/.\//' | sed 's/$/\//')
elif [ -n "$USERNAME" ]; then
  echo "* 'repo.txt' file not found in script directory. Using provided USERNAME to parse for 'OWNERS' files."
  COMPONENT_LIST=$(grep -rl "\- ${USERNAME}" . | sed 's/OWNERS//')
else
  echo "ERROR: Must specify USERNAME or provide a 'repo.txt' file in the script directory."
  exit 1
fi

echo "* USERNAME: ${USERNAME:-"(Using repo.txt)"}"
echo "* OLD_VERSION: ${OLD_VERSION}"
echo "* NEW_VERSION: ${NEW_VERSION}"
echo ""

for dirpath in ${COMPONENT_LIST}; do
  # Parse component name
  COMPONENT=$(echo ${dirpath} | sed 's/stolostron//g' | sed 's/[/.]//g')
  echo "===== Processing ${COMPONENT} ====="
  FILE_PREFIX="${RELEASE_CONFIG_PATH}/${dirpath}stolostron-${COMPONENT}"
  OLD_FILENAME="${FILE_PREFIX}-release-${OLD_VERSION}.yaml"
  NEW_FILENAME="${FILE_PREFIX}-release-${NEW_VERSION}.yaml"

  # Copy old release configuration to new release configuration
  cp ${OLD_FILENAME} ${NEW_FILENAME}
  if [[ "$?" == "0" ]]; then
    # Detect code type ("go" or "nodejs")
    CODE_TYPE=$(yq e '.build_root.image_stream_tag.tag' ${NEW_FILENAME} | grep -o '^[[:alpha:]]*')

    # Set 'zz_generated_metadata' to the new release branch
    branch="release-${NEW_VERSION}" yq e '.zz_generated_metadata.branch=strenv(branch)' -i ${NEW_FILENAME}

    # Update the 'promotion' stanza
    if [ "$(yq e '.promotion.to[0].name' ${NEW_FILENAME})" != "null" ]; then
      # - For the new version:
      ver="${NEW_VERSION}" yq e '.promotion.to[0].name=strenv(ver)' -i ${NEW_FILENAME}
      yq e '.promotion.to[0].disabled=true' -i ${NEW_FILENAME}
      # - For the 'main' branch:
      ver="${NEW_VERSION}" yq e '.promotion.to[0].name=strenv(ver)' -i ${FILE_PREFIX}-main.yaml
      ver="${NEW_VERSION}" \
        yq e '.tests[] |= select(.as=="git-fast-forward").steps.env.DESTINATION_BRANCH = "release-"+strenv(ver)' -i ${FILE_PREFIX}-main.yaml

      # - For the old version, re-enable promotion:
      yq e 'del(.promotion.to[0].disabled)' -i ${OLD_FILENAME}
    fi

    # Update the 'latest-image-mirror' tests item
    ver="${NEW_VERSION}" yq e '.tests[] |= select(.as=="latest-image-mirror").steps.env.IMAGE_TAG="latest-"+env(ver)' -i ${NEW_FILENAME}

    # Handle UI version tag in 'e2e-tests'
    oldver="${OLD_VERSION}" newver="${NEW_VERSION}" \
      yq e '.tests[] |= select(.as=="e2e-tests").steps.test[].commands |= sub("latest-"+strenv(oldver), "latest-"+strenv(newver))' -i ${NEW_FILENAME}

    # Add custom YAML to 'tests' stanza (uncomment code below and add the YAML strings to it)
    # if [[ "${CODE_TYPE}" == "go" ]]; then
    #   NEW_YAML=''
    # else
    #   NEW_YAML=''
    # fi
    # new_yaml="${NEW_YAML}" yq e '.tests += env(new_yaml)' -i ${NEW_FILENAME}
  else
    echo "* Copy failed. Skipping config update."
  fi

  RULES_CONFIG_FILE="${RELEASE_RULES_PATH}/${dirpath}_prowconfig.yaml"
  EXISTING_CONFIG="$(oldver="${OLD_VERSION}" component="${COMPONENT}" \
    yq e '.branch-protection.orgs.stolostron.repos[strenv(component)].branches["release-"+strenv(oldver)]' ${RULES_CONFIG_FILE})"
  if [ -f "${RULES_CONFIG_FILE}" ] && [ "${EXISTING_CONFIG}" != "null" ]; then
    oldconfig="${EXISTING_CONFIG}" newver="${NEW_VERSION}" component="${COMPONENT}" \
      yq e '.branch-protection.orgs.stolostron.repos[strenv(component)] |= .branches["release-"+strenv(newver)]=env(oldconfig)' -i ${RULES_CONFIG_FILE}
  else
    echo "* Skipping update for _prowconfig.yaml"
  fi

done
echo "* File processing complete"
echo ""

echo "===== Prow update / Create new branches ====="
# Check for dependencies
if [[ "$(
  docker ps &>/dev/null
  echo $?
)" != "0" ]]; then
  echo "ERROR: Docker must be running to continue."
  exit 1
fi
# Prompt to continue
echo "* You should review the updated files before continuing to run 'make update' and creating branches."
echo "* After your review, you can make changes that you require for individual files and then continue, or you can exit and then update and rerun the script to fix broader changes."
while read -r -p "Would you like to continue to run 'make update' and create branches? (y/n) " response; do
  case "$response" in
  Y | y)
    break
    ;;
  N | n)
    exit 0
    ;;
  esac
done

echo "* Running 'make update'"
make update
if [ $? -ne 0 ]; then
  echo "* 'make update' exited with an error. Check the output above."
  exit 1
fi
echo ""

# Create new PR with each component as a separate commit
BRANCH_NAME="ocm-new-grc-release-${NEW_VERSION}"
git checkout -b ${BRANCH_NAME}
echo "* Creating PR for the updated configurations."
for dirpath in ${COMPONENT_LIST}; do
  # Parse component name
  COMPONENT=$(echo ${dirpath} | sed 's/stolostron//g' | sed 's/[/.]//g')
  echo "===== Creating commit for ${COMPONENT} ====="
  echo "* Adding files"
  git add ${RELEASE_CONFIG_PATH}/../*/${COMPONENT}/stolostron-${COMPONENT}-*
  git add ${RELEASE_RULES_PATH}/${dirpath}_prowconfig.yaml
  echo "* Committing files"
  git commit -m "New release ${NEW_VERSION} (${COMPONENT})"
done
echo "* Push changes"
git push --set-upstream origin ${BRANCH_NAME}
if [[ -n "$(git ls-files -mo)" ]]; then
  echo "WARNING: Some files were not committed. Check 'git status' for more information."
fi
echo ""

echo "===== Processing complete ====="
echo "* You can use this link to open a new PR:"
GIT_URL="$(git remote get-url --push origin)"
PULL_URL=${GIT_URL%.git}
echo "* ${PULL_URL}/pull/new/${BRANCH_NAME}"
