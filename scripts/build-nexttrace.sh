#!/bin/sh
set -eu

# Keep the source immutable and auditable. The commit is the v1.7.5 release.
nexttrace_version=v1.7.5
nexttrace_commit=7ff73b2c51f9f37a9bc1d6139b7053948f0250db
nexttrace_source_sha256=906a468b03044864e33d79a44075e5859557c64a619411019e0f861a80357c9e
nexttrace_source_url="https://codeload.github.com/nxtrace/NTrace-core/tar.gz/${nexttrace_commit}"

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)
patch_file="$repo_dir/patches/nexttrace-remove-file-output.patch"
output_path=${1:-/usr/local/bin/nexttrace}

for command in awk go install patch sha256sum tar wget; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "missing required command: $command" >&2
        exit 1
    fi
done

work_dir=$(mktemp -d)
case "$work_dir" in
    /tmp/*|/var/tmp/*) ;;
    *)
        echo "refusing unexpected temporary directory: $work_dir" >&2
        exit 1
        ;;
esac
cleanup() {
    rm -rf -- "$work_dir"
}
trap cleanup EXIT INT TERM

archive_path="$work_dir/nexttrace.tar.gz"
source_dir="$work_dir/source"
binary_path="$work_dir/nexttrace"

wget -q -O "$archive_path" "$nexttrace_source_url"
actual_sha256=$(sha256sum "$archive_path" | awk '{print $1}')
if [ "$actual_sha256" != "$nexttrace_source_sha256" ]; then
    echo "nexttrace $nexttrace_version source checksum mismatch" >&2
    echo "expected: $nexttrace_source_sha256" >&2
    echo "actual:   $actual_sha256" >&2
    exit 1
fi

mkdir "$source_dir"
tar -xzf "$archive_path" --strip-components=1 -C "$source_dir"
patch --batch --forward -p1 -d "$source_dir" < "$patch_file"

(
    cd "$source_dir"
    CGO_ENABLED="${CGO_ENABLED:-0}" go build -trimpath -ldflags '-w -s' -o "$binary_path" .
)

output_dir=$(dirname -- "$output_path")
mkdir -p "$output_dir"
install -m 0755 "$binary_path" "$output_path"
