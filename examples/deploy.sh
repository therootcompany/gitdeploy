#!/bin/bash

# These environment variables are set by the caller
my_envs='GIT_REF_NAME
GIT_REF_TYPE
GIT_REPO_ID
GIT_REPO_OWNER
GIT_REPO_NAME
GIT_CLONE_URL'

# The directory of this bash script
base_dir="$(dirname "$(readlink -f "$0")")"

if [[ -f "${base_dir}/${GIT_REPO_ID}/deploy.sh" ]]
then
  echo "Running deplay script for ${GIT_REPO_ID}"
  bash "${base_dir}/${GIT_REPO_ID}/deploy.sh"
else
  echo "Nothing to do for ${GIT_REPO_ID}"
  for x in $my_envs; do
      echo "$x=${!x}"
  done
  sleep 1
fi
