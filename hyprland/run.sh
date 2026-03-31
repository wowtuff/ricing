#!/bin/bash
set -euo pipefail

MYDIR="$(cd "$(dirname "$0")" && pwd)"
IMAGE_BASE="hyprland-preview"
CONTAINER="hyprland-preview"
DATA_VOL="hyprland-preview-data"
PORT_VNC="5070"
PORT_NOVNC="6090"
PROFILE="arch-hyprland"
REFRESH_MODE="refresh"
CMD=""
ARGS=()

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --profile)
                PROFILE="${2:-}"
                shift 2
                ;;
            --)
                shift
                ARGS=("$@")
                break
                ;;
            *)
                if [ -z "$CMD" ]; then
                    CMD="$1"
                else
                    ARGS+=("$1")
                fi
                shift
                ;;
        esac
    done
}

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || {
        echo "error: required command not found: $1"
        exit 1
    }
}

list_profiles() {
    echo "arch-hyprland"
    echo "arch-i3"
    echo "arch-gnome"
    echo "arch-plasma"
    echo "arch-xfce"
    echo "arch-cinnamon"
    echo "arch-mate"
    echo "arch-lxqt"
    echo "debian-i3"
}

set_profile() {
    local common_arch_x11_dir="$MYDIR/profiles/arch-x11-common"
    case "$PROFILE" in
        arch-hyprland)
            IMAGE="$IMAGE_BASE-$PROFILE"
            DOCKERFILE="$MYDIR/profiles/arch-hyprland/Dockerfile"
            START_FILE="$MYDIR/profiles/arch-hyprland/start.sh"
            REFRESH_FILE="$MYDIR/profiles/arch-hyprland/refresh.sh"
            START_CMD="/workspace/profiles/arch-hyprland/start.sh"
            REFRESH_CMD="/workspace/profiles/arch-hyprland/refresh.sh"
            RUNTIME_FILES=(
                "$MYDIR/profiles/arch-hyprland/runtime/hyprland.conf"
                "$MYDIR/profiles/arch-hyprland/runtime/generated.conf"
                "$MYDIR/profiles/arch-hyprland/runtime/kitty"
                "$MYDIR/profiles/arch-hyprland/runtime/waybar"
                "$MYDIR/profiles/arch-hyprland/runtime/rofi"
                "$MYDIR/profiles/arch-hyprland/runtime/dunst"
                "$MYDIR/profiles/arch-hyprland/runtime/swww"
            )
            RESTART_FILES=("$START_FILE" "$REFRESH_FILE")
            BUILD_FILES=("$DOCKERFILE")
            REFRESH_MODE="refresh"
            ;;
        arch-i3)
            IMAGE="$IMAGE_BASE-$PROFILE"
            DOCKERFILE="$MYDIR/profiles/arch-i3/Dockerfile"
            START_FILE="$MYDIR/profiles/arch-i3/start.sh"
            REFRESH_FILE="$MYDIR/profiles/arch-i3/refresh.sh"
            START_CMD="/workspace/profiles/arch-i3/start.sh"
            REFRESH_CMD="/workspace/profiles/arch-i3/refresh.sh"
            RUNTIME_FILES=("$MYDIR/profiles/arch-i3/runtime")
            RESTART_FILES=("$START_FILE" "$REFRESH_FILE")
            BUILD_FILES=("$DOCKERFILE")
            REFRESH_MODE="refresh"
            ;;
        arch-gnome)
            IMAGE="$IMAGE_BASE-$PROFILE"
            DOCKERFILE="$MYDIR/profiles/arch-gnome/Dockerfile"
            START_FILE="$MYDIR/profiles/arch-gnome/start.sh"
            REFRESH_FILE="$MYDIR/profiles/arch-gnome/refresh.sh"
            START_CMD="/workspace/profiles/arch-gnome/start.sh"
            REFRESH_CMD="/workspace/profiles/arch-gnome/refresh.sh"
            RUNTIME_FILES=("$common_arch_x11_dir/runtime" "$MYDIR/profiles/arch-gnome/runtime")
            RESTART_FILES=("$START_FILE" "$REFRESH_FILE" "$common_arch_x11_dir/start.sh" "$common_arch_x11_dir/refresh.sh")
            BUILD_FILES=("$DOCKERFILE")
            REFRESH_MODE="restart"
            ;;
        arch-plasma)
            IMAGE="$IMAGE_BASE-$PROFILE"
            DOCKERFILE="$MYDIR/profiles/arch-plasma/Dockerfile"
            START_FILE="$MYDIR/profiles/arch-plasma/start.sh"
            REFRESH_FILE="$MYDIR/profiles/arch-plasma/refresh.sh"
            START_CMD="/workspace/profiles/arch-plasma/start.sh"
            REFRESH_CMD="/workspace/profiles/arch-plasma/refresh.sh"
            RUNTIME_FILES=("$common_arch_x11_dir/runtime" "$MYDIR/profiles/arch-plasma/runtime")
            RESTART_FILES=("$START_FILE" "$REFRESH_FILE" "$common_arch_x11_dir/start.sh" "$common_arch_x11_dir/refresh.sh")
            BUILD_FILES=("$DOCKERFILE")
            REFRESH_MODE="restart"
            ;;
        arch-xfce)
            IMAGE="$IMAGE_BASE-$PROFILE"
            DOCKERFILE="$MYDIR/profiles/arch-xfce/Dockerfile"
            START_FILE="$MYDIR/profiles/arch-xfce/start.sh"
            REFRESH_FILE="$MYDIR/profiles/arch-xfce/refresh.sh"
            START_CMD="/workspace/profiles/arch-xfce/start.sh"
            REFRESH_CMD="/workspace/profiles/arch-xfce/refresh.sh"
            RUNTIME_FILES=("$common_arch_x11_dir/runtime" "$MYDIR/profiles/arch-xfce/runtime")
            RESTART_FILES=("$START_FILE" "$REFRESH_FILE" "$common_arch_x11_dir/start.sh" "$common_arch_x11_dir/refresh.sh")
            BUILD_FILES=("$DOCKERFILE")
            REFRESH_MODE="restart"
            ;;
        arch-cinnamon)
            IMAGE="$IMAGE_BASE-$PROFILE"
            DOCKERFILE="$MYDIR/profiles/arch-cinnamon/Dockerfile"
            START_FILE="$MYDIR/profiles/arch-cinnamon/start.sh"
            REFRESH_FILE="$MYDIR/profiles/arch-cinnamon/refresh.sh"
            START_CMD="/workspace/profiles/arch-cinnamon/start.sh"
            REFRESH_CMD="/workspace/profiles/arch-cinnamon/refresh.sh"
            RUNTIME_FILES=("$common_arch_x11_dir/runtime" "$MYDIR/profiles/arch-cinnamon/runtime")
            RESTART_FILES=("$START_FILE" "$REFRESH_FILE" "$common_arch_x11_dir/start.sh" "$common_arch_x11_dir/refresh.sh")
            BUILD_FILES=("$DOCKERFILE")
            REFRESH_MODE="restart"
            ;;
        arch-mate)
            IMAGE="$IMAGE_BASE-$PROFILE"
            DOCKERFILE="$MYDIR/profiles/arch-mate/Dockerfile"
            START_FILE="$MYDIR/profiles/arch-mate/start.sh"
            REFRESH_FILE="$MYDIR/profiles/arch-mate/refresh.sh"
            START_CMD="/workspace/profiles/arch-mate/start.sh"
            REFRESH_CMD="/workspace/profiles/arch-mate/refresh.sh"
            RUNTIME_FILES=("$common_arch_x11_dir/runtime" "$MYDIR/profiles/arch-mate/runtime")
            RESTART_FILES=("$START_FILE" "$REFRESH_FILE" "$common_arch_x11_dir/start.sh" "$common_arch_x11_dir/refresh.sh")
            BUILD_FILES=("$DOCKERFILE")
            REFRESH_MODE="restart"
            ;;
        arch-lxqt)
            IMAGE="$IMAGE_BASE-$PROFILE"
            DOCKERFILE="$MYDIR/profiles/arch-lxqt/Dockerfile"
            START_FILE="$MYDIR/profiles/arch-lxqt/start.sh"
            REFRESH_FILE="$MYDIR/profiles/arch-lxqt/refresh.sh"
            START_CMD="/workspace/profiles/arch-lxqt/start.sh"
            REFRESH_CMD="/workspace/profiles/arch-lxqt/refresh.sh"
            RUNTIME_FILES=("$common_arch_x11_dir/runtime" "$MYDIR/profiles/arch-lxqt/runtime")
            RESTART_FILES=("$START_FILE" "$REFRESH_FILE" "$common_arch_x11_dir/start.sh" "$common_arch_x11_dir/refresh.sh")
            BUILD_FILES=("$DOCKERFILE")
            REFRESH_MODE="restart"
            ;;
        debian-i3)
            IMAGE="$IMAGE_BASE-$PROFILE"
            DOCKERFILE="$MYDIR/profiles/debian-i3/Dockerfile"
            START_FILE="$MYDIR/profiles/debian-i3/start.sh"
            REFRESH_FILE="$MYDIR/profiles/debian-i3/refresh.sh"
            START_CMD="/workspace/profiles/debian-i3/start.sh"
            REFRESH_CMD="/workspace/profiles/debian-i3/refresh.sh"
            RUNTIME_FILES=("$MYDIR/profiles/debian-i3/runtime")
            RESTART_FILES=("$START_FILE" "$REFRESH_FILE")
            BUILD_FILES=("$DOCKERFILE")
            REFRESH_MODE="refresh"
            ;;
        *)
            echo "unknown profile: $PROFILE"
            exit 1
            ;;
    esac
}

ensure_host_paths() {
    mkdir -p "$(dirname "$START_FILE")" "$(dirname "$REFRESH_FILE")"
    touch "$START_FILE" "$REFRESH_FILE"
    for path in "${RUNTIME_FILES[@]}"; do
        if [ -e "$path" ]; then
            continue
        fi
        case "$path" in
            *.conf|*.ini|*.json|*.css|*.txt)
                mkdir -p "$(dirname "$path")"
                touch "$path"
                ;;
            *)
                mkdir -p "$path"
                ;;
        esac
    done
}

is_running() {
    docker ps --format '{{.Names}}' | grep -qx "$CONTAINER"
}

exists() {
    docker ps -a --format '{{.Names}}' | grep -qx "$CONTAINER"
}

current_profile() {
    docker inspect -f '{{ index .Config.Labels "wow.preview.profile" }}' "$CONTAINER" 2>/dev/null || true
}

need_profile() {
    if ! is_running; then
        echo "container $CONTAINER is not running"
        exit 1
    fi
    local cur
    cur=$(current_profile)
    if [ -n "$cur" ] && [ "$cur" != "$PROFILE" ]; then
        echo "container $CONTAINER is running profile $cur"
        echo "use --profile $cur or restart with --profile $PROFILE"
        exit 1
    fi
}

show_endpoints() {
    echo ""
    echo "================================================"
    echo " Profile  : $PROFILE"
    echo " Container: $CONTAINER"
    echo " Image    : $IMAGE"
    echo " VNC      : localhost:$PORT_VNC"
    echo " noVNC    : http://localhost:$PORT_NOVNC"
    echo "================================================"
}

build_image() {
    echo "building $IMAGE..."
    if [ "$PROFILE" = "arch-plasma" ]; then
        local build_container="${CONTAINER}-build-${PROFILE}"
        docker rm -f "$build_container" >/dev/null 2>&1 || true
        docker run \
            --name "$build_container" \
            --network host \
            archlinux:latest \
            /bin/bash -lc '
                set -euo pipefail
                printf "%s\n" \
                    "142.0.200.124 mirrors.kernel.org" \
                    "194.156.163.63 geo.mirror.pkgbuild.com" \
                    "180.150.156.88 mirror.rackspace.com" \
                    >> /etc/hosts
                printf "%s\n" \
                    "Server = https://mirrors.kernel.org/archlinux/\$repo/os/\$arch" \
                    "Server = https://geo.mirror.pkgbuild.com/\$repo/os/\$arch" \
                    "Server = https://mirror.rackspace.com/\$repo/os/\$arch" \
                    > /etc/pacman.d/mirrorlist
                pacman-key --init && pacman-key --populate archlinux
                pacman -Sy --noconfirm --disable-download-timeout archlinux-keyring
                pacman -S --noconfirm --disable-download-timeout \
                    bash sudo dbus curl git python python-pip ca-certificates \
                    xorg-server-xvfb x11vnc xorg-xauth xorg-xrdb xorg-xsetroot \
                    xterm dmenu jack2 firefox kitty fastfetch mesa xdg-user-dirs \
                    ttf-dejavu ttf-jetbrains-mono noto-fonts noto-fonts-emoji \
                    plasma-desktop plasma-workspace conky htop wmctrl xdotool
                echo "en_US.UTF-8 UTF-8" > /etc/locale.gen
                locale-gen
                echo "LANG=en_US.UTF-8" > /etc/locale.conf
                python -m pip install --break-system-packages websockify
                git clone --depth 1 https://github.com/novnc/noVNC.git /opt/novnc
                ln -sf /opt/novnc/vnc.html /opt/novnc/index.html
                useradd -m -G wheel,video,audio -s /bin/bash hypruser
                echo "hypruser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers
                mkdir -p /run/user/1000 /home/hypruser/.config /home/hypruser/.rice
                chown -R hypruser:hypruser /run/user/1000 /home/hypruser
            '
        docker commit "$build_container" "$IMAGE" > /dev/null
        docker rm -f "$build_container" >/dev/null 2>&1 || true
        return
    fi
    docker build --network host -t "$IMAGE" -f "$DOCKERFILE" "$MYDIR"
}

replace_if_needed() {
    local cur
    if ! is_running; then
        return
    fi
    cur=$(current_profile)
    if [ "$cur" = "$PROFILE" ]; then
        return
    fi
    echo "switching preview from ${cur:-unknown} to $PROFILE..."
    docker rm -f "$CONTAINER" 2>/dev/null || true
}

run_attached() {
    ensure_host_paths
    replace_if_needed
    if is_running; then
        need_profile
        show_endpoints
        docker logs -f "$CONTAINER"
        return
    fi
    docker rm -f "$CONTAINER" 2>/dev/null || true
    docker volume create "$DATA_VOL" >/dev/null
    echo "starting $IMAGE..."
    docker run -it \
        --name "$CONTAINER" \
        --hostname "$CONTAINER" \
        --privileged \
        --rm \
        --label "wow.preview.profile=$PROFILE" \
        -p "$PORT_VNC:$PORT_VNC" \
        -p "$PORT_NOVNC:$PORT_NOVNC" \
        -e TERM=xterm-256color \
        -e COLORTERM=truecolor \
        -v "$MYDIR:/workspace" \
        -v "$DATA_VOL:/home/hypruser/.local/share" \
        "$IMAGE" /bin/bash "$START_CMD"
}

run_detached() {
    ensure_host_paths
    replace_if_needed
    if is_running; then
        need_profile
        show_endpoints
        echo "use: ./run.sh logs"
        return
    fi
    docker rm -f "$CONTAINER" 2>/dev/null || true
    docker volume create "$DATA_VOL" >/dev/null
    echo "starting $IMAGE in detached mode..."
    docker run -d \
        --name "$CONTAINER" \
        --hostname "$CONTAINER" \
        --privileged \
        --rm \
        --label "wow.preview.profile=$PROFILE" \
        -p "$PORT_VNC:$PORT_VNC" \
        -p "$PORT_NOVNC:$PORT_NOVNC" \
        -e TERM=xterm-256color \
        -e COLORTERM=truecolor \
        -v "$MYDIR:/workspace" \
        -v "$DATA_VOL:/home/hypruser/.local/share" \
        "$IMAGE" /bin/bash "$START_CMD" >/dev/null
    show_endpoints
    echo "use: ./run.sh logs"
}

restart_detached() {
    docker rm -f "$CONTAINER" 2>/dev/null || true
    run_detached
}

open_root_shell() {
    need_profile
    docker exec -it -u root "$CONTAINER" bash
}

open_user_shell() {
    need_profile
    docker exec -it -u hypruser "$CONTAINER" bash -lc '
        envf=/home/hypruser/.rice/session.env
        [ -f "$envf" ] && . "$envf"
        exec bash -i
    '
}

exec_in_user() {
    need_profile
    if [ ${#ARGS[@]} -eq 0 ]; then
        echo "usage: ./run.sh exec -- <command>"
        exit 1
    fi
    docker exec -it -u hypruser "$CONTAINER" bash -lc '
        envf=/home/hypruser/.rice/session.env
        [ -f "$envf" ] && . "$envf"
        exec "$@"
    ' bash "${ARGS[@]}"
}

show_logs() {
    need_profile
    docker logs -f "$CONTAINER"
}

show_status() {
    echo "profile:"
    if exists; then
        current_profile
    else
        echo "$PROFILE"
    fi
    echo ""
    echo "image:"
    docker images | awk 'NR==1 || $1 ~ /^'"$IMAGE"'$/'
    echo ""
    echo "container:"
    docker ps -a --filter "name=^${CONTAINER}$"
    if exists; then
        echo ""
        echo "ports:"
        docker port "$CONTAINER" || true
    fi
}

inspect_container() {
    if ! exists; then
        echo "container $CONTAINER does not exist"
        exit 1
    fi
    echo "----- docker inspect -----"
    docker inspect "$CONTAINER"
    echo ""
    echo "----- process list -----"
    docker exec -u root "$CONTAINER" ps aux || true
    echo ""
    echo "----- listening sockets -----"
    docker exec -u root "$CONTAINER" sh -c 'ss -lntp || netstat -lntp || true'
    echo ""
    echo "----- session env -----"
    docker exec -u root "$CONTAINER" sh -c 'cat /home/hypruser/.rice/session.env 2>/dev/null || true'
    echo ""
    echo "----- hypr cache -----"
    docker exec -u root "$CONTAINER" sh -c 'ls -la /home/hypruser/.cache/hyprland || true && echo && tail -n 200 /home/hypruser/.cache/hyprland/* 2>/dev/null || true'
}

refresh_container() {
    need_profile
    if [ "$REFRESH_MODE" = "restart" ]; then
        restart_detached
        return
    fi
    docker exec -u hypruser "$CONTAINER" /bin/bash "$REFRESH_CMD"
}

stop_container() {
    if exists; then
        docker stop "$CONTAINER"
    else
        echo "container $CONTAINER is not running"
    fi
}

clean_all() {
    echo "removing container, image, and debug volume..."
    docker rm -f "$CONTAINER" 2>/dev/null || true
    docker rmi "$IMAGE" 2>/dev/null || true
    docker volume rm "$DATA_VOL" 2>/dev/null || true
}

rebuild_image() {
    docker rm -f "$CONTAINER" 2>/dev/null || true
    docker rmi "$IMAGE" 2>/dev/null || true
    build_image
}

hash_paths() {
    local out
    out=$(
        {
            for p in "$@"; do
                [ -e "$p" ] || continue
                if [ -d "$p" ]; then
                    find "$p" -type f -print0
                else
                    printf '%s\0' "$p"
                fi
            done
        } | sort -z | xargs -0 -r sha256sum 2>/dev/null | sha256sum 2>/dev/null | awk '{print $1}'
    )
    if [ -n "$out" ]; then
        echo "$out"
    else
        echo "none"
    fi
}

watch_files() {
    need_profile
    local last_rt last_rs last_bd now_rt now_rs now_bd
    last_rt=$(hash_paths "${RUNTIME_FILES[@]}")
    last_rs=$(hash_paths "${RESTART_FILES[@]}")
    last_bd=$(hash_paths "${BUILD_FILES[@]}")
    echo "watching $PROFILE..."
    while true; do
        sleep 1
        now_rt=$(hash_paths "${RUNTIME_FILES[@]}")
        now_rs=$(hash_paths "${RESTART_FILES[@]}")
        now_bd=$(hash_paths "${BUILD_FILES[@]}")
        if [ "$now_bd" != "$last_bd" ]; then
            echo "dockerfile changed, rebuild required"
            last_bd=$now_bd
        fi
        if [ "$now_rs" != "$last_rs" ]; then
            echo "start script changed, restarting preview..."
            restart_detached
            last_rt=$(hash_paths "${RUNTIME_FILES[@]}")
            last_rs=$(hash_paths "${RESTART_FILES[@]}")
            last_bd=$(hash_paths "${BUILD_FILES[@]}")
            continue
        fi
        if [ "$now_rt" != "$last_rt" ]; then
            if [ "$REFRESH_MODE" = "restart" ]; then
                echo "runtime files changed, restarting preview..."
            else
                echo "runtime files changed, refreshing preview..."
            fi
            refresh_container
            last_rt=$now_rt
        fi
    done
}

print_usage() {
    cat <<EOF
Usage: ./run.sh <command> [--profile name] [-- extra args]

Commands:
  list        List profiles
  build       Build the Docker image
  rebuild     Remove old image and rebuild
  run         Run attached
  up          Run detached
  restart     Restart detached
  shell       Open session shell in running container
  user        Alias for shell
  root        Open root shell in running container
  exec        Run command in live preview session
  refresh     Reload the running preview
  watch       Watch files and refresh or restart
  logs        Follow container logs
  status      Show image/container status
  inspect     Dump useful debug info from container
  stop        Stop container
  clean       Remove container, image, and debug volume
EOF
}

main() {
    parse_args "$@"
    case "${CMD:-}" in
        list|"")
            ;;
        *)
            require_cmd docker
            ;;
    esac
    set_profile
    case "${CMD:-}" in
        list)
            list_profiles
            ;;
        build)
            build_image
            ;;
        rebuild)
            rebuild_image
            ;;
        run)
            run_attached
            ;;
        up)
            run_detached
            ;;
        restart)
            restart_detached
            ;;
        shell|user)
            open_user_shell
            ;;
        root)
            open_root_shell
            ;;
        exec)
            exec_in_user
            ;;
        refresh)
            refresh_container
            ;;
        watch)
            watch_files
            ;;
        logs)
            show_logs
            ;;
        status)
            show_status
            ;;
        inspect)
            inspect_container
            ;;
        stop)
            stop_container
            ;;
        clean)
            clean_all
            ;;
        *)
            print_usage
            exit 1
            ;;
    esac
}

main "$@"
