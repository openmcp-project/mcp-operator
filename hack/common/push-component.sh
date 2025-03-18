#!/bin/bash

set -euo pipefail
source "$(realpath "$(dirname $0)/environment.sh")"

VERSION=$("$COMMON_SCRIPT_DIR/get-version.sh")
COMPONENT_REGISTRY="$($COMMON_SCRIPT_DIR/get-registry.sh --component)"

overwrite=""
if [[ -n ${OVERWRITE_COMPONENTS:-} ]] && [[ ${OVERWRITE_COMPONENTS} != "false" ]]; then
  overwrite="--overwrite"
fi

echo
echo "> Uploading Component Descriptors to $COMPONENT_REGISTRY ..."
compdir="$("$COMMON_SCRIPT_DIR/get-tmp-dir.sh")/component"
$OCM transfer componentversions "$compdir" "$COMPONENT_REGISTRY" $overwrite | indent 1
