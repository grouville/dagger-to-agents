#!/bin/sh

LOAD_ENV_ARG=""
# Check if the environment file prepared by GooseClient.Run exists
if [ -f "/tmp/path_to_happiness" ]; then
  LOAD_ENV_ARG="--load-env /tmp/path_to_happiness"
fi

# Pipe stdin to dagger mcp, now including the conditional LOAD_ENV_ARG
# The OPENAI_API_KEY is kept as per the original script.
# Debug logs are maintained.
tee /tmp/debug.stdin.log | \
  OPENAI_API_KEY=toto dagger -m /target mcp --env-privileged $LOAD_ENV_ARG \
  2>/tmp/debug.stderr.log | /usr/bin/tee /tmp/debug.stdout.log
