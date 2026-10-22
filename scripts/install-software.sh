#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
"$script_dir/build-nexttrace.sh" /usr/local/bin/nexttrace
sh "$script_dir/install-speedtest.sh"
