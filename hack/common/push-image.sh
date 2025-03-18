#!/bin/bash

set -euo pipefail
source "$(realpath "$(dirname $0)/environment.sh")"

if [[ -z ${IMAGE_REGISTRY:-} ]]; then
	IMAGE_REGISTRY=$("$COMMON_SCRIPT_DIR/get-registry.sh" -i)
fi

VERSION=$("$COMMON_SCRIPT_DIR/get-version.sh")

(
  cd "$PROJECT_ROOT"

  echo
  echo "> Pushing images to registry $IMAGE_REGISTRY ..."
  for comp in ${COMPONENTS//,/ }; do
    echo "Pushing image for component '$comp' ..." | indent 1
    img="${IMAGE_REGISTRY}/${comp}:${VERSION}"
    for pf in ${PLATFORMS//,/ }; do
      os=${pf%/*}
      arch=${pf#*/}
      pfimg="${img}-${os}-${arch}"

      echo "> Pushing platform-specific image for $pf ..." | indent 2
      docker push "$pfimg" | indent 3

      echo "> Adding image to multi-platform manifest ..." | indent 2
      docker manifest create $img --amend $pfimg | indent 3
    done

    echo "> Pushing multi-platform manifest ..."
    docker manifest push "$img" | indent 2

    if [[ ${ADDITIONAL_TAG:-} ]]; then
      echo "> Adding additional tag '$ADDITIONAL_TAG' ..."
      docker buildx imagetools create "$img" --tag "${img%:*}:$ADDITIONAL_TAG" | indent 2
    fi
  done
)