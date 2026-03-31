#!/usr/bin/env bash
set -euo pipefail

launch_applet() {
    local applet=$1
    if pgrep -af "plasmawindowed.*$applet" >/dev/null 2>&1; then
        return 0
    fi
    nohup plasmawindowed "$applet" >/dev/null 2>&1 &
}

launch_applet org.kde.plasma.binaryclock
launch_applet org.kde.plasma.mediacontroller

if command -v kitty >/dev/null 2>&1; then
    if ! pgrep -af 'kitty.*All Media Visualizer' >/dev/null 2>&1; then
        nohup kitty \
            --class MonochromeVisualizer \
            --title "All Media Visualizer" \
            --override font_family='JetBrains Mono' \
            --override font_size=12.0 \
            --override background_opacity=0.82 \
            --override background=#000000 \
            --override foreground=#f5f5f5 \
            --override cursor=#ffffff \
            --override selection_background=#3a3a3a \
            --override window_padding_width=18 \
            --override hide_window_decorations=yes \
            --override remember_window_size=no \
            --override initial_window_width=72c \
            --override initial_window_height=16c \
            /home/life2harsh/ricing/kde-reference/run-all-media-visualizer.sh >/dev/null 2>&1 &
    fi
fi
