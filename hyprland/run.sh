#!/bin/bash

IMAGE="hyprland-preview"
case "$1" in
    build)
        echo "building $IMAGE"
        docker build -t $IMAGE .
        ;;
    run)
        echo "starting $IMAGE..."
        docker run -it --privileged \
            -p 5070:5070 \
            -p 6090:6090 \
            --name hyprland-preview \
            --rm \
            $IMAGE
        ;;
    shell)
        echo "opening root shell"
        docker exec -it -u root hyprland-preview bash
        ;;
    user)
        echo "opening hypruser shell"
        docker exec -it -u hypruser hyprland-preview bash
        ;;
    stop)
        docker stop hyprland-preview
        ;;
    rebuild)
        docker rmi $IMAGE 2>/dev/null || true
        docker build -t $IMAGE .
        ;;
    *)
        echo "build: build the image"
        echo "run: start the container"
        echo "shell: root shell in running container"
        echo "user: hypruser shell in running container"
        echo "stop: stop the container"
        echo "rebuild: force rebuild from scratch"
        ;;
esac
