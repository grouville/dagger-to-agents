#!/bin/sh

LOAD_ENV_ARG=""

# Check if the environment file prepared by GooseClient.Run exists
if [ -f "/tmp/path_to_happiness" ]; then
  LOAD_ENV_ARG="--env-file /tmp/path_to_happiness"
fi

/bin/dagger -m /target mcp --env-privileged $LOAD_ENV_ARG --export-env
