#!/bin/bash
set -e
set -u
set -x

echo "[${GIT_REPO_ID}#${GIT_REF_NAME}] Started at ${GIT_DEPLOY_TIMESTAMP}"
sleep 0.5
echo "[${GIT_REPO_ID}#${GIT_REF_NAME}] Finished"

# TODO start/end? duration?
#curl -X POST "${GIT_DEPLOY_CALLBACK_URL}" -d '
#    { "report": [ { "name":"sleep", "code":"PASS", "message":"", "details":"" } ] }
#'
