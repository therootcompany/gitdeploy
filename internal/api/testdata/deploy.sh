#!/bin/bash
set -e
set -u
#set -x

echo "[${GIT_REPO_ID:-}#${GIT_REF_NAME:-}] Started at ${GIT_DEPLOY_TIMESTAMP:-}"
sleep ${GIT_DEPLOY_TEST_WAIT:-0.1}
echo "[${GIT_REPO_ID:-}#${GIT_REF_NAME:-}] Finished"

echo "Reporting to '${GIT_DEPLOY_CALLBACK_URL:-}' ..."
#curl -fsSL "${GIT_DEPLOY_CALLBACK_URL}" \
curl -fsSL "${GIT_DEPLOY_CALLBACK_URL}?format=pytest" \
    -H 'Content-Type: application/json' \
    -d '
    { "exitcode": 0,
      "root": "/home/app/srv/status.example.com/e2e-selenium",
      "tests": [
        { "nodeid": "pytest::idthing",
          "outcome": "passed"
        }
      ]
    }
'
#    -d '
#    { "report":
#        { "name": "sleep test",
#          "status": "PASS",
#          "message": "a top level result group",
#          "results": [
#              { "name": "sub test", "status": "PASS", "message": "a sub group", "detail": "logs or smth" }
#          ]
#        }
#    }
#'

echo "[${GIT_REPO_ID:-}#${GIT_REF_NAME:-}] Generated Report"
