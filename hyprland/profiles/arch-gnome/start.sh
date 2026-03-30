#!/bin/bash
set -euo pipefail

PROFILE=arch-gnome
TITLE="Arch GNOME Preview Container"
DESKTOP_NAME=GNOME
DESKTOP_ID=GNOME
SESSION_COMMAND="gnome-session --session=gnome"
ROOT_COLOR="#111827"
SESSION_BOOT_WAIT=10
WELCOME_DELAY=8

. /workspace/profiles/arch-x11-common/start.sh
