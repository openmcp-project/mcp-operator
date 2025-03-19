#!/bin/bash

set -euo pipefail
source "$(realpath "$(dirname $0)/../environment.sh")"

HELM_VERSION="$1"

arch=$(uname -m)
if [[ "$arch" == "x86_64" ]]; then
  arch="amd64"
fi
os=$(uname | tr '[:upper:]' '[:lower:]')
curl -sfL "https://get.helm.sh/helm-${HELM_VERSION}-${os}-${arch}.tar.gz" --output "$LOCALBIN/helm.tar.gz"
mkdir -p "$LOCALBIN/helm-unpacked"
tar -xzf "$LOCALBIN/helm.tar.gz" --directory "$LOCALBIN/helm-unpacked"
mv "$LOCALBIN/helm-unpacked/${os}-${arch}/helm" "$LOCALBIN/helm"
chmod +x "$LOCALBIN/helm"
rm -rf "$LOCALBIN/helm.tar.gz" "$LOCALBIN/helm-unpacked"
