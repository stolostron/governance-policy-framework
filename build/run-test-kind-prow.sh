#! /bin/bash

# Fix the key's permissions
KEY="${SHARED_DIR}/private.pem"
chmod 400 "${KEY}"

# Create variables used by ssh and scp
IP="$(cat "${SHARED_DIR}/public_ip")"
HOST="ec2-user@$IP"
OPT=(-q -o "UserKnownHostsFile=/dev/null" -o "StrictHostKeyChecking=no" -i "${KEY}")

# Save the contents of $IMAGE_REF to a file on the KinD instance
ssh "${OPT[@]}" "${HOST}" "echo ${IMAGE_REF} > /tmp/image_ref"

# Set the environment on the KinD instance
echo "* Copying over test files..."
# We have to use a subdirectory since Go refuses to use a 'go.mod' file in '/tmp'
WORK_DIR=/tmp/governance-policy-framework
ssh "${OPT[@]}" "${HOST}" "mkdir -p ${WORK_DIR}/build/"
scp "${OPT[@]}" go.mod "${HOST}:${WORK_DIR}/"
scp "${OPT[@]}" go.sum "${HOST}:${WORK_DIR}/"
scp "${OPT[@]}" Makefile "${HOST}:${WORK_DIR}/"
scp "${OPT[@]}" build/wait_for.sh "${HOST}:${WORK_DIR}/build/"
scp "${OPT[@]}" build/run-e2e-tests.sh "${HOST}:${WORK_DIR}/build/"
scp -r "${OPT[@]}" deploy/ "${HOST}:${WORK_DIR}/"
scp -r "${OPT[@]}" test/ "${HOST}:${WORK_DIR}/"

# Run the KinD script on the KinD instance
echo "* Running E2E script on Kind cluster..."
KIND_COMMAND="cd ${WORK_DIR} && deployOnHub=${deployOnHub} CGO_ENABLED=0 ./build/run-e2e-tests.sh"
ssh "${OPT[@]}" "${HOST}" "${KIND_COMMAND}" > >(tee "${ARTIFACT_DIR}/test-e2e.log") 2>&1 || ERROR_CODE=$?

# Copy any debug logs
if [[ -n "${ERROR_CODE}" ]]; then
  echo "* Checking for debug logs..."
  scp -r "${OPT[@]}" "${HOST}:${WORK_DIR}/test-output/" ${ARTIFACT_DIR}/
fi

# Manually exit in case an exit code was captured
exit ${ERROR_CODE}
