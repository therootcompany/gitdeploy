#!/bin/bash

for x in $@; do
    echo "$x"
done

my_envs='GIT_DEPLOY_JOB_ID
GIT_DEPLOY_PROMOTE_TO
GIT_REF_NAME
GIT_REF_TYPE
GIT_REPO_OWNER
GIT_REPO_NAME
GIT_CLONE_URL'

echo 'Doing "work" ...'
sleep 5

for x in $my_envs; do
    echo "$x=${!x}"
done
