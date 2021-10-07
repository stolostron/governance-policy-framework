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
ssh "${OPT[@]}" "${HOST}" "mkdir -p /tmp/build/ /tmp/test/"
scp "${OPT[@]}" Makefile "${HOST}:/tmp/Makefile"
scp "${OPT[@]}" build/wait_for.sh "${HOST}:/tmp/build/wait_for.sh"
scp "${OPT[@]}" build/run-e2e-tests.sh "${HOST}:/tmp/build/run-e2e-tests.sh"
scp -r "${OPT[@]}" test "${HOST}:/tmp/test"

# Run the KinD script on the KinD instance
echo "* Running E2E script on Kind cluster..."
KIND_COMMAND="cd /tmp && export deployOnHub=${deployOnHub} && ./build/run-e2e-tests.sh"
ssh "${OPT[@]}" "${HOST}" "${KIND_COMMAND}" > >(tee "${ARTIFACT_DIR}/test-e2e.log") 2>&1 || ERROR_CODE=$?

# Copy any debug logs
echo "* Checking for debug logs..."
if ssh "${OPT[@]}" "${HOST}" "ls /tmp/test-output/debug"; then
  scp -r "${OPT[@]}" "${HOST}:/tmp/test-output/*" ${ARTIFACT_DIR}
fi

# Manually exit in case an exit code was captured
exit ${ERROR_CODE}
