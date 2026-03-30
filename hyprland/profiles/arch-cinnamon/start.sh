#!/bin/bash
set -euo pipefail

PROFILE=arch-cinnamon
TITLE="Arch Cinnamon Preview Container"
DESKTOP_NAME=Cinnamon
DESKTOP_ID=X-Cinnamon
SESSION_COMMAND="cinnamon-session"
ROOT_COLOR="#3f1d2e"
SESSION_BOOT_WAIT=8
WELCOME_DELAY=6

. /workspace/profiles/arch-x11-common/start.sh
