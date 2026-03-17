## what's inside

- **archlinux** as the base
- **hyprland 0.53.3** (pinned from archive)
- **waybar**, **rofi**, **foot**, **kitty** for that classic hyprland rice
- **wayvnc** + **noVNC** so you can access the desktop from your browser
- **dunst** for notifications
- **swww** for wallpaper
- **grim** + **slurp** for screenshots

## quick start

```bash
# build the docker image
./run.sh build

# run it (attached - you'll see logs)
./run.sh run

# or run in background
./run.sh up

# check what's running
./run.sh status
```

## accessing the desktop

once the container is running:

- **VNC client**: connect to `localhost:5070`
- **browser**: open `http://localhost:6090`

the default config uses:
- `super + return` → open terminal (foot)
- `super + d` → app launcher (rofi)
- `super + q` → close window
- `super + f` → fullscreen
- `super + e` → open kitty

## available commands

| command | what it does |
|---------|--------------|
| `build` | build the docker image |
| `rebuild` | nuke old image and rebuild |
| `run` | start container attached |
| `up` | start container detached |
| `restart` | restart detached container |
| `shell` | get root shell inside container |
| `user` | get hypruser shell inside container |
| `logs` | follow container logs |
| `status` | show image/container status |
| `inspect` | dump debug info |
| `stop` | stop the container |
| `clean` | remove container, image, and volume |

## how it works

### dockerfile
sets up archlinux, installs all the wayland/hyprland dependencies, downloads a pinned hyprland 0.53.3 from the arch archive (since it's more stable than latest), and adds noVNC for browser access.

### run.sh
handles building and running the container. it mounts your local config directories into the container so you can edit configs on your host and see changes immediately.

### start.sh
the container entrypoint. it:
1. starts **seatd** (seat daemon for logind)
2. launches **hyprland** in headless mode
3. creates a virtual display
4. starts **wayvnc** on port 5070
5. starts **noVNC** websocket proxy on port 6090

### hyprland.conf
basic hyprland config. it sources `~/.config/hypr/generated.conf` at the end so you can add your own stuff there without touching the main config.

## project structure

```
.
├── dockerfile          # builds the image
├── run.sh              # build & run script
├── start.sh            # container entrypoint
├── hyprland.conf       # hyprland config
├── generated.conf      # your custom config (gitignored)
├── kitty/              # kitty terminal config
├── waybar/             # waybar config
├── rofi/               # rofi config
├── dunst/              # dunst config
└── swww/               # swww wallpaper config
```

## notes

- the container runs as user `hypruser` (uid 1000)
- all your config dirs are mounted from your host, so edits are instant
- the display runs headless, which is why you need VNC/noVNC to see anything
- if hyprland crashes, the container stays alive for 60 minutes so you can inspect it with `./run.sh inspect`

## troubleshooting

```bash
# can't connect to VNC?
./run.sh logs

# check what's actually running inside
./run.sh inspect

# get a shell to debug
./run.sh shell
```

enjoy your preview :)
