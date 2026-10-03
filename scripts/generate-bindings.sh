#!/usr/bin/env bash
# Resolve the CLI from the repository's Wails module pin. Failure is fatal.
set -euo pipefail
cd "$(dirname "$0")/../apps/desktop"
wails_version="$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)"
[[ "$wails_version" == v* ]] || { echo "Missing pinned Wails module version" >&2; exit 1; }
go run "github.com/wailsapp/wails/v3/cmd/wails3@${wails_version}" generate bindings
