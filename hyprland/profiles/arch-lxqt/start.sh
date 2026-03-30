#!/bin/bash
set -euo pipefail

PROFILE=arch-lxqt
TITLE="Arch LXQt Preview Container"
DESKTOP_NAME=LXQt
DESKTOP_ID=LXQt
SESSION_COMMAND="startlxqt"
ROOT_COLOR="#1e293b"
SESSION_BOOT_WAIT=8
WELCOME_DELAY=6

. /workspace/profiles/arch-x11-common/start.sh
