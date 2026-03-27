#!/bin/bash
set -euo pipefail

echo "================================================"
echo " Arch i3 Preview Container"
echo "================================================"

uid=1000
user=hypruser
disp=:1
runtime_dir=/run/user/$uid
rice_dir=/home/$user/.rice
cfg_dir=/home/$user/.config
src_dir=/workspace/profiles/arch-i3/runtime
mkdir -p "$rice_dir"

exec > >(tee -a "$rice_dir/boot.log") 2>&1

link_path() {
    local src=$1
    local dst=$2
    mkdir -p "$(dirname "$dst")"
    rm -rf "$dst"
    ln -s "$src" "$dst"
    chown -h "$user:$user" "$dst"
}

write_env() {
    cat > "$rice_dir/session.env" <<EOF
export DISPLAY=$disp
export XDG_RUNTIME_DIR=$runtime_dir
export XDG_SESSION_TYPE=x11
export XDG_CURRENT_DESKTOP=i3
export DBUS_SESSION_BUS_ADDRESS=unix:path=$runtime_dir/bus
EOF
    chown "$user:$user" "$rice_dir/session.env"
}

if [ -d "$src_dir" ]; then
    link_path "$src_dir/i3" "$cfg_dir/i3"
    link_path "$src_dir/Xresources" "/home/$user/.Xresources"
fi

dbus-uuidgen > /etc/machine-id 2>/dev/null || true
mkdir -p /run/dbus
dbus-daemon --system --fork 2>/dev/null || true
mkdir -p "$runtime_dir"
chown "$user:$user" "$runtime_dir"
chmod 700 "$runtime_dir"
chown -R "$user:$user" "$rice_dir" "/home/$user/.config"

echo "[1/4] Starting Xvfb on $disp..."
Xvfb "$disp" -screen 0 1920x1080x24 -nolisten tcp &
XVFB_PID=$!
sleep 2
if ! kill -0 "$XVFB_PID" 2>/dev/null; then
    echo "ERROR: Xvfb failed to start"
    sleep 3600
    exit 1
fi

write_env

echo "[2/4] Starting i3..."
su - "$user" -c "
    export DISPLAY=$disp
    export XDG_RUNTIME_DIR=$runtime_dir
    export DBUS_SESSION_BUS_ADDRESS=unix:path=$runtime_dir/bus
    dbus-daemon --session --address=unix:path=$runtime_dir/bus --fork
    command -v xrdb >/dev/null 2>&1 && [ -f ~/.Xresources ] && xrdb -merge ~/.Xresources || true
    xsetroot -solid '#181825'
    exec i3
" &
I3_PID=$!
sleep 2
if ! kill -0 "$I3_PID" 2>/dev/null; then
    echo "ERROR: i3 failed to start"
    sleep 3600
    exit 1
fi

echo "[3/4] Starting x11vnc on :5070..."
su - "$user" -c "
    export DISPLAY=$disp
    exec x11vnc -display $disp -rfbport 5070 -listen 0.0.0.0 -forever -shared -nopw -xkb -noxdamage
" &
VNC_PID=$!
sleep 2
if ! kill -0 "$VNC_PID" 2>/dev/null; then
    echo "ERROR: x11vnc failed to start"
    sleep 3600
    exit 1
fi

echo "[4/4] Starting noVNC on :6090..."
websockify --web=/opt/novnc 6090 localhost:5070 &

echo ""
echo "================================================"
echo " Ready!"
echo " Display : $disp"
echo " VNC     : localhost:5070"
echo " Browser : http://localhost:6090"
echo "================================================"

wait $I3_PID || true
echo "i3 exited, keeping container alive for inspection..."
sleep 3600
