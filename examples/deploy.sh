#!/bin/bash

# The directory of this bash script
base_dir="$(dirname "$(readlink -f "$0")")"

if [[ -f "${base_dir}/${GIT_REPO_ID}/deploy.sh" ]]
then
  echo "Running deplay script for ${GIT_REPO_ID}"
  bash "${base_dir}/${GIT_REPO_ID}/deploy.sh"
  exit 0
fi

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
my_envs='GIT_REF_NAME
GIT_REF_TYPE
GIT_REPO_ID
GIT_REPO_OWNER
GIT_REPO_NAME
GIT_CLONE_URL'
for x in $my_envs; do
    echo "$x=${!x}"
done

sleep 1
