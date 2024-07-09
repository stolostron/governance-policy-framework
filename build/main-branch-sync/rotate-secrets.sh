#! /bin/bash

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

GITHUB_BOT_USER="acm-grc-security"
AWS_BOT_USER="ocm-grc-aws-kind-bot"
COLLECTIVE_NS="acm-grc-security"

# Verify the CLI prerequisites
for CLI in gh aws jq yq oc; do
  if ! (which ${CLI} &>/dev/null); then
    echo "The ${CLI} CLI is not installed. Install to continue."
    exit 1
  fi
done

# Verify the oc version
MAJOR_OC=$(oc version -o json | jq -r '.clientVersion.gitVersion' | grep -o "[0-9]\+\.[0-9]\+" | sed 's/\.[0-9]*$//')
MINOR_OC=$(oc version -o json | jq -r '.clientVersion.gitVersion' | grep -o "[0-9]\+\.[0-9]\+" | sed 's/^[0-9]*\.//')
if (( ${MAJOR_OC} < 4 )) || (( ${MAJOR_OC} == 4 )) && (( ${MINOR_OC} < 11 )); then
  echo "The openshift CLI must be at version 4.11 or later. Current version: $(oc version -o json | jq -r '.clientVersion.gitVersion')"
  exit 1
fi

# Verify AWS access
AWS_LOGIN_USER=$(aws iam list-access-keys | jq -r '.AccessKeyMetadata[0].UserName')

if [[ -z "${AWS_LOGIN_USER}" ]]; then
  echo "Log in to the AWS CLI with an Access Key (creatable from the AWS console) to continue"
  exit 1
fi

echo "Currently logged into AWS as: ${AWS_LOGIN_USER}"

# Verify GitHub access
gh auth status

# Verify we're connected to Collective
CLUSTER=$(oc config get-contexts | awk '/^\052/ {print $3}' | awk '{gsub("^api-",""); gsub("(\/|-red-chesterfield).*",""); print}')
if [[ "${CLUSTER}" != "collective-aws" ]] || (! oc status &>/dev/null); then
  echo "The oc CLI is not currently logged in to the Collective cluster. Please configure the CLI and try again."
  echo "Current cluster: ${CLUSTER}"
  echo "Link to login command: https://oauth-openshift.apps.collective.aws.red-chesterfield.com/oauth/token/request"
  exit 1
fi

echo "For GitHub, 'click through' to the specific token, and select 'Regenerate token'"
echo "For SonarCloud, you must delete the existing token and create a new one."

# Collect tokens from user input
read -s -p "Enter the \"Clusterpool Token\" GitHub token regenerated from ${GITHUB_BOT_USER} (https://github.com/settings/tokens): " COLLECTIVE_GH_TOKEN
echo ""
read -s -p "Enter the \"Token for Builds\" GitHub token regenerated from ${GITHUB_BOT_USER} (https://github.com/settings/tokens): " BUILDS_GH_TOKEN
echo ""
read -s -p "Enter the \"open-cluster-management+grcbot\" regenerated SonarCloud token (https://sonarcloud.io/account/security/): " SONAR_TOKEN
echo ""

# Generate new AWS token
echo "Generating new AWS token..."
OLD_AWS_TOKEN=$(aws iam list-access-keys --user-name ${AWS_BOT_USER} | jq -r '.AccessKeyMetadata | sort_by(.CreateDate)[0].AccessKeyId')
# Delete oldest AWS token to make space for creation
aws iam delete-access-key --user-name ${AWS_BOT_USER} --access-key-id ${OLD_AWS_TOKEN}
# Create and store new AWS token
NEW_AWS_TOKEN_JSON=$(aws iam create-access-key --user-name ${AWS_BOT_USER})
AWS_ACCESS_KEY_ID=$(echo "${NEW_AWS_TOKEN_JSON}" | jq -r '.AccessKey.AccessKeyId')
AWS_SECRET_ACCESS_KEY=$(echo "${NEW_AWS_TOKEN_JSON}" | jq -r '.AccessKey.SecretAccessKey')

# Regenerate ServiceAccount tokens on Collective by deleting old Secrets
echo "Regenerating secrets on Collective in the ${COLLECTIVE_NS} namespace..."
SERVICE_ACCT_NAME="policy-grc-sa"
oc delete secret $(oc get sa ${SERVICE_ACCT_NAME} -n ${COLLECTIVE_NS} -o jsonpath='{.secrets[*].name}')
SERVICE_ACCT_NAME="policy-grc-prow-sa"
oc delete secret $(oc get sa ${SERVICE_ACCT_NAME} -n ${COLLECTIVE_NS} -o jsonpath='{.secrets[*].name}')

# Update credentials on Collective using regenerated tokens
oc delete secret rhacmstackem-github-secret policy-grc-aws-creds -n ${COLLECTIVE_NS}
oc create secret generic rhacmstackem-github-secret -n ${COLLECTIVE_NS} --from-literal=user=acm-grc-security --from-literal=token="${COLLECTIVE_GH_TOKEN}"
oc create secret generic policy-grc-aws-creds -n ${COLLECTIVE_NS} --from-literal=aws_access_key_id="${AWS_ACCESS_KEY_ID}" --from-literal=aws_secret_access_key="${AWS_SECRET_ACCESS_KEY}"

# Update credentials for each existing cluster deployment
for CLUSTER_DEPLOYMENT in $(oc get clusterdeployments -l cluster.open-cluster-management.io/clusterset=acm-grc-security -A --no-headers | awk '{ print $1 }'); do
  echo "Updating secrets on Collective for ClusterDeployment ${CLUSTER_DEPLOYMENT}..."
  oc delete secret -n ${CLUSTER_DEPLOYMENT} ${CLUSTER_DEPLOYMENT}-aws-creds
  oc create secret generic ${CLUSTER_DEPLOYMENT}-aws-creds -n ${CLUSTER_DEPLOYMENT} --from-literal=aws_access_key_id=${AWS_ACCESS_KEY_ID} --from-literal=aws_secret_access_key=${AWS_SECRET_ACCESS_KEY}
done

# Get new token from Collective
COLLECTIVE_SECRET=$(oc create token ${SERVICE_ACCT_NAME} -n ${COLLECTIVE_NS} --duration 2160h)

# Update SonarCloud tokens in GitHub repos
echo "Setting SonarCloud token on GitHub repos..."
for REPO in $(cat ${SCRIPT_DIR}/repo.txt && cat ${SCRIPT_DIR}/repo-extra.txt); do
  gh secret set SONAR_TOKEN -b ${SONAR_TOKEN} --repo ${REPO}
done

# Update GitHub tokens for GitHub Actions
echo "Setting GitHub token on GitHub repos..."
for REPO in stolostron/governance-policy-framework stolostron/gatekeeper-operator; do
  gh secret set WORKFLOW_USER -b ${GITHUB_BOT_USER} --repo ${REPO}
  gh secret set WORKFLOW_TOKEN -b ${BUILDS_GH_TOKEN} --repo ${REPO}
done

while read -r -p "This script is going to print the new secrets to the screen. Is your screen secure? (Press 'y' to continue) " response; do
  case "$response" in
     Y|y )  break
            ;;
  esac
done

echo "
* Manual updates required:

========
  PROW
========
- https://vault.ci.openshift.org/ui/vault/secrets/kv/list/selfservice/ocm-grc-secrets/

  ocm-grc-clusterpool.token     ${COLLECTIVE_SECRET}

  Note: For each token, login with OIDC, click the link for the token, click \"Create new version\", 
        and update the values for the keys above with the regenerated tokens.

  Note: If you can not see the secrets after logging in, contact another member of the team to gain
        access. You can not be added to the collection until you log in once. Refer to
        https://github.com/stolostron/cicd-docs/blob/main/prow/secrets.md.

=============
  BITWARDEN
=============
- https://vault.bitwarden.com/#/vault
  SonarCloud GRC Build Token  ${SONAR_TOKEN}

  Note: This value is updated for discoverability.
"
