# Automation tools for policy-grc-squad

## Releasing a new version upstream

- **Prerequisites**:
  - `jq` installed
  - Write access to the repos

1. Export the new `vX.Y.Z` version to be released to `NEW_RELEASE`.
2. Run `upstream-release.sh`.

## Refreshing builds with a no-op PR

- **Prerequisites**:
  - `yq` installed
  - Write access to the repos

1. Update `repo.txt` to list the repos that require a refresh.
2. Run the `refresh.sh` script. (It will update the "Date" comment in the README of each repo and
   push a new branch to the repo with the updates and provide a URL to open the PRs.)

## Updating the OWNERS files

- **Prerequisites**:
  - `yq` installed
  - Write access to the repos

1. Add or remove an owner to the `OWNERS` files of all repos (these can both be exported on the same run of the script):
   - To add an owner to all repos: `export NEW_OWNER=<github-user-id>`
   - To remove an owner from all repos: `export DELETE_OWNER=<github-user-id>`
2. Change to the `main-branch-sync/` directory.
3. Either:
   - Verify the repos listed in `repo.txt`
   - Use the `fetch-repo-list.sh` script to dynamically fetch a list of repos for a team (the script defaults to team
     `sig-policy` in org `open-cluster-management-io`):
     ```shell
     export GITHUB_TOKEN=<github-token>
     export GITHUB_ORG=<org-name>       # Exporting this is mandatory for `update-owners.sh` if the value is not "stolostron"
     export REPOS=$(./fetch-repo-list.sh)
     ```
4. Run the `update-owners.sh` script. (It will update the files in each repo and push a new branch to the repo with the
   updates and then provide a URL to open the PRs.)

## Rotating CI secrets

- **Prerequisites**:
  - CLIs installed: `gh`, `aws`, `oc`, `jq`
  - Log in to the Collective cluster
  - Log in to GitHub with username `acm-grc-security`

1. Run the `rotate-secrets.sh` script. (It will prompt for the new GitHub and SonarCloud tokens,
   regenerate the AWS token, rotate the Collective tokens, rotate the GitHub tokens, and provide
   manual steps to update tokens in Prow, Travis, and Bitwarden.)
