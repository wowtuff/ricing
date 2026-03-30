#!/bin/bash
set -euo pipefail

envf=/home/hypruser/.rice/session.env

if [ ! -f "$envf" ]; then
    echo "session env not ready"
    exit 1
fi

. "$envf"

if [ -n "${PREVIEW_REFRESH_COMMAND:-}" ]; then
    bash -lc "$PREVIEW_REFRESH_COMMAND"
    exit 0
fi

echo "restart required"
