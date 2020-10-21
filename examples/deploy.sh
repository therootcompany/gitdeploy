#!/bin/bash

# The directory of this bash script
base_dir="$(dirname "$(readlink -f "$0")")"

function deploy_local() {
    echo "Running deplay script for ${GIT_REPO_ID}"
    bash -o errexit -o nounset "${base_dir}/${GIT_REPO_ID}/deploy.sh"
}

function deploy_trusted() {
    my_tmp="$(mktemp -d -t "tmp.XXXXXXXXXX")"
    git clone --depth=1 "${GIT_CLONE_URL}" -b "${GIT_REF_NAME}" "${my_tmp}/${GIT_REPO_NAME}"

    pushd "${my_tmp}/${GIT_REPO_NAME}"
        if [[ -f ".gitdeploy/deploy.sh" ]]
        then
            bash -o errexit -o nounset ".gitdeploy/deploy.sh"
        else
            echo "Missing ${GIT_REPO_ID}/.gitdeploy/deploy.sh"
        fi
    popd

    rm -rf "${my_tmp}/${GIT_REPO_NAME}/"
}

function show_help() {
    echo ""
    echo "Nothing to do for ${GIT_REPO_ID}"
    echo ""
    echo "Want to set it up? Try this:"
    echo "    mkdir -p ${base_dir}/${GIT_REPO_ID}"
    echo "    rsync -av ${base_dir}/git.example.com/org/project/ ${base_dir}/${GIT_REPO_ID}/"
    echo ""
    echo "Then edit the example deploy.sh to do what you need."
    echo "    vim ${base_dir}/${GIT_REPO_ID}/deploy.sh"
    echo ""
    echo "You may also like to take a look at the Go, Node.js, and other starter templates:"
    echo "    ls ${base_dir}/git.example.com/org/"
    echo ""
    echo "You can use any of these ENVs in your deploy script:"

    # These environment variables are set by the caller
    my_envs='GIT_REPO_ID
    GIT_CLONE_URL
    GIT_REPO_OWNER
    GIT_REPO_NAME
    GIT_REF_TYPE
    GIT_REF_NAME
    GIT_REPO_TRUSTED
    '
    for x in $my_envs; do
        echo "$x=${!x}"
    done

    sleep 1
}

if [[ -f "${base_dir}/${GIT_REPO_ID}/deploy.sh" ]]; then
    deploy_local
    exit 0
elif [[ "true" == "${GIT_REPO_TRUSTED}" ]]; then
    deploy_trusted
    exit 0
else
    show_help
    exit 1
fi
