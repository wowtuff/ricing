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
mkdir -p "$rice_dir" "$cfg_dir"

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
    [ -d "$dir" ] || return 0
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
    if [ -n "${PREVIEW_REFRESH_COMMAND:-}" ]; then
        printf 'export PREVIEW_REFRESH_COMMAND=%q\n' "$PREVIEW_REFRESH_COMMAND" >> "$rice_dir/session.env"
    fi
    chown "$user:$user" "$rice_dir/session.env"
}

dbus-uuidgen > /etc/machine-id 2>/dev/null || true
mkdir -p /run/dbus "$runtime_dir" "$home_dir/.cache"
dbus-daemon --system --fork 2>/dev/null || true
chown "$user:$user" "$runtime_dir"
chmod 700 "$runtime_dir"

if [ "$PROFILE" = "arch-plasma" ]; then
    mkdir -p "$cfg_dir/conky" "$cfg_dir/plasma-rice/wallpapers"
    export PREVIEW_REFRESH_COMMAND='bash -lc "$HOME/.config/plasma-rice/launch.sh"'

    cat > "$cfg_dir/plasma-rice/launch.sh" <<'EOF'
#!/bin/bash
set -euo pipefail

wallpaper="$HOME/.config/plasma-rice/wallpapers/blue-lake.svg"
clock_conf="$HOME/.config/conky/top-clock.conf"
stats_conf="$HOME/.config/conky/right-stats.conf"
rail_conf="$HOME/.config/conky/left-rail.conf"
dock_conf="$HOME/.config/conky/bottom-dock.conf"

pkill -f 'conky.*top-clock.conf' 2>/dev/null || true
pkill -f 'conky.*right-stats.conf' 2>/dev/null || true
pkill -f 'conky.*left-rail.conf' 2>/dev/null || true
pkill -f 'conky.*bottom-dock.conf' 2>/dev/null || true
pkill -f 'kitty.*RiceFetch' 2>/dev/null || true
pkill -f 'kitty.*RiceTop' 2>/dev/null || true

lookandfeeltool -a org.kde.breezedark.desktop >/dev/null 2>&1 || true
plasma-apply-colorscheme BreezeDark >/dev/null 2>&1 || true
plasma-apply-wallpaperimage "$wallpaper" >/dev/null 2>&1 || true

if command -v kwriteconfig5 >/dev/null 2>&1; then
  kwriteconfig5 --file "$HOME/.config/kdeglobals" --group General --key ColorScheme BreezeDark >/dev/null 2>&1 || true
fi

nohup conky -c "$clock_conf" >/tmp/kde-rice-clock.log 2>&1 &
nohup conky -c "$stats_conf" >/tmp/kde-rice-stats.log 2>&1 &
nohup conky -c "$rail_conf" >/tmp/kde-rice-rail.log 2>&1 &
nohup conky -c "$dock_conf" >/tmp/kde-rice-dock.log 2>&1 &

sleep 2

nohup kitty --class RiceFetch --title 'Rice Fetch' \
  --config "$HOME/.config/kitty/kitty.conf" \
  --override font_size=10.0 \
  --override background=#1a3557 \
  --override background_opacity=0.58 \
  --override initial_window_width=90c \
  --override initial_window_height=28c \
  --override hide_window_decorations=yes \
  bash -lc 'clear; fastfetch || true; exec bash' >/tmp/kde-rice-fetch.log 2>&1 &

nohup kitty --class RiceTop --title 'Rice Htop' \
  --config "$HOME/.config/kitty/kitty.conf" \
  --override font_size=10.0 \
  --override background=#1a3557 \
  --override background_opacity=0.54 \
  --override initial_window_width=92c \
  --override initial_window_height=28c \
  --override hide_window_decorations=yes \
  bash -lc 'clear; htop || exec bash' >/tmp/kde-rice-top.log 2>&1 &

place_window() {
  local class=$1 x=$2 y=$3 w=$4 h=$5 id
  id=$(xdotool search --sync --class "$class" | tail -n 1)
  [ -n "$id" ] || return 0
  wmctrl -i -r "$id" -b add,above,sticky,skip_taskbar >/dev/null 2>&1 || true
  xdotool windowmove "$id" "$x" "$y" >/dev/null 2>&1 || true
  xdotool windowsize "$id" "$w" "$h" >/dev/null 2>&1 || true
}

sleep 3
place_window RiceFetch 170 150 660 420
place_window RiceTop 860 150 660 420
EOF
    chmod +x "$cfg_dir/plasma-rice/launch.sh"

    cat > "$cfg_dir/conky/top-clock.conf" <<'EOF'
conky.config = {
  alignment = 'top_middle',
  background = true,
  double_buffer = true,
  update_interval = 1,
  own_window = true,
  own_window_type = 'dock',
  own_window_argb_visual = true,
  own_window_argb_value = 0,
  own_window_hints = 'undecorated,below,sticky,skip_taskbar,skip_pager',
  minimum_width = 760,
  maximum_width = 760,
  gap_x = 0,
  gap_y = 10,
  draw_shades = false,
  draw_outline = false,
  draw_borders = false,
  use_xft = true,
  xftalpha = 1,
  default_color = '#e8edf6',
  uppercase = false,
  use_spacer = 'none',
  override_utf8_locale = true,
  font = 'JetBrains Mono:size=10'
}

conky.text = [[
${alignc}${font JetBrains Mono:size=42:bold}${color #eef4fc}${time %H:%M}${font}
${alignc}${voffset -12}${font JetBrains Mono:size=8}${color #cfdcf0}${time %A, %d %B %Y}${font}
]]
EOF

    cat > "$cfg_dir/conky/right-stats.conf" <<'EOF'
conky.config = {
  alignment = 'top_right',
  background = true,
  double_buffer = true,
  update_interval = 1,
  own_window = true,
  own_window_type = 'dock',
  own_window_argb_visual = true,
  own_window_argb_value = 0,
  own_window_hints = 'undecorated,below,sticky,skip_taskbar,skip_pager',
  minimum_width = 300,
  maximum_width = 300,
  gap_x = 28,
  gap_y = 168,
  draw_shades = false,
  draw_outline = false,
  draw_borders = false,
  use_xft = true,
  xftalpha = 1,
  default_color = '#ebf2fb',
  uppercase = false,
  use_spacer = 'none',
  override_utf8_locale = true,
  font = 'JetBrains Mono:size=10'
}

conky.text = [[
${alignc}${font JetBrains Mono:size=12:bold}${color #dce8f9}${time %A}${font}${alignr}${font JetBrains Mono:size=8}${color #b6c7de}Uptime: ${uptime_short}${font}
${alignc}${voffset -6}${font JetBrains Mono:size=34:bold}${color #f6f9fe}${time %d}${font}
${alignc}${voffset -10}${font JetBrains Mono:size=16:bold}${color #dce8f9}${time %b.}${font}
${alignc}${voffset -2}${font JetBrains Mono:size=26:bold}${color #f6f9fe}${time %H:%M}${font}
${voffset 12}${color #bcd1ec}Core 0:${alignr}${cpu cpu0}%
${color #7fc2ff}${cpubar cpu0 8,300}
${color #bcd1ec}CPU 1:${alignr}${cpu cpu1}%
${color #f0d56d}${cpubar cpu1 8,300}
${color #bcd1ec}Mem:${alignr}$mem / $memmax
${color #ffffff}${membar 8,300}
${color #bcd1ec}Swap:${alignr}$swap / $swapmax
${color #ff8b8b}${swapbar 8,300}
${color #bcd1ec}${hr 2}
${color #dce8f9}Manjaro Linux${alignr}${color #b6c7de}${kernel}
${color #dce8f9}CPU:${alignr}${color #b6c7de}${execi 30 bash -lc "lscpu | awk -F: '/Model name/ {gsub(/^ +/,\"\",$2); print $2; exit}' | cut -c1-24"}
${color #dce8f9}Memory:${alignr}${color #b6c7de}$memmax
]]
EOF

    cat > "$cfg_dir/conky/left-rail.conf" <<'EOF'
conky.config = {
  alignment = 'top_left',
  background = true,
  double_buffer = true,
  update_interval = 2,
  own_window = true,
  own_window_type = 'dock',
  own_window_argb_visual = true,
  own_window_argb_value = 0,
  own_window_hints = 'undecorated,above,sticky,skip_taskbar,skip_pager',
  minimum_width = 56,
  maximum_width = 56,
  gap_x = 10,
  gap_y = 210,
  draw_shades = false,
  draw_outline = false,
  draw_borders = false,
  use_xft = true,
  xftalpha = 1,
  default_color = '#dbe7f7',
  uppercase = false,
  use_spacer = 'none',
  override_utf8_locale = true,
  font = 'JetBrains Mono:size=12:bold'
}

conky.text = [[
${alignc}${color #edf4ff}⣿
${voffset 20}${alignc}${color #89a9cf}◔
${voffset 20}${alignc}${color #edf4ff}↹
${voffset 20}${alignc}${color #edf4ff}⌘
${voffset 20}${alignc}${color #edf4ff}⌂
${voffset 20}${alignc}${color #edf4ff}⋮
]]
EOF

    cat > "$cfg_dir/conky/bottom-dock.conf" <<'EOF'
conky.config = {
  alignment = 'bottom_middle',
  background = true,
  double_buffer = true,
  update_interval = 2,
  own_window = true,
  own_window_type = 'dock',
  own_window_argb_visual = true,
  own_window_argb_value = 0,
  own_window_hints = 'undecorated,above,sticky,skip_taskbar,skip_pager',
  minimum_width = 520,
  maximum_width = 520,
  gap_x = 0,
  gap_y = 18,
  draw_shades = false,
  draw_outline = false,
  draw_borders = false,
  use_xft = true,
  xftalpha = 1,
  default_color = '#edf4ff',
  uppercase = false,
  use_spacer = 'none',
  override_utf8_locale = true,
  font = 'JetBrains Mono:size=22:bold'
}

conky.text = [[
${alignc}${color #f2f6fd}⬤  ⬤  ⬤  ⬤  ⬤  ⬤  ⬤  ⬤
]]
EOF

    cat > "$cfg_dir/plasma-rice/wallpapers/blue-lake.svg" <<'EOF'
<svg xmlns="http://www.w3.org/2000/svg" width="1920" height="1080" viewBox="0 0 1920 1080">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0%" stop-color="#6f9fd2"/>
      <stop offset="48%" stop-color="#4779ac"/>
      <stop offset="100%" stop-color="#08101d"/>
    </linearGradient>
    <linearGradient id="water" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0%" stop-color="#17314f" stop-opacity="0.50"/>
      <stop offset="100%" stop-color="#050b15" stop-opacity="0.96"/>
    </linearGradient>
    <filter id="blur" x="-20%" y="-20%" width="140%" height="140%">
      <feGaussianBlur stdDeviation="12"/>
    </filter>
    <filter id="soft" x="-20%" y="-20%" width="140%" height="140%">
      <feGaussianBlur stdDeviation="4"/>
    </filter>
  </defs>
  <rect width="1920" height="1080" fill="url(#bg)"/>
  <ellipse cx="960" cy="92" rx="340" ry="110" fill="#c6dbf1" opacity="0.10" filter="url(#blur)"/>
  <g opacity="0.36" filter="url(#soft)">
    <path d="M0 615 C170 578, 300 568, 440 576 S770 606, 960 582 S1360 550, 1605 570 S1830 595, 1920 585 L1920 720 L0 720 Z" fill="#132844"/>
    <path d="M0 675 C220 650, 360 652, 540 662 S920 688, 1190 668 S1605 642, 1920 660 L1920 800 L0 800 Z" fill="#1f3a60" opacity="0.95"/>
  </g>
  <rect x="0" y="590" width="1920" height="490" fill="url(#water)"/>
  <g opacity="0.18" filter="url(#blur)">
    <path d="M0 650 C190 630, 320 632, 520 645 S920 670, 1185 652 S1600 625, 1920 645 L1920 860 L0 860 Z" fill="#aed1ee"/>
  </g>
  <g opacity="0.16">
    <path d="M0 748 C176 780, 340 792, 548 780 S892 742, 1148 754 S1532 798, 1920 774 L1920 1080 L0 1080 Z" fill="#c6dcf2"/>
  </g>
</svg>
EOF
fi

sync_runtime_dir "$common_runtime_dir"
sync_runtime_dir "$src_dir"

if [ "$DESKTOP_ID" = "XFCE" ]; then
    mkdir -p "$cfg_dir/xfce4/terminal"
    cat > "$cfg_dir/xfce4/terminal/terminalrc" <<'EOF'
[Configuration]
ColorBackground=#000000
ColorForeground=#f2f2f2
ColorCursor=#f2f2f2
BackgroundMode=TERMINAL_BACKGROUND_SOLID
FontName=JetBrains Mono 11
EOF
fi
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

if [ "${WELCOME_ENABLED:-1}" = "1" ]; then
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
        nohup xterm -fa 'JetBrains Mono' -fs 11 -geometry 120x34+60+60 -e bash -lc 'fastfetch || true; exec bash' > /home/$user/.rice/welcome.log 2>&1 &
    " > /dev/null 2>&1 || true
) &
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
echo " Desktop : $DESKTOP_NAME"
echo " Display : $disp"
echo " VNC     : localhost:5070"
echo " Browser : http://localhost:6090"
echo "================================================"

wait $SESSION_PID || true
echo "$DESKTOP_NAME exited, keeping container alive for inspection..."
sleep 3600
