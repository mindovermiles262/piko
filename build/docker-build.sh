#!/bin/bash

set -euo pipefail

if [ $# -ne 1 ]; then
  echo "No release tag name given. Failing"
  exit 1
fi

docker build \
  --build-arg version="$1" \
  --platform=linux/amd64,linux/arm64 \
  -f ./Dockerfile \
  -t "ghcr.io/andydunstall/piko:$1" \
  -t "ghcr.io/andydunstall/piko:latest" \
  .

