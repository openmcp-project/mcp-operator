#!/bin/bash

set -euo pipefail
source "$(realpath "$(dirname $0)/../environment.sh")"

JQ_VERSION="$1"
os="linux64"
if [[ $(uname -o) == "Darwin" ]]; then
  os="osx-amd64"
fi
curl -sfL "https://github.com/stedolan/jq/releases/download/jq-${JQ_VERSION}/jq-${os}" --output "${LOCALBIN}/jq"
chmod +x "${LOCALBIN}/jq"