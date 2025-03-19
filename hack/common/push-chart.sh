#!/bin/bash

set -euo pipefail
source "$(realpath "$(dirname $0)/environment.sh")"

VERSION=$("$COMMON_SCRIPT_DIR/get-version.sh")
HELM_REGISTRY=$("$COMMON_SCRIPT_DIR/get-registry.sh" --helm)

echo
echo "> Uploading helm charts to $HELM_REGISTRY ..."
tmpdir="$("$COMMON_SCRIPT_DIR/get-tmp-dir.sh")"
for comp in ${COMPONENTS//,/ }; do
  chname="$(cat "$PROJECT_ROOT/charts/$comp/Chart.yaml" | $YAML2JSON | $JQ -r .name)"
	echo "> Pushing helm chart for component '$comp' ..." | indent 1
  "$HELM" push "$tmpdir/$chname-$VERSION.tgz" "oci://$HELM_REGISTRY" | indent 2
done
