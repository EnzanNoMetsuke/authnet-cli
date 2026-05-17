#!/usr/bin/env sh
set -eu

version="${1:-v2.12.2}"
bindir="${2:-./bin}"
tmpfile="$(mktemp)"
trap 'rm -f "$tmpfile"' EXIT

mkdir -p "$bindir"

curl -sSfL https://golangci-lint.run/install.sh -o "$tmpfile"
sh "$tmpfile" -b "$bindir" "$version"
"$bindir/golangci-lint" --version
