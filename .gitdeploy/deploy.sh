#!/bin/bash
set -e
set -u

if [[ "${GIT_REF_NAME}" != "master" ]]
then
    echo "Nothing to do for ${GIT_REPO_ID}#${GIT_REF_NAME}"
    exit 0
fi

echo "Deploying ${GIT_REPO_ID}#${GIT_REF_NAME} ..."

my_tmp="$(mktemp -d -t "tmp.XXXXXXXXXX")"
# See the Git Credentials Cheat Sheet
# https://coolaj86.com/articles/vanilla-devops-git-credentials-cheatsheet/
git clone --depth=1 "${GIT_CLONE_URL}" -b "${GIT_REF_NAME}" "${my_tmp}/${GIT_REPO_NAME}"
pushd "${my_tmp}/${GIT_REPO_NAME}/"

    # create xversion.go for local build
    go run -mod=vendor git.rootprojects.org/root/go-gitver/v2
    go generate -mod=vendor ./...
    go build -mod=vendor .
    rm xversion.go

    # TODO
    #goreleaser --rm-dist
    #webi gitdeploy

    mkdir -p ~/.local/bin/
    rsync -av ./gitdeploy ~/.local/bin/

    # restart system service
    sudo systemctl restart gitdeploy

popd

rm -rf "${my_tmp}/${GIT_REPO_NAME}/"
