#!/usr/bin/env bash

set -euo pipefail  # exit on errors and unset vars, and stop on the first error in a "pipeline"

# required:
: "${KUBECONFIG:?KUBECONFIG must be set}"

NAMESPACE=""
RESOURCE=""
CONDITION=""

# script configuration parameters:
MAX_STEP_DURATION="${MAX_STEP_DURATION:-180}"
MAX_DURATION="${MAX_DURATION:-360}"
WAIT_STEP="${WAIT_STEP:-15}"

while getopts "n:r:c:m:" option; do
    case "${option}" in 
        n)
            NAMESPACE="${OPTARG}"
            ;;
        r)
            RESOURCE="${OPTARG}"
            ;;
        c)
            CONDITION="${OPTARG}"
            ;;
        m)
            MAX_DURATION="${OPTARG}"
            ;;
    esac
done

: "${RESOURCE:?The resource (-r) parameter must be set}"

TOTAL_WAIT=0

# If a namespace is specified, wait for the namespace to exist
if [[ -n "${NAMESPACE}" ]]; then
    STEP_WAIT=0
    GOT_NAMESPACE=`kubectl get ns ${NAMESPACE} -o name || echo ""`
    until [[ -n "${GOT_NAMESPACE}" ]]; do
        if [[ "${TOTAL_WAIT}" -gt "${MAX_DURATION}" ]] || [[ "${STEP_WAIT}" -gt "${MAX_STEP_DURATION}" ]]; then
		    echo "Timed out waiting for namespace ${NAMESPACE}" >&2
            exit 1
        fi
        sleep "${WAIT_STEP}"
        (( STEP_WAIT += ${WAIT_STEP} ))
        (( TOTAL_WAIT += ${WAIT_STEP} ))
        GOT_NAMESPACE=`kubectl get ns ${NAMESPACE} -o name || echo ""`
    done
    KUBENS="--namespace=${NAMESPACE}"
else
    KUBENS=""
fi

# Wait for the main resource to exist
STEP_WAIT=0
GOT_RESOURCE=`kubectl ${KUBENS} get ${RESOURCE} -o name || echo ""`
echo wahit for the main $RESOURCE
until [[ -n "${GOT_RESOURCE}" ]]; do
    if [[ "${TOTAL_WAIT}" -gt "${MAX_DURATION}" ]] || [[ "${STEP_WAIT}" -gt "${MAX_STEP_DURATION}" ]]; then
        echo "Timed out waiting for resource ${RESOURCE}" >&2
        exit 1
    fi
    sleep "${WAIT_STEP}"
    (( STEP_WAIT += ${WAIT_STEP} ))
    (( TOTAL_WAIT += ${WAIT_STEP} ))
    GOT_RESOURCE=`kubectl ${KUBENS} get ${RESOURCE} -o name || echo ""`
done

# If a condition was provided, wait for it
if [[ -n "${CONDITION}" ]]; then
    (( MAX_DURATION -= TOTAL_WAIT ))
    if [[ "${MAX_DURATION}" -lt "${MAX_STEP_DURATION}" ]]; then
        WAIT_LENGTH="${MAX_DURATION}"
    else
        WAIT_LENGTH="${MAX_STEP_DURATION}"
    fi

    kubectl ${KUBENS} wait "--for=${CONDITION}" ${RESOURCE} "--timeout=${WAIT_LENGTH}s"
fi
