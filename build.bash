#!/usr/bin/env bash
exec go run build/mage.go -d build "$@"
