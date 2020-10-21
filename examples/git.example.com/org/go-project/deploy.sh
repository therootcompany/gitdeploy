#!/bin/bash
set -u
set -e

if [[ "${GIT_REF_NAME}" != "master" ]]
then
  echo "Nothing to do for ${GIT_REPO_ID}#${GIT_REF_NAME}"
  exit 0
fi

echo "Deploying ${GIT_REPO_ID}#${GIT_REF_NAME} ..."

# See the Git Credentials Cheat Sheet
# https://coolaj86.com/articles/vanilla-devops-git-credentials-cheatsheet/
my_tmp="$(mktemp -d -t "tmp.XXXXXXXXXX")"
git clone --depth=1 "${GIT_CLONE_URL}" -b "${GIT_REF_NAME}" "${my_tmp}/${GIT_REPO_NAME}"
pushd "${my_tmp}/${GIT_REPO_NAME}/"
  go generate -mod=vendor ./...
  go build -mod=vendor .

  mkdir -p ~/.local/bin/
  rsync -av ./${GIT_REPO_NAME} ~/.local/bin/

  sudo systemctl restart ${GIT_REPO_NAME}
popd

rm -rf "${my_tmp}/${GIT_REPO_NAME}/"
