#!/bin/bash
set -u

if [[ "${GIT_REF_NAME}" != "master" ]]
then
  echo "Nothing to do for ${GIT_REPO_ID}#${GIT_REF_NAME}"
  exit 0
fi

echo "Deploying ${GIT_REPO_ID}#${GIT_REF_NAME} ..."

my_tmp="$(mktemp -d -t "tmp.XXXXXXXXXX")"
git clone "${GIT_CLONE_URL}" "${my_tmp}/${GIT_REPO_NAME}"
pushd "${my_tmp}/${GIT_REPO_NAME}/"
  go generate -mod=vendor ./...
  go build -mod=vendor .
  mkdir -p ~/.local/bin/
  rsync -av ./gitdeploy ~/.local/bin/

  sudo systemctl restart gitdeploy
popd