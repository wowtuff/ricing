#!/bin/bash
set -euo pipefail

echo "================================================"
echo " Hyprland Preview Container"
echo "================================================"

uid=1000
user=hypruser
runtime_dir=/run/user/$uid
rice_dir=/home/$user/.rice
cfg_dir=/home/$user/.config
src_dir=/workspace
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
export XDG_RUNTIME_DIR=$runtime_dir
export XDG_SESSION_TYPE=wayland
export XDG_CURRENT_DESKTOP=Hyprland
export AQ_BACKENDS=headless
export WLR_RENDERER=pixman
export EGL_PLATFORM=surfaceless
export LIBSEAT_BACKEND=seatd
export HYPRLAND_NO_SD_NOTIFY=1
export DBUS_SESSION_BUS_ADDRESS=unix:path=$runtime_dir/bus
export HYPRLAND_INSTANCE_SIGNATURE=$SOCKET
export WAYLAND_DISPLAY=$WAYLAND_DISPLAY_SOCKET
export HEADLESS_OUTPUT=$HEADLESS_OUTPUT
EOF
    chown "$user:$user" "$rice_dir/session.env"
}

if [ -d "$src_dir" ]; then
    link_path "$src_dir/hyprland.conf" "$cfg_dir/hypr/hyprland.conf"
    link_path "$src_dir/generated.conf" "$cfg_dir/hypr/generated.conf"
    link_path "$src_dir/kitty" "$cfg_dir/kitty"
    link_path "$src_dir/waybar" "$cfg_dir/waybar"
    link_path "$src_dir/rofi" "$cfg_dir/rofi"
    link_path "$src_dir/dunst" "$cfg_dir/dunst"
    link_path "$src_dir/swww" "$cfg_dir/swww"
fi

dbus-uuidgen > /etc/machine-id 2>/dev/null || true

mkdir -p /run/dbus
dbus-daemon --system --fork 2>/dev/null || true

mkdir -p "$runtime_dir"
chown "$user:$user" "$runtime_dir"
chmod 700 "$runtime_dir"

mkdir -p /home/$user/.cache/hyprland
chown -R "$user:$user" /home/$user/.cache "$rice_dir"

rm -rf "$runtime_dir/hypr"
rm -f "$runtime_dir"/wayland-*

echo "[1/4] Starting seatd..."
SEATD_VTBOUND=0 seatd &
SEATD_PID=$!
sleep 1
chmod 666 /run/seatd.sock || true
echo "      seatd running (pid $SEATD_PID)"

echo "[2/4] Starting Hyprland..."
su - "$user" -c "
    export XDG_RUNTIME_DIR=$runtime_dir
    export XDG_SESSION_TYPE=wayland
    export XDG_CURRENT_DESKTOP=Hyprland
    export AQ_BACKENDS=headless
    export WLR_RENDERER=pixman
    export EGL_PLATFORM=surfaceless
    export LIBSEAT_BACKEND=seatd
    export HYPRLAND_NO_SD_NOTIFY=1
    export DBUS_SESSION_BUS_ADDRESS=unix:path=$runtime_dir/bus
    dbus-daemon --session --address=unix:path=$runtime_dir/bus --fork
    exec start-hyprland
" &
HYPRLAND_PID=$!

echo "      Waiting for Hyprland instance..."
SOCKET=""
for i in $(seq 1 30); do
    SOCKET=$(ls "$runtime_dir/hypr" 2>/dev/null | head -1 || true)
    if [ -n "$SOCKET" ]; then
        echo "      Hyprland instance: $SOCKET"
        break
    fi
    sleep 1
done

if [ -z "$SOCKET" ]; then
    echo "ERROR: Hyprland failed to start"
    sleep 3600
    exit 1
fi

export HYPRLAND_INSTANCE_SIGNATURE="$SOCKET"
su - "$user" -c "
    export XDG_RUNTIME_DIR=$runtime_dir
    export HYPRLAND_INSTANCE_SIGNATURE=$SOCKET
    hyprctl -j monitors all | jq -r '.[].name' | while read -r mon; do
        hyprctl keyword monitor "\$mon,disabled"
    done
" || true


WAYLAND_DISPLAY_SOCKET=""
for s in "$runtime_dir"/wayland-*; do
    if [ -S "$s" ]; then
        WAYLAND_DISPLAY_SOCKET="$(basename "$s")"
        break
    fi
done

if [ -z "$WAYLAND_DISPLAY_SOCKET" ]; then
    echo "ERROR: No Wayland socket found"
    sleep 3600
    exit 1
fi

echo "      Wayland display: $WAYLAND_DISPLAY_SOCKET"



echo "[3/4] Creating virtual display..."
su - "$user" -c "
    export XDG_RUNTIME_DIR=$runtime_dir
    export HYPRLAND_INSTANCE_SIGNATURE=$SOCKET
    hyprctl output create headless 1920x1080
" || true

sleep 2

echo "      Detecting actual headless output..."
HEADLESS_OUTPUT="$(
    su - "$user" -c "
        export XDG_RUNTIME_DIR=$runtime_dir
        export HYPRLAND_INSTANCE_SIGNATURE=$SOCKET
        hyprctl -j monitors all
    " | jq -r '.[] | select(.make == "" and .model == "") | .name' | tail -n1
)"

if [ -z "${HEADLESS_OUTPUT:-}" ]; then
    echo "ERROR: No HEADLESS output detected"
    su - "$user" -c "
        export XDG_RUNTIME_DIR=$runtime_dir
        export HYPRLAND_INSTANCE_SIGNATURE=$SOCKET
        hyprctl monitors all || true
    "
    sleep 3600
    exit 1
fi

echo "      Using output: $HEADLESS_OUTPUT"
write_env

echo "[4/4] Starting wayvnc on :5070..."
su - "$user" -c "
    export XDG_RUNTIME_DIR=$runtime_dir
    export WAYLAND_DISPLAY=$WAYLAND_DISPLAY_SOCKET
    exec wayvnc -o $HEADLESS_OUTPUT 0.0.0.0 5070
" &
WAYVNC_PID=$!

sleep 2

if ! kill -0 "$WAYVNC_PID" 2>/dev/null; then
    echo "ERROR: wayvnc failed to start"
    sleep 3600
    exit 1
fi

echo "      Starting noVNC on :6090..."
websockify --web=/opt/novnc 6090 localhost:5070 &

echo ""
echo "================================================"
echo " Ready!"
echo " Output : $HEADLESS_OUTPUT"
echo " VNC    : localhost:5070"
echo " Browser: http://localhost:6090"
echo "================================================"

wait $HYPRLAND_PID || true
echo "Hyprland crashed, keeping container alive for inspection..."
sleep 3600
