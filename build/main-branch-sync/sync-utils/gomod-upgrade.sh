#! /bin/bash

set -e

if [[ -z "${REPO_PATH}" ]]; then
  echo "error: REPO_PATH is not set" >&2
  exit 1
fi

desired_go_version="$1"

if [[ -z "${desired_go_version}" ]]; then
  echo "error: specifying a X.Y Go version positional argument is required" >&2
  exit 1
fi

cd "${REPO_PATH}" || exit 1

if [[ -f ${REPO_PATH}/go.mod ]]; then
  ${SED} -i -E 's/^go 1\.[0-9]+\.[0-9]+$/go '"${desired_go_version}"'.0/' go.mod

  go mod tidy || {
    echo "error: Failed to tidy go.mod in ${REPO_PATH}" >&2
    read -r -p "Take care of upgrade errors. Press enter to continue"
  }

  make fmt || true
  make lint || {
    echo "error: Failed to lint in ${REPO_PATH}" >&2
    read -r -p "Take care of linting errors. Press enter to continue"
  }
fi

for pkg in $(go mod edit -json go.mod | jq -r ".Require[].Path"); do
  if [[ ${pkg} == "k8s.io/"* ]]; then
    continue
  fi

  echo "== Handling package ${pkg}"
  current_version="$(go mod edit -json go.mod | jq -r ".Require[] | select(.Path == \"${pkg}\") | .Version")"
  versions="$(curl -s https://proxy.golang.org/${pkg}/@v/list | { grep -v "+incompatible\|alpha.\|beta.\|rc." || true; } | sort --version-sort -r)"
  retractions=""

  for version in latest ${versions}; do
    echo "-- Looking up ${pkg}@${version}"
    pkg_json="$(go list -m -json "${pkg}@${version}")"
    pkg_version="$(echo "${pkg_json}" | jq -r '.Version')"
    go_mod="$(echo "${pkg_json}" | jq -r '.GoMod')"
    gomod_json="$(go mod edit -json "${go_mod}")"
    go_version="$(echo "${gomod_json}" | jq -r '.Go')"

    if [[ "$(echo "${gomod_json}" | jq '.Retract | length > 0')" == "true" ]]; then
      retractions="$(echo "${gomod_json}" | jq '.Retract[].High')"
    fi

    skip="$(echo "${retractions}" | grep "${version}" || true)"

    if [[ "${go_version}" != "${desired_go_version}"* ]] && [[ -z "${skip}" ]]; then
      if [[ "${version}" == "latest" ]]; then
        if [[ "${pkg_version}" == "${current_version}" ]]; then
          echo "* Already at latest version. Continuing."

          break
        fi

        higher_version="$(printf "%s\n%s\n" "${current_version}" "${pkg_version}" | sort --version-sort | tail -1)"

        if [[ "${higher_version}" == "${current_version}" ]]; then
          echo "* Current version ${current_version} is higher than latest version ${pkg_version}. Continuing."

          break
        fi
      fi

      echo "* Getting ${pkg}@${version}"
      go get "${pkg}@${version}" || read -r -p "Take care of upgrade errors. Press enter to continue"

      break
    fi

    echo "x Skipping ${pkg}@${version}"
  done
done

go mod tidy || {
  echo "error: Failed to tidy go.mod in ${REPO_PATH}" >&2
  read -r -p "Take care of upgrade errors. Press enter to continue"
}
make fmt || true
make lint || {
  echo "error: Failed to lint in ${REPO_PATH}" >&2
  read -r -p "Take care of linting errors. Press enter to continue"
}
