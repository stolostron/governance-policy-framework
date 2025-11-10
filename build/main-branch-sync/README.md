# Automation tools for policy-grc-squad

## Releasing a new version upstream

- **Prerequisites**:
  - `jq` installed
  - Write access to the repos

1. Export the new `vX.Y.Z` version to be released to `NEW_RELEASE`.
2. Run [`upstream-release.sh`](./upstream-release.sh).
3. Follow the directions printed to the screen by the script.

## Performing an update across all repos

- **Prerequisites**:
  - Write access to the repos

Run [`repo-bulk-update.sh`](./repo-bulk-update.sh) with the positional argument you'd like to use
for the updates, selecting from scripts in [`sync-utils/`](./sync-utils/) while omitting the `.sh`
extension. For example, to sync common build files to both upstream and `stolostron`, run this
command from the base of the repo (NOTE: running it from the base of the repo is not required--the
script is location agnostic):

```bash
build/main-branch-sync/repo-bulk-update.sh --upstream --commit-msg "chore: sync common build files" common-file-sync
```

Or, to run custom commands that may only be needed one time, update the
[`sync-utils/custom-sync.sh`](./sync-utils/custom-sync.sh) script or, for the Konflux pipeline files
the [`sync-utils/konflux-update.sh`](./sync-utils/konflux-update.sh) script, with the commands you'd
like to run. You can test them out with the `--dry-run` flag to view the diff without committing
changes. For example:

```bash
build/main-branch-sync/repo-bulk-update.sh --dry-run custom-sync
```

Or, for Konflux pipeline file updates:

```bash
build/main-branch-sync/repo-bulk-update.sh --dry-run konflux-update
```

You can view the help menu by passing the `--help` flag:

```bash
build/main-branch-sync/repo-bulk-update.sh --help
```

## Refreshing builds with a no-op PR

- **Prerequisites**:
  - `yq` installed
  - Write access to the repos

1. Update [`repo.txt`](./repo.txt) to list the repos that require a refresh.
2. Run the [`refresh.sh`](./refresh.sh) script. (It will update the "Date" comment in the README of
   each repo and push a new branch to the repo with the updates and provide a URL to open the PRs.)

## Updating the OWNERS files

- **Prerequisites**:
  - `yq` installed
  - Write access to the repos

1. Add or remove an owner to the `OWNERS` files of all repos (these can both be exported on the same
   run of the script):
   - To add an owner to all repos: `export NEW_OWNER=<github-user-id>`
   - To remove an owner from all repos: `export DELETE_OWNER=<github-user-id>`
2. Run the [`update-owners.sh`](./update-owners.sh) script. (It will update the files in each repo
   and push a new branch to the repo with the updates and then provide a URL to open the PRs.)

## Rotating CI secrets

- **Prerequisites**:
  - CLIs installed: `gh`, `aws`, `oc`, `jq`
  - Log in to the Collective cluster
  - Log in to GitHub with username `acm-grc-security`

1. Run the [`rotate-secrets.sh`](./rotate-secrets.sh) script. (It will prompt for the new GitHub and
   SonarCloud tokens, regenerate the AWS token, rotate the Collective tokens, rotate the GitHub
   tokens, and provide manual steps to update tokens in Prow, Travis, and Bitwarden.)
