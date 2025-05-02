#!/bin/sh

tee /tmp/debug.stdin.log | OPENAI_API_KEY=toto dagger -m /target mcp --env-privileged 2>/tmp/debug.stderr.log | /usr/bin/tee /tmp/debug.stdout.log &
wait
