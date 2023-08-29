#!/usr/bin/env bash

RUNNER_HOME=/home/runner/work/_temp/_github_home

# Copy over any credentials from the Actions home directory that may have been
# initialized via docker/login-action
if [[ -f ${HOME}/.docker/config.json ]]; then
  mkdir -p ${RUNNER_HOME}/.docker
  cp ${HOME}/.docker/config.json ${RUNNER_HOME}/.docker
fi
