#!/bin/bash

echo "preview container"

dbus-uuidgen > /etc/machine-id 2>/dev/null || true
mkdir -p /run/user/1000
chown hypruser:hypruser /run/user/1000

#seatd
echo "seatd start"
SEATD_VTBOUND=0 seatd &
SEATD_PID=$!
sleep 1
chmod 777 /run/seatd.sock
echo "seatd (pid $SEATD_PID) running"

#hyprland headless
echo "hyprland headless start"
su - hypruser -c "
    export XDG_RUNTIME_DIR=/run/user/1000
    export XDG_SESSION_TYPE=wayland
    export WLR_BACKENDS=headless
    export WLR_RENDERER=pixman
    Hyprland
" &
HYPRLAND_PID=$!
echo "hyprland socket wait"
for i in $(seq 1 30); do
    SOCKET=$(ls /run/user/1000/hypr/ 2>/dev/null | head -1)
    if [ -n "$SOCKET" ]; then
        echo "hyprland socket ready: $SOCKET"
        break
    fi
    sleep 1
done

if [ -z "$SOCKET" ]; then
    echo "ERROR!!!!!!!!!!!! hyprland fail"
    exit 1
fi
export HYPRLAND_INSTANCE_SIGNATURE=$SOCKET

# virtual headless output for novnc
echo "virtual headless output"
su - hypruser -c "
    export XDG_RUNTIME_DIR=/run/user/1000
    export WAYLAND_DISPLAY=wayland-1
    export HYPRLAND_INSTANCE_SIGNATURE=$SOCKET
    hyprctl output create headless
" && echo "Virtual output created" || echo "Warning!!! could not create output (may already exist)"
sleep 1

# Start wayvnc
echo "[4/4] Starting wayvnc on :5070..."
su - hypruser -c "
    export XDG_RUNTIME_DIR=/run/user/1000
    export WAYLAND_DISPLAY=wayland-1
    wayvnc 0.0.0.0 5070
" &
WAYVNC_PID=$!
sleep 1

# Start websockify and novnc
echo "Starting noVNC on :6090..."
websockify --web=/opt/novnc 6090 localhost:5070 &
echo " VNC client : localhost:5070"
echo " Browser    : http://localhost:6090"
wait $HYPRLAND_PID
