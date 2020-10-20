#!/bin/bash
set -u
set -e

my_mirror=git.example.com/org/project
echo "Mirroring ${GIT_REPO_ID}#${GIT_REF_NAME} to $my_mirror ..."

# See the Git Credentials Cheat Sheet
# https://coolaj86.com/articles/vanilla-devops-git-credentials-cheatsheet/
if [[ ! -d "$HOME/git-mirrors/${GIT_REPO_ID}" ]]; then
    mkdir -p "$(dirname "$HOME/git-mirrors/${GIT_REPO_ID}")"
    git clone "${GIT_CLONE_URL}" "$HOME/git-mirrors/${GIT_REPO_ID}"
    pushd "$HOME/git-mirrors/${GIT_REPO_ID}"
      git remote add mirror ssh://git@${my_mirror}.git
      git fetch --all
    popd
fi

pushd "${HOME}/git-mirrors/${GIT_REPO_ID}"
    # get a random branch name
    my_tmp="$(xxd -l8 -ps /dev/urandom)"
    # get clean
    git reset --hard HEAD
    # make a temp branch
    git checkout -b "${my_tmp}"
    # delete the existing branch, if any
    git branch -D "${GIT_REF_NAME}"
    # get updated
    git fetch --force origin
    # get the latest branch
    git checkout --force "origin/${GIT_REF_NAME}" -b "${GIT_REF_NAME}"
    # delete the temp branch
    git branch -D "${my_tmp}"
    # get the latest of this branch
    git pull --force
    # overwrite the mirror's copy
    git push --force mirror "${GIT_REF_NAME}"
popd
