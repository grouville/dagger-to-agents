#!/usr/bin/env bash

set -ueo pipefail

function f() {
	pids=
	while read line; do
		pids="$pids $line"
	done
	if [[ -z $pids ]]; then
		echo "nothing to kill"
		return
	fi
	set -x
	kill -9 $pids
}
ps -eo pid,pgid,args | awk '$1 != "PID" && $2 != "1" && $2 != "'$PPID'" && $2 != "'$$'" {print $0}' | tee /dev/tty | awk '{print $1}' | f
