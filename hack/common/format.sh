#!/bin/bash

set -euo pipefail
source "$(realpath "$(dirname $0)/environment.sh")"

write_mode="-w"
if [[ ${1:-} == "--verify" ]]; then
  write_mode=""
  shift
fi

# MODULE_NAME must be set to the name of the local go module.
tmp=$("${FORMATTER}" -l $write_mode -local=$MODULE_NAME $("$COMMON_SCRIPT_DIR/unfold.sh" --clean --no-unfold "$@"))

if [[ -z ${write_mode} ]] && [[ ${tmp} ]]; then
  echo "unformatted files detected, please run 'make format'" 1>&2
  echo "$tmp" 1>&2
  exit 1
fi

if [[ ${tmp} ]]; then
  echo "> Formatting imports ..."
  echo "$tmp"
fi
