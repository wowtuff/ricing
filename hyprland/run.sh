#!/bin/bash
set -euo pipefail

MYDIR="$(cd "$(dirname "$0")" && pwd)"
IMAGE_BASE="hyprland-preview"
CONTAINER="hyprland-preview"
DATA_VOL="hyprland-preview-data"
PORT_VNC="5070"
PORT_NOVNC="6090"
PROFILE="arch-hyprland"
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
}

set_profile() {
    case "$PROFILE" in
        arch-hyprland)
            IMAGE="$IMAGE_BASE-$PROFILE"
            DOCKERFILE="$MYDIR/Dockerfile"
            START_FILE="$MYDIR/start.sh"
            REFRESH_FILE="$MYDIR/refresh.sh"
            RUNTIME_FILES=(
                "$MYDIR/hyprland.conf"
                "$MYDIR/generated.conf"
                "$MYDIR/kitty"
                "$MYDIR/waybar"
                "$MYDIR/rofi"
                "$MYDIR/dunst"
                "$MYDIR/swww"
            )
            RESTART_FILES=("$START_FILE" "$REFRESH_FILE")
            BUILD_FILES=("$DOCKERFILE")
            ;;
        *)
            echo "unknown profile: $PROFILE"
            exit 1
            ;;
    esac
}

ensure_host_paths() {
    mkdir -p "$MYDIR/kitty" "$MYDIR/waybar" "$MYDIR/rofi" "$MYDIR/dunst" "$MYDIR/swww"
    touch "$MYDIR/generated.conf" "$MYDIR/hyprland.conf" "$START_FILE" "$REFRESH_FILE"
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
    docker build -t "$IMAGE" -f "$DOCKERFILE" "$MYDIR"
}

run_attached() {
    ensure_host_paths
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
        "$IMAGE" /bin/bash /workspace/start.sh
}

run_detached() {
    ensure_host_paths
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
        "$IMAGE" /bin/bash /workspace/start.sh >/dev/null
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
    docker exec -u hypruser "$CONTAINER" /bin/bash /workspace/refresh.sh
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
            echo "runtime files changed, refreshing preview..."
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
    require_cmd docker
    parse_args "$@"
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
