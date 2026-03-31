#!/usr/bin/env bash
set -euo pipefail

if command -v cava >/dev/null 2>&1; then
    exec cava
fi

printf '%s\n' \
  'Visualizer launcher is ready but cava is not installed yet.' \
  'Install cava, then re-run the KDE reference launcher.'
read -r -p 'Press Enter to close… ' _
