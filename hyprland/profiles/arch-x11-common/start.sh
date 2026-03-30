#!/bin/bash
set -euo pipefail

: "${PROFILE:?}"
: "${TITLE:?}"
: "${DESKTOP_NAME:?}"
: "${DESKTOP_ID:?}"
: "${SESSION_COMMAND:?}"

uid=1000
user=hypruser
home_dir=/home/$user
disp=:1
runtime_dir=/run/user/$uid
rice_dir=$home_dir/.rice
cfg_dir=$home_dir/.config
src_dir=/workspace/profiles/$PROFILE/runtime
common_runtime_dir=/workspace/profiles/arch-x11-common/runtime
mkdir -p "$rice_dir"

exec > >(tee -a "$rice_dir/boot.log") 2>&1

echo "================================================"
echo " $TITLE"
echo "================================================"

link_path() {
    local src=$1
    local dst=$2
    mkdir -p "$(dirname "$dst")"
    rm -rf "$dst"
    ln -s "$src" "$dst"
    chown -h "$user:$user" "$dst"
}

sync_runtime_dir() {
    local dir=$1
    [ -d "$dir" ] || return
    for item in "$dir"/*; do
        [ -e "$item" ] || continue
        local name
        name=$(basename "$item")
        case "$name" in
            Xresources)
                link_path "$item" "$home_dir/.Xresources"
                ;;
            *)
                link_path "$item" "$cfg_dir/$name"
                ;;
        esac
    done
}

write_env() {
    cat > "$rice_dir/session.env" <<EOF
export DISPLAY=$disp
export XDG_RUNTIME_DIR=$runtime_dir
export XDG_SESSION_TYPE=x11
export XDG_CURRENT_DESKTOP=$DESKTOP_ID
export DESKTOP_SESSION=$DESKTOP_ID
export XDG_SESSION_DESKTOP=$DESKTOP_ID
export DBUS_SESSION_BUS_ADDRESS=unix:path=$runtime_dir/bus
export LIBGL_ALWAYS_SOFTWARE=1
export QT_X11_NO_MITSHM=1
export NO_AT_BRIDGE=1
export PREVIEW_PROFILE=$PROFILE
export PREVIEW_URL=http://127.0.0.1:6090/?autoconnect=1&resize=remote
EOF
    chown "$user:$user" "$rice_dir/session.env"
}

dbus-uuidgen > /etc/machine-id 2>/dev/null || true
mkdir -p /run/dbus "$runtime_dir" "$home_dir/.cache"
dbus-daemon --system --fork 2>/dev/null || true
chown "$user:$user" "$runtime_dir"
chmod 700 "$runtime_dir"

sync_runtime_dir "$common_runtime_dir"
sync_runtime_dir "$src_dir"

chown -R "$user:$user" "$rice_dir" "$cfg_dir" "$home_dir/.cache"
write_env

echo "[1/4] Starting Xvfb on $disp..."
Xvfb "$disp" -screen 0 1920x1080x24 -nolisten tcp &
XVFB_PID=$!
sleep 2
if ! kill -0 "$XVFB_PID" 2>/dev/null; then
    echo "ERROR: Xvfb failed to start"
    sleep 3600
    exit 1
fi

echo "[2/4] Starting $DESKTOP_NAME..."
su - "$user" -c "
    export HOME=$home_dir
    export DISPLAY=$disp
    export XDG_RUNTIME_DIR=$runtime_dir
    export XDG_SESSION_TYPE=x11
    export XDG_CURRENT_DESKTOP=$DESKTOP_ID
    export DESKTOP_SESSION=$DESKTOP_ID
    export XDG_SESSION_DESKTOP=$DESKTOP_ID
    export DBUS_SESSION_BUS_ADDRESS=unix:path=$runtime_dir/bus
    export LIBGL_ALWAYS_SOFTWARE=1
    export QT_X11_NO_MITSHM=1
    export NO_AT_BRIDGE=1
    dbus-daemon --session --address=unix:path=$runtime_dir/bus --fork
    command -v xrdb >/dev/null 2>&1 && [ -f ~/.Xresources ] && xrdb -merge ~/.Xresources || true
    xsetroot -solid '${ROOT_COLOR:-#111827}'
    exec bash -lc '$SESSION_COMMAND'
" &
SESSION_PID=$!
sleep "${SESSION_BOOT_WAIT:-8}"
if ! kill -0 "$SESSION_PID" 2>/dev/null; then
    echo "ERROR: $DESKTOP_NAME failed to start"
    sleep 3600
    exit 1
fi

(
    sleep "${WELCOME_DELAY:-6}"
    su - "$user" -c "
        export HOME=$home_dir
        export DISPLAY=$disp
        export XDG_RUNTIME_DIR=$runtime_dir
        export XDG_SESSION_TYPE=x11
        export XDG_CURRENT_DESKTOP=$DESKTOP_ID
        export DESKTOP_SESSION=$DESKTOP_ID
        export XDG_SESSION_DESKTOP=$DESKTOP_ID
        export DBUS_SESSION_BUS_ADDRESS=unix:path=$runtime_dir/bus
        export LIBGL_ALWAYS_SOFTWARE=1
        export QT_X11_NO_MITSHM=1
        export NO_AT_BRIDGE=1
        nohup xterm -fa 'JetBrains Mono' -fs 11 -geometry 120x34+60+60 -e bash -lc 'fastfetch || true; exec bash' >/home/$user/.rice/welcome.log 2>&1 &
    " >/dev/null 2>&1 || true
) &

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
echo " Desktop : $DESKTOP_NAME"
echo " Display : $disp"
echo " VNC     : localhost:5070"
echo " Browser : http://localhost:6090"
echo "================================================"

wait $SESSION_PID || true
echo "$DESKTOP_NAME exited, keeping container alive for inspection..."
sleep 3600
