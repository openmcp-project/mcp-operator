#!/bin/bash

set -euo pipefail
source "$(realpath "$(dirname $0)/environment.sh")"

VERSION=$("$COMMON_SCRIPT_DIR/get-version.sh")

echo
echo "> Packaging helm charts to prepare for upload ..."
tmpdir="$("$COMMON_SCRIPT_DIR/get-tmp-dir.sh")"
for comp in ${COMPONENTS//,/ }; do
	echo "> Packaging helm chart for component '$comp' ..." | indent 1
  "$HELM" package "$PROJECT_ROOT/charts/$comp" -d "$tmpdir" --version "$VERSION" | indent 2 # file name is <chart name>-<chart version>.tgz (derived from Chart.yaml)
done
