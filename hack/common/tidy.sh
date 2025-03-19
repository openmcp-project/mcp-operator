#!/bin/bash

set -euo pipefail
source "$(realpath "$(dirname $0)/environment.sh")"

function tidy() {
  go mod tidy -e
}

# NESTED_MODULES must be set to the list of nested go modules, e.g. 'api,nested2,nested3'
for nm in ${NESTED_MODULES//,/ }; do
  echo "Tidy $nm module ..."
  (
    cd "$PROJECT_ROOT/$nm"
    tidy
  )
done

echo "Tidy root module ..."
(
  cd "$PROJECT_ROOT"
  tidy
)
