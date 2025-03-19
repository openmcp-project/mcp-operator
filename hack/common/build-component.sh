#!/bin/bash

set -euo pipefail
source "$(realpath "$(dirname $0)/environment.sh")"

VERSION=$("$COMMON_SCRIPT_DIR/get-version.sh")
CHART_REGISTRY=$("$COMMON_SCRIPT_DIR/get-registry.sh" --helm)
IMG_REGISTRY=$("$COMMON_SCRIPT_DIR/get-registry.sh" --image)

pushd ${PROJECT_ROOT} > /dev/null 2>&1
COMMIT="$(git rev-parse HEAD)"
popd > /dev/null 2>&1

echo
echo "> Building component in version ${CD_VERSION:-$VERSION} ..."
compdir="$("$COMMON_SCRIPT_DIR/get-tmp-dir.sh")/component"
$OCM add componentversions --file "$compdir" --version "$VERSION" --create --force --templater spiff "$COMPONENT_DEFINITION_FILE" -- \
  VERSION="$VERSION" \
  CHART_REGISTRY="$CHART_REGISTRY" \
  IMG_REGISTRY="$IMG_REGISTRY" \
  COMMIT="$COMMIT" \
  COMPONENTS="$COMPONENTS" \
  ${CD_VERSION:+CD_VERSION=}${CD_VERSION:-} \
  ${CHART_VERSION:+CHART_VERSION=}${CHART_VERSION:-} \
  ${IMG_VERSION:+IMG_VERSION=}${IMG_VERSION:-} \
  ${BP_COMPONENTS:+BP_COMPONENTS=}${BP_COMPONENTS:-} \
  ${CHART_COMPONENTS:+CHART_COMPONENTS=}${CHART_COMPONENTS:-} \
  ${IMG_COMPONENTS:+IMG_COMPONENTS=}${IMG_COMPONENTS:-} \
  | indent 1

echo "Use '$(realpath --relative-base=$(pwd) $OCM) get cv $compdir -o yaml' to view the generated component descriptor."