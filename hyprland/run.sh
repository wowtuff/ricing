#!/bin/bash
set -euo pipefail

IMAGE="hyprland-preview"
CONTAINER="hyprland-preview"
MYDIR="$(cd "$(dirname "$0")" && pwd)"
DEBUG_VOLUME="hyprland-preview-data"

PORT_VNC="5070"
PORT_NOVNC="6090"

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || {
        echo "error: required command not found: $1"
        exit 1
    }
}

is_running() {
    docker ps --format '{{.Names}}' | grep -qx "$CONTAINER"
}

exists() {
    docker ps -a --format '{{.Names}}' | grep -qx "$CONTAINER"
}

ensure_host_paths() {
    mkdir -p \
        "$MYDIR/kitty" \
        "$MYDIR/waybar" \
        "$MYDIR/rofi" \
        "$MYDIR/dunst" \
        "$MYDIR/swww"

    touch \
        "$MYDIR/generated.conf" \
        "$MYDIR/start.sh" \
        "$MYDIR/hyprland.conf"
}

show_endpoints() {
    echo ""
    echo "================================================"
    echo " Container: $CONTAINER"
    echo " Image    : $IMAGE"
    echo " VNC      : localhost:$PORT_VNC"
    echo " noVNC    : http://localhost:$PORT_NOVNC"
    echo "================================================"
}

build_image() {
    echo "building $IMAGE..."
    docker build -t "$IMAGE" .
}

run_container() {
    ensure_host_paths

    echo "starting $IMAGE..."
    docker rm -f "$CONTAINER" 2>/dev/null || true
    docker volume create "$DEBUG_VOLUME" >/dev/null

    docker run -it \
        --name "$CONTAINER" \
        --hostname "$CONTAINER" \
        --privileged \
        --rm \
        -p "$PORT_VNC:$PORT_VNC" \
        -p "$PORT_NOVNC:$PORT_NOVNC" \
        -e TERM=xterm-256color \
        -e COLORTERM=truecolor \
        -v "$MYDIR/start.sh:/start.sh" \
        -v "$MYDIR/hyprland.conf:/home/hypruser/.config/hypr/hyprland.conf" \
        -v "$MYDIR/generated.conf:/home/hypruser/.config/hypr/generated.conf" \
        -v "$MYDIR/kitty:/home/hypruser/.config/kitty" \
        -v "$MYDIR/waybar:/home/hypruser/.config/waybar" \
        -v "$MYDIR/rofi:/home/hypruser/.config/rofi" \
        -v "$MYDIR/dunst:/home/hypruser/.config/dunst" \
        -v "$MYDIR/swww:/home/hypruser/.config/swww" \
        -v "$DEBUG_VOLUME:/home/hypruser/.local/share" \
        "$IMAGE"
}

run_container_detached() {
    ensure_host_paths

    echo "starting $IMAGE in detached mode..."
    docker rm -f "$CONTAINER" 2>/dev/null || true
    docker volume create "$DEBUG_VOLUME" >/dev/null

    docker run -d \
        --name "$CONTAINER" \
        --hostname "$CONTAINER" \
        --privileged \
        --rm \
        -p "$PORT_VNC:$PORT_VNC" \
        -p "$PORT_NOVNC:$PORT_NOVNC" \
        -e TERM=xterm-256color \
        -e COLORTERM=truecolor \
        -v "$MYDIR/start.sh:/start.sh" \
        -v "$MYDIR/hyprland.conf:/home/hypruser/.config/hypr/hyprland.conf" \
        -v "$MYDIR/generated.conf:/home/hypruser/.config/hypr/generated.conf" \
        -v "$MYDIR/kitty:/home/hypruser/.config/kitty" \
        -v "$MYDIR/waybar:/home/hypruser/.config/waybar" \
        -v "$MYDIR/rofi:/home/hypruser/.config/rofi" \
        -v "$MYDIR/dunst:/home/hypruser/.config/dunst" \
        -v "$MYDIR/swww:/home/hypruser/.config/swww" \
        -v "$DEBUG_VOLUME:/home/hypruser/.local/share" \
        "$IMAGE" >/dev/null

    show_endpoints
    echo "use: ./run.sh logs"
}

open_root_shell() {
    if ! is_running; then
        echo "container $CONTAINER is not running"
        exit 1
    fi
    echo "opening root shell..."
    docker exec -it -u root "$CONTAINER" bash
}

open_user_shell() {
    if ! is_running; then
        echo "container $CONTAINER is not running"
        exit 1
    fi
    echo "opening hypruser shell..."
    docker exec -it -u hypruser "$CONTAINER" bash
}

show_logs() {
    if ! is_running; then
        echo "container $CONTAINER is not running"
        exit 1
    fi
    docker logs -f "$CONTAINER"
}

show_status() {
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
    echo "----- /run/user/1000 -----"
    docker exec -u root "$CONTAINER" sh -c 'ls -la /run/user/1000 && echo && find /run/user/1000 -maxdepth 3 -type s -o -type f 2>/dev/null | sort' || true

    echo ""
    echo "----- hypr cache -----"
    docker exec -u root "$CONTAINER" sh -c 'ls -la /home/hypruser/.cache/hyprland || true && echo && tail -n 200 /home/hypruser/.cache/hyprland/* 2>/dev/null || true'
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
    docker volume rm "$DEBUG_VOLUME" 2>/dev/null || true
}

rebuild_image() {
    docker rm -f "$CONTAINER" 2>/dev/null || true
    docker rmi "$IMAGE" 2>/dev/null || true
    build_image
}

restart_detached() {
    docker rm -f "$CONTAINER" 2>/dev/null || true
    run_container_detached
}

print_usage() {
    cat <<EOF
Usage: ./run.sh <command>

Commands:
  build       Build the Docker image
  rebuild     Remove old image and rebuild
  run         Run attached
  up          Run detached
  restart     Restart detached
  shell       Open root shell in running container
  user        Open hypruser shell in running container
  logs        Follow container logs
  status      Show image/container status
  inspect     Dump useful debug info from container
  stop        Stop container
  clean       Remove container, image, and debug volume
EOF
}

main() {
    require_cmd docker

    case "${1:-}" in
        build)
            build_image
            ;;
        rebuild)
            rebuild_image
            ;;
        run)
            run_container
            ;;
        up)
            run_container_detached
            ;;
        restart)
            restart_detached
            ;;
        shell)
            open_root_shell
            ;;
        user)
            open_user_shell
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
