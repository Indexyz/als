#!/bin/sh
set -eu

nexttrace_arch=$(uname -m)
case "$nexttrace_arch" in
    aarch64) nexttrace_arch=arm64 ;;
    x86_64) nexttrace_arch=amd64 ;;
esac

# Select the full nexttrace binary; releases also contain tiny and ntr variants.
mkdir -p /usr/local/bin
wget -O /usr/local/bin/nexttrace \
    "https://github.com/nxtrace/NTrace-core/releases/latest/download/nexttrace_linux_${nexttrace_arch}"
chmod 755 /usr/local/bin/nexttrace

sh -e "$(dirname "$0")/install-speedtest.sh"
