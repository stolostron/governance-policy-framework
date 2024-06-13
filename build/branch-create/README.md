# Automation tools for policy-grc-squad

## Updating the fast-forwarding version

- **Prerequisites**:
  - `yq` installed, `docker` installed and running
  - SSH access to GitHub
  - Write access to the repos
  - Fork and clone your fork of the [`release`](https://github.com/openshift/release) repo
- To disable fast-forwarding for GRC, set the `FAST_FORWARD` GitHub Actions variable in this repository 
  to "false". (This is not necessary when moving to a new release version.)

1. Update the version information at the base of the repo (Do not merge this until step 2 is merged.):
   ```shell
   OLD_VERSION=$(cat CURRENT_VERSION)
   printf X.Y > CURRENT_VERSION
   mv CURRENT_SUPPORTED_VERSIONS CURRENT_SUPPORTED_VERSIONS.bk
   { echo "${OLD_VERSION}"; head -2 CURRENT_SUPPORTED_VERSIONS.bk; } > CURRENT_SUPPORTED_VERSIONS
   rm CURRENT_SUPPORTED_VERSIONS.bk
   ```
   These commands script will:
   - Export the previous release version (also used in step 2)
   - Update the `CURRENT_VERSION` file to the new release version
   - Update `CURRENT_SUPPORTED_VERSION` with the new set of supported versions
2. Update existing and create new Prow configurations for the new version (see
   [CICD docs](https://github.com/stolostron/cicd-docs/blob/main/prow) for details on
   Prow):
   - Copy the absolute path to `update-release.sh`: `ls $PWD/build/branch-create/update-release.sh`
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
   - Once all PRs are validated and merged, delete each repo's `release` branches that were created
     (or delete your fork of the `release` repo) to prevent conflict with the next run of the
     script.
3. **After the prow configurations are updated, so that the prow jobs are triggered on the new branches**, 
   merge the `CURRENT_VERSION` update. `sync.sh` will create the new `release-*` branches and pick up the 
   new Prow configurations.
4. Check that fast-forwarding is re-enabled (the `FAST_FORWARD` GitHub Actions variable in this 
   repository should be set to "true")
