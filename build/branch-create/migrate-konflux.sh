#!/bin/bash

set -eo pipefail

# Script to migrate Tekton pipeline configurations for Konflux branches
# Processes existing konflux-{repo}-acm-{version} branches across stolostron repositories

# Get script directory
SCRIPT_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

# Read current version from CURRENT_VERSION file
CURRENT_VERSION=$(cat "${SCRIPT_PATH}/../../CURRENT_VERSION")
VERSION_NO_DOTS="${CURRENT_VERSION//./}"

repos=$(cat "${SCRIPT_PATH}/repo.txt")
new_branch="$(id -un)-acm-${VERSION_NO_DOTS}"

echo "Migrate Konflux Script - Version ${CURRENT_VERSION}"
echo "=============================================="

# Validate required tools
missing_deps=()
for dep in yq git gh; do
  if ! command -v "${dep}" &>/dev/null; then
    missing_deps+=("${dep}")
  fi
done
if [ ${#missing_deps[@]} -ne 0 ]; then
  echo "ERROR: Missing required dependencies: ${missing_deps[*]}"
  echo "Please install the missing tools and try again."
  exit 1
fi

# Process each repository
echo "Processing repositories for konflux-*-acm-${VERSION_NO_DOTS} branches..."
echo ""

processed_count=0
skipped_count=0

for repo in ${repos}; do
  repo_name=$(basename "${repo}")
  konflux_branch="konflux-${repo_name}-acm-${VERSION_NO_DOTS}"

  echo "=== Processing ${repo} ==="
  echo "Looking for branch: ${konflux_branch}"

  # Set up repository directory path (will be ignored by .gitignore)
  REPO_PATH="${SCRIPT_PATH}/${repo}"

  # Check if directory already exists from previous run
  if [[ -d "${REPO_PATH}" ]]; then
    echo "INFO: Repository directory already exists, removing..."
    rm -rf "${REPO_PATH}"
  fi

  # Clone repository
  if ! git clone --quiet "https://github.com/${repo}.git" "${REPO_PATH}"; then
    echo "WARNING: Failed to clone ${repo}, skipping..."
    ((skipped_count++))
    continue
  fi

  # Set up git command with repository path
  GIT="git -C ${REPO_PATH}"

  # Check if konflux branch exists
  if ! ${GIT} ls-remote --exit-code --heads origin "${konflux_branch}" >/dev/null 2>&1; then
    echo "INFO: Branch ${konflux_branch} not found in ${repo}, skipping..."
    ((skipped_count++))
    echo ""
    continue
  fi

  # Checkout the konflux branch
  if ! ${GIT} checkout --quiet "${konflux_branch}"; then
    echo "WARNING: Failed to checkout ${konflux_branch} in ${repo}, skipping..."
    ((skipped_count++))
    echo ""
    continue
  fi

  # Check if .tekton directory exists
  if [[ ! -d "${REPO_PATH}/.tekton" ]]; then
    echo "INFO: No .tekton directory found in ${repo}, skipping..."
    ((skipped_count++))
    echo ""
    continue
  fi

  echo "Found .tekton directory, processing pipeline configurations..."

  echo "  Reformatting .tekton YAML files..."

  # Reflow all YAML files in .tekton directory for consistent formatting
  for file in "${REPO_PATH}"/.tekton/*.yaml; do
    yq -i '.' "${file}"
  done

  echo "  Processing pipeline configurations..."

  # Process both pull-request and push events
  for event in pull-request push; do
    new_file="${REPO_PATH}/.tekton/${repo_name}-acm-${VERSION_NO_DOTS}-${event}.yaml"

    # Find existing file with this event type (excluding the new file name)
    existing_files=()
    while IFS= read -r -d '' file; do
      existing_files+=("$file")
    done < <(find "${REPO_PATH}/.tekton" -name "*-${event}.yaml" -not -name "$(basename "${new_file}")" -print0 2>/dev/null)

    # Skip if no new file exists
    if [[ ! -f "${new_file}" ]]; then
      echo "    New file $(basename "${new_file}") not found, skipping ${event} event..."
      continue
    fi

    # Skip if no existing file found
    if [[ ${#existing_files[@]} -eq 0 ]]; then
      echo "    No existing ${event} file found to migrate from, skipping..."
      continue
    fi

    # Use the first existing file if multiple found
    existing_file="${existing_files[0]}"

    if [[ ${#existing_files[@]} -gt 1 ]]; then
      echo "    WARNING: Multiple existing ${event} files found, using $(basename "${existing_file}")"
    fi

    echo "    Migrating ${event} configuration from $(basename "${existing_file}") to $(basename "${new_file}")"

    # Update CEL expression to target release branch
    new_cel="target_branch == \"release-${CURRENT_VERSION}\""

    if [[ ${event} == "pull-request" ]]; then
      new_cel="target_branch in [\"main\", \"release-${CURRENT_VERSION}\"]"
    fi

    # Check if the file has the CEL expression annotation
    existing_cel=$(yq -e '.metadata.annotations["pipelinesascode.tekton.dev/on-cel-expression"]' "${new_file}" 2>/dev/null || echo "")
    if [[ -n "${existing_cel}" ]]; then
      new_cel=${existing_cel//target_branch*/${new_cel}} yq -i ".metadata.annotations[\"pipelinesascode.tekton.dev/on-cel-expression\"] = strenv(new_cel)" "${new_file}"
      echo "      Updated CEL expression to: ${new_cel}"
    fi

    # Extract output-image parameter from new file
    new_image_value=$(yq -e '.spec.params[] | select(.name == "output-image") | .value' "${new_file}" 2>/dev/null || echo "")

    if [[ -n "${new_image_value}" ]]; then
      echo "      Found output-image value: ${new_image_value}"

      # Update the new file with the correct image value from existing file's params
      if yq -e '.spec.params[] | select(.name == "output-image")' "${existing_file}" >/dev/null 2>&1; then
        new_image_value=$(yq -e '.spec.params[] | select(.name == "output-image") | .value' "${new_file}")

        # Update the new file with parameters from existing file
        new_image_value=${new_image_value} yq -i "(.spec.params[] | select(.name == \"output-image\") | .value) = env(new_image_value)" "${existing_file}"

        existing_params=$(yq -e '.spec.params' "${existing_file}")
        existing_params=${existing_params} yq -i ".spec.params = env(existing_params)" "${new_file}"

        echo "      Updated parameters from existing file"
      fi
    fi

    # Replace pipelineSpec with pipelineRef from existing file
    if yq -e '.spec.pipelineRef' "${existing_file}" >/dev/null 2>&1; then
      pipeline_ref=$(yq -e '.spec.pipelineRef' "${existing_file}")

      echo "      Replacing pipelineSpec with pipelineRef from $(basename "${existing_file}")"

      # Delete pipelineSpec and add pipelineRef
      yq -i 'del(.spec.pipelineSpec)' "${new_file}"
      pipeline_ref=${pipeline_ref} yq -i ".spec.pipelineRef = env(pipeline_ref)" "${new_file}"

      # Sort spec keys
      yq -i '.spec |= sort_keys(.)' "${new_file}"

      echo "      Updated pipelineRef configuration"
    fi

    # Remove the old configuration file
    echo "      Removing old configuration file: $(basename "${existing_file}")"
    rm "${existing_file}"
  done

  # If changes were made, commit and push
  echo "  Committing and pushing changes..."

  # Stage all changes
  ${GIT} add .

  # Check if there are any staged changes
  if ! ${GIT} diff --staged --exit-code >/dev/null 2>&1; then
    echo "    Amending existing commit with pipeline migration changes..."

    # Amend the existing commit
    if ${GIT} commit --amend --no-edit; then
      echo "    Successfully amended commit"

      ${GIT} --no-pager diff HEAD~1

      proceed=false
      while read -r -p "Continue to create a new branch and open a PR? (n - skip, y - proceed) " response; do
        case "${response}" in
        Y | y)
          proceed=true
          break
          ;;
        N | n)
          proceed=false
          break
          ;;
        *)
          echo "Invalid response. Enter 'n' or 'y'."
          continue
          ;;
        esac
      done
      if ! ${proceed}; then
        continue
      fi

      # Push to a new branch
      echo "    Pushing updated branch..."
      # Checkout the new branch from the current HEAD
      if ! ${GIT} checkout -b "${new_branch}"; then
        echo "    ERROR: Failed to create new branch ${new_branch}"
        continue
      fi

      # Find existing open PR for the konflux branch using gh CLI
      pr_number=$(
        gh pr list \
          --repo "https://github.com/${repo}.git" \
          --state open \
          --head "${konflux_branch}" \
          --json number \
          --jq '.[0].number' 2>/dev/null
      )

      if [[ -z "${pr_number}" ]]; then
        echo "    WARNING: No open PR found for branch ${konflux_branch}"
        pr_closes=""
      else
        pr_closes="Closes #${pr_number}"
        echo "    Found open PR: #${pr_number} for branch ${konflux_branch}"
      fi

      # Push the new branch
      if ${GIT} push --set-upstream origin "${new_branch}"; then
        echo "    Successfully pushed changes to origin on branch ${new_branch}"
      else
        echo "    ERROR: Failed to push changes to origin"
        continue
      fi

      # Open a new PR using gh CLI if it doesn't already exist for this new branch
      pr_exists=$(gh pr list --repo "https://github.com/${repo}.git" --state open --head "${new_branch}" --json number --jq '.[0].number' 2>/dev/null)
      if [[ -n "${pr_exists}" ]]; then
        echo "    PR already exists for branch ${new_branch}: #${pr_exists}"
      else
        pr_title="Migrate Konflux configs for ACM ${CURRENT_VERSION}"
        pr_body=$(printf "Automated migration via [\`migrate-konflux.sh\`](https://github.com/stolostron/governance-policy-framework/blob/main/build/branch-create/migrate-konflux.sh) script.\n\n%s" "${pr_closes}")

        if gh pr create \
          --repo "https://github.com/${repo}.git" \
          --head "${new_branch}" \
          --base main \
          --title "${pr_title}" \
          --body "${pr_body}"; then
          echo "    Successfully created pull request for ${new_branch}"
        else
          echo "    ERROR: Failed to create pull request for ${new_branch}"
        fi
      fi
    else
      echo "    ERROR: Failed to amend commit"
    fi
  else
    echo "    No changes to commit"
  fi

  ((processed_count++))

  echo "Completed processing ${repo}"
  echo ""
done

echo "=============================================="
echo "Migration complete!"
echo "Processed: ${processed_count} repositories"
echo "Skipped: ${skipped_count} repositories"

echo "Open PRs:"
for repo in ${repos}; do
  pr_url=$(gh pr list --repo "${repo}" --state open --head "${new_branch}" --json url --jq '.[0].url' 2>/dev/null)
  if [[ -n "${pr_url}" ]]; then
    echo "* ${pr_url}"
  fi
done
