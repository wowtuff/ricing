#!/bin/bash
set -euo pipefail

PROFILE=arch-xfce
TITLE="Arch XFCE Preview Container"
DESKTOP_NAME=XFCE
DESKTOP_ID=XFCE
SESSION_COMMAND="startxfce4"
ROOT_COLOR="#172554"
SESSION_BOOT_WAIT=8
WELCOME_DELAY=6

. /workspace/profiles/arch-x11-common/start.sh
