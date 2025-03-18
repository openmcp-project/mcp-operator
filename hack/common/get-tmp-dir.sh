#!/bin/bash

set -euo pipefail
source "$(realpath "$(dirname $0)/environment.sh")"

# Creates a directory within the temporary folder (either TMPDIR or /tmp) and returns its path.

tmpdir="${TMPDIR:-"/tmp"}"
tmpdir="${tmpdir%/}/mcp/$(basename "$PROJECT_ROOT")"
mkdir -p "$tmpdir"
echo "$tmpdir"