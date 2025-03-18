#!/bin/bash

set -euo pipefail
source "$(realpath "$(dirname $0)/environment.sh")"

echo "> Checking if documentation index needs changes"
doc_index_file="$PROJECT_ROOT/docs/README.md"
tmp_compare_file=$(mktemp)
"$COMMON_SCRIPT_DIR/generate-docs-index.sh" "$tmp_compare_file" >/dev/null
if ! cmp -s "$doc_index_file" "$tmp_compare_file"; then
  echo "The documentation index requires changes."
  echo "Please run 'make generate-docs' to update it."
  exit 1
fi
echo "Documentation index is up-to-date."