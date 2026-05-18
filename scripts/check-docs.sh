#!/usr/bin/env sh
set -eu

required_files="
README.md
CONTRIBUTING.md
docs/safety-model.md
docs/agent-usage.md
docs/project-spec.md
docs/response-code-reference.md
"

for file in $required_files; do
  if [ ! -f "$file" ]; then
    echo "missing required docs file: $file" >&2
    exit 1
  fi
done

required_links="
docs/project-spec.md
docs/safety-model.md
docs/agent-usage.md
CONTRIBUTING.md
docs/response-code-reference.md
"

for link in $required_links; do
  if ! grep -R "($link)" README.md docs CONTRIBUTING.md >/dev/null; then
    echo "required docs link not referenced: $link" >&2
    exit 1
  fi
done

required_commands="
authnet --version
authnet version
authnet paths
authnet config validate
authnet auth test
authnet profile list
authnet profile setup
authnet profile remove
authnet transaction get
authnet transaction list
authnet transaction unsettled list
authnet customer-profile get
authnet customer-profile list
authnet response-code explain
authnet sandbox
authnet completion
"

for command in $required_commands; do
  if ! grep -R "$command" README.md docs CONTRIBUTING.md >/dev/null; then
    echo "required command reference missing from docs: $command" >&2
    exit 1
  fi
done
