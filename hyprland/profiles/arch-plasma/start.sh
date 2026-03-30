#!/bin/bash
set -euo pipefail

PROFILE=arch-plasma
TITLE="Arch Plasma Preview Container"
DESKTOP_NAME=Plasma
DESKTOP_ID=KDE
SESSION_COMMAND="startplasma-x11"
ROOT_COLOR="#0f172a"
SESSION_BOOT_WAIT=10
WELCOME_DELAY=8

. /workspace/profiles/arch-x11-common/start.sh
