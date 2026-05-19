#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
dist_dir="$repo_root/dist"
checksums_file="$dist_dir/checksums.txt"

if [ ! -f "$checksums_file" ]; then
  echo "missing checksum file: $checksums_file" >&2
  exit 1
fi

if [ ! -s "$checksums_file" ]; then
  echo "checksum file is empty: $checksums_file" >&2
  exit 1
fi

printf 'Validating checksums in %s\n' "$checksums_file"

(
  cd "$dist_dir"
  shasum -a 256 -c --strict "checksums.txt"
)
