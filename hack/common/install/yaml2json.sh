#!/bin/bash

set -euo pipefail
source "$(realpath "$(dirname $0)/../environment.sh")"

YAML2JSON_VERSION="$1"

arch=$(uname -m)
if [[ "$arch" == "x86_64" ]]; then
  arch="amd64"
fi
os=$(uname | tr '[:upper:]' '[:lower:]')
curl -sfL "https://github.com/bronze1man/yaml2json/releases/download/${YAML2JSON_VERSION}/yaml2json_${os}_${arch}" --output "${LOCALBIN}/yaml2json"
chmod +x "${LOCALBIN}/yaml2json"