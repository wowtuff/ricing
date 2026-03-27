## what's inside

- `arch-hyprland` for the original headless Hyprland preview
- `arch-i3` for an Arch X11 i3 preview
- `debian-i3` for a Debian X11 i3 preview
- one live preview container on `localhost:5070` and `http://localhost:6090`
- session-aware `shell`, `exec`, `refresh`, and `watch` commands

## quick start

```bash
./run.sh list
./run.sh build
./run.sh up
./run.sh exec -- bash -lc 'printf "%s\n" "$XDG_SESSION_TYPE $XDG_CURRENT_DESKTOP"'
```

switch profiles with `--profile`:

```bash
./run.sh build --profile arch-i3
./run.sh up --profile arch-i3
./run.sh build --profile debian-i3
./run.sh up --profile debian-i3
```

## available commands

- `list` shows shipped profiles
- `build` builds the selected profile image
- `rebuild` removes the selected image and rebuilds it
- `run` starts or reuses the selected preview and streams logs
- `up` starts or reuses the selected preview in the background
- `restart` restarts the selected preview
- `shell` opens a session shell inside the live preview
- `user` is an alias for `shell`
- `root` opens a root shell inside the live container
- `exec` runs a command inside the live preview session
- `refresh` reloads the running preview
- `watch` watches files and refreshes or restarts when they change
- `logs` follows container logs
- `status` shows image and container state
- `inspect` dumps useful debug info
- `stop` stops the running preview
- `clean` removes the container, image, and data volume

## how it works

- `run.sh` keeps one active preview container and swaps it when you change profiles
- the whole `hyprland/` folder is mounted into the container as `/workspace`
- runtime files come from the mounted workspace, so config and startup edits do not need an image rebuild
- `shell` and `exec` load the live session env, so commands affect the same GUI you see in noVNC
- `watch` refreshes runtime config changes, restarts when startup scripts change, and tells you when a rebuild is required

## profile notes

- `arch-hyprland` uses Hyprland, `wayvnc`, and noVNC
- `arch-i3` uses Xvfb, i3, `x11vnc`, and noVNC
- `debian-i3` uses the same X11 preview path on Debian

## troubleshooting

```bash
./run.sh status
./run.sh logs
./run.sh inspect
./run.sh shell
```
