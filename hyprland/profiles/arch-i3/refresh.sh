#!/bin/bash
set -euo pipefail

envf=/home/hypruser/.rice/session.env

if [ ! -f "$envf" ]; then
    echo "session env not ready"
    exit 1
fi

. "$envf"
i3-msg reload >/dev/null
