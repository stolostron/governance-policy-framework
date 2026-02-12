# New release tools

- [New release tools](#new-release-tools)
  - [Updating the fast-forwarding version](#updating-the-fast-forwarding-version)
  - [Handling new Konflux configurations](#handling-new-konflux-configurations)

## Updating the fast-forwarding version

- **Prerequisites**:
  - `yq` installed, `docker` installed and running
  - SSH access to GitHub
  - Write access to the repos
  - Fork and clone your fork of the [`release`](https://github.com/openshift/release) repo

1. Update the version information at the base of the repo (Do not merge this until step 2 is
   merged.):

   ```shell
   NEW_VERSION=X.Y
   ```

   ```shell
   OLD_VERSION=$(cat CURRENT_VERSION)
   printf ${NEW_VERSION} > CURRENT_VERSION
   mv CURRENT_SUPPORTED_VERSIONS CURRENT_SUPPORTED_VERSIONS.bk
   { echo "${OLD_VERSION}"; cat CURRENT_SUPPORTED_VERSIONS.bk; } > CURRENT_SUPPORTED_VERSIONS
   rm CURRENT_SUPPORTED_VERSIONS.bk
   ```

   These commands script will:
   - Store the previous release version (also used in step 2)
   - Update the `CURRENT_VERSION` file to the new release version
   - Update `CURRENT_SUPPORTED_VERSION` with the new set of supported versions

2. Check the
   [RHACM Lifecycle page](https://access.redhat.com/support/policy/updates/advanced-cluster-management#lifecycle-dates)
   and remove any versions from the CURRENT_SUPPORTED_VERSIONS file that are not currently
   supported.

3. Update existing and create new Prow configurations for the new version (see
   [CICD docs](https://github.com/stolostron/cicd-docs/blob/main/prow) for details on Prow):
   - Copy the absolute path to `update-release.sh`:

     ```shell
     ls ${PWD}/build/branch-create/update-release.sh
     ```

   - Change to the local directory for the [`release`](https://github.com/openshift/release) repo
   - Run the `update-release.sh` script using the path you copied.
     - **Notes**: The `update-release.sh` script will:
       - Create a new Prow configuration for the new version and update existing configurations
         accordingly
       - Run `make update` to validate the configurations and generate the jobs (which takes a few
         minutes but should not be interrupted)
       - Create a branch for each repo's new configuration and display a URL to open a PR
   - Use the URLs in the output to open new PRs (reviewers should be assigned automatically once the
     PR is opened).
     - **Notes for reviewers**: Rehearsals are not required to pass for the PR to merge. Some
       rehearsals will likely fail because it uses references from the current `release` repo's PR
       but applies them to the target repo, where they don't match up. These failures can be
       ignored, and failures that appear to be environmental should be re-run. Other failures (like
       if a unit test fails) should be investigated and resolved before approving and merging.
   - Once all PRs are validated and merged, delete the `ocm-new-grc-release-X.Y` branch for the
     `release` repo that was created (or delete your fork of the `release` repo) to prevent conflict
     with the next run of the script.

4. Set the `FAST_FORWARD` GitHub Actions variable in this repository to "false" to disable
   fast-forwarding for GRC so that the new Konflux configurations are not inadvertently
   fast-forwarded to the previous release branch.
5. Run the script in [Handling new Konflux configurations](#handling-new-konflux-configurations) to
   create new PRs to update the Konflux configurations.
6. **After the Prow and Konflux configurations are updated**, re-run the CI and merge the
   `CURRENT_VERSION` update. `sync.sh` will create the new `release-*` branches and pick up the new
   Prow configurations.
7. Set the `FAST_FORWARD` GitHub Actions variable in this repository to "true" to re-enable
   fast-forwarding.

## Handling new Konflux configurations

Once Konflux PRs land in our repositories as a result of the creation of the new ACM Konflux
application and components, run the [`migrate-konflux.sh`](./migrate-konflux.sh) script:

```shell
./build/branch-create/migrate-konflux.sh
```

The script will iterate over [`repo.txt`](./repo.txt), search for the newly created Konflux
branches, update the configuration based on the existing release configuration, and create a new PR
with a "closes" keyword to close the existing PR from Konflux on merge.
