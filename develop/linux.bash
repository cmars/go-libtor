#!/usr/bin/env bash
set -eux
cd $(dirname $0)
podman build -t go-libtor-devel-linux:latest -f Dockerfile.linux .
podman run --rm -it -v $(pwd)/..:/go/src/go-libtor go-libtor-devel-linux:latest

