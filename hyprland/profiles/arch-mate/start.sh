#!/bin/bash
set -euo pipefail

PROFILE=arch-mate
TITLE="Arch MATE Preview Container"
DESKTOP_NAME=MATE
DESKTOP_ID=MATE
SESSION_COMMAND="mate-session"
ROOT_COLOR="#203a2a"
SESSION_BOOT_WAIT=8
WELCOME_DELAY=6

. /workspace/profiles/arch-x11-common/start.sh
