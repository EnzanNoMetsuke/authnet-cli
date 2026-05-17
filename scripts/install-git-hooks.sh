#!/usr/bin/env sh
set -eu

git config core.hooksPath .githooks
chmod +x .githooks/pre-commit

echo "Git hooks installed from .githooks"
