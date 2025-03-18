#!/bin/bash

set -euo pipefail
source "$(realpath "$(dirname $0)/environment.sh")"

VERSION=$1

GO_MOD_FILE="${PROJECT_ROOT}/go.mod"

# update VERSION file
echo "$VERSION" > "$PROJECT_ROOT/VERSION"

for comp in ${COMPONENTS//,/ }; do
  CHART_FILE="${PROJECT_ROOT}/charts/${comp}/Chart.yaml"
  CHART_VALUES_FILE="${PROJECT_ROOT}/charts/${comp}/values.yaml"

  # update version and image tag in helm charts
  sed -E -i -e "s@version: v?[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+.*@version: $VERSION@1" "${CHART_FILE}"
  sed -E -i -e "s@appVersion: v?[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+.*@appVersion: $VERSION@1" "${CHART_FILE}"
  sed -i -e "s@  tag: .*@  tag: ${VERSION}@" "${CHART_VALUES_FILE}"

  # remove backup files (created by sed on MacOS)
  for file in "${CHART_FILE}" "${CHART_VALUES_FILE}"; do
    rm -f "${file}-e"
  done
done

# MODULE_NAME must be set to the local go module name.
# NESTED_MODULES must be set to the list of nested go modules, e.g. 'api,nested2,nested3'
for nm in ${NESTED_MODULES//,/ }; do
  # update go.mod
  sed -i -e "s@	$MODULE_NAME/$nm .*@	$MODULE_NAME/$nm ${VERSION}@" "${GO_MOD_FILE}"
  # remove backup file (created by sed on MacOS)
  rm -f "${GO_MOD_FILE}-e"
done
