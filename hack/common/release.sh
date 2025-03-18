#!/bin/bash

set -euo pipefail
source "$(realpath "$(dirname $0)/environment.sh")"

VERSION=$("$COMMON_SCRIPT_DIR/get-version.sh")

(
  cd "$PROJECT_ROOT"

  echo "> Checking for uncommitted changes"
  if [[ -n "$(git status --porcelain=v1)" ]]; then
    echo "There are uncommitted changes in the working directory."
    git status --short
    echo
    echo "These changes will be included in the release commit, unless you stash, commit, or remove them otherwise before."
    echo "Do you want to continue, including these changes in the release commit? Please confirm with 'yes' or 'y':"
    read confirm
    if [[ "$confirm" != "yes" ]] && [[ "$confirm" != "y" ]]; then
      echo "Release aborted."
      exit 0
    fi
  else
    echo "No uncommitted changes found."
  fi
  echo

  echo "> Finding latest release"
  major=${VERSION%%.*}
  major=${major#v}
  minor=${VERSION#*.}
  minor=${minor%%.*}
  patch=${VERSION##*.}
  patch=${patch%%-*}
  echo "v${major}.${minor}.${patch}"
  echo

  semver=${1:-"minor"}

  case "$semver" in
    ("major")
      major=$((major + 1))
      minor=0
      patch=0
      ;;
    ("minor")
      minor=$((minor + 1))
      patch=0
      ;;
    ("patch")
      patch=$((patch + 1))
      ;;
    (*)
      echo "invalid argument: $semver"
      exit 1
      ;;
  esac

  release_version="v$major.$minor.$patch"

  echo "The release version will be $release_version. Please confirm with 'yes' or 'y':"
  read confirm

  if [[ "$confirm" != "yes" ]] && [[ "$confirm" != "y" ]]; then
    echo "Release not confirmed."
    exit 0
  fi
  echo

  echo "> Updating version to release version"
  "$COMMON_SCRIPT_DIR/set-version.sh" $release_version
  echo

  git add --all
  git commit -m "release $release_version"
  echo

  echo "> Successfully finished"
)