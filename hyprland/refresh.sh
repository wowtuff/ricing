#!/bin/bash
set -euo pipefail

envf=/home/hypruser/.rice/session.env
rice_dir=/home/hypruser/.rice

if [ ! -f "$envf" ]; then
    echo "session env not ready"
    exit 1
fi

. "$envf"

restart_if_up() {
    local proc=$1
    shift
    if ! pgrep -u "$(id -u)" -x "$proc" >/dev/null 2>&1; then
        return
    fi
    pkill -u "$(id -u)" -x "$proc" || true
    "$@" >> "$rice_dir/$proc.log" 2>&1 &
}

hyprctl reload
restart_if_up waybar waybar
restart_if_up dunst dunst
