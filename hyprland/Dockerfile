FROM archlinux:latest

RUN printf '%s\n' \
    'Server = https://geo.mirror.pkgbuild.com/$repo/os/$arch' \
    'Server = https://mirrors.kernel.org/archlinux/$repo/os/$arch' \
    > /etc/pacman.d/mirrorlist

# initial keyring
RUN pacman-key --init && pacman-key --populate archlinux

# refresh normal repos first
RUN pacman -Syyu --noconfirm --disable-download-timeout

# install base tools first from normal mirrors
RUN pacman -S --noconfirm --disable-download-timeout \
    glibc \
    bash \
    sudo \
    dbus \
    seatd \
    curl \
    wget \
    git \
    jq \
    python \
    python-pip \
    ca-certificates

# set locale
RUN echo "en_US.UTF-8 UTF-8" > /etc/locale.gen && \
    locale-gen && \
    echo "LANG=en_US.UTF-8" > /etc/locale.conf

ENV LANG=en_US.UTF-8
ENV LC_ALL=en_US.UTF-8

# install non-Hyprland preview stack from normal mirrors
RUN pacman -S --noconfirm --disable-download-timeout \
    wayland \
    wayland-protocols \
    xorg-xwayland \
    libinput \
    wayvnc \
    jack2 \
    foot \
    kitty \
    firefox \
    waybar \
    kitty \
    fastfetch \
    rofi-wayland \
    swww \
    dunst \
    wl-clipboard \
    grim \
    slurp \
    noto-fonts \
    noto-fonts-emoji \
    ttf-jetbrains-mono \
    ttf-nerd-fonts-symbols \
    base-devel

# download a pinned pre-0.54 Hyprland stack from archive()
RUN set -eux; \
    mkdir -p /tmp/hyprpkgs; \
    curl -fL --retry 5 --retry-delay 3 -o /tmp/hyprpkgs/hyprutils.pkg.tar.zst \
      https://archive.archlinux.org/packages/h/hyprutils/hyprutils-0.11.0-1-x86_64.pkg.tar.zst; \
    curl -fL --retry 5 --retry-delay 3 -o /tmp/hyprpkgs/hyprgraphics.pkg.tar.zst \
      https://archive.archlinux.org/packages/h/hyprgraphics/hyprgraphics-0.5.0-1-x86_64.pkg.tar.zst; \
    curl -fL --retry 5 --retry-delay 3 -o /tmp/hyprpkgs/hyprcursor.pkg.tar.zst \
      https://archive.archlinux.org/packages/h/hyprcursor/hyprcursor-0.1.13-3-x86_64.pkg.tar.zst; \
    curl -fL --retry 5 --retry-delay 3 -o /tmp/hyprpkgs/hyprlang.pkg.tar.zst \
      https://archive.archlinux.org/packages/h/hyprlang/hyprlang-0.6.8-1-x86_64.pkg.tar.zst; \
    curl -fL --retry 5 --retry-delay 3 -o /tmp/hyprpkgs/aquamarine.pkg.tar.zst \
      https://archive.archlinux.org/packages/a/aquamarine/aquamarine-0.10.0-2-x86_64.pkg.tar.zst; \
    curl -fL --retry 5 --retry-delay 3 -o /tmp/hyprpkgs/hyprland.pkg.tar.zst \
      https://archive.archlinux.org/packages/h/hyprland/hyprland-0.53.3-2-x86_64.pkg.tar.zst; \
    pacman -U --noconfirm /tmp/hyprpkgs/*.pkg.tar.zst; \
    rm -rf /tmp/hyprpkgs

# noVNC websocket proxy
RUN pip install --break-system-packages websockify

# noVNC frontend
RUN git clone --depth 1 https://github.com/novnc/noVNC.git /opt/novnc && \
    ln -sf /opt/novnc/vnc.html /opt/novnc/index.html

# user and groups
RUN groupadd -f seat && \
    useradd -m -G wheel,seat,video,input,audio -s /bin/bash hypruser && \
    echo "hypruser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# runtime + config dirs
RUN mkdir -p /run/user/1000 && \
    chown hypruser:hypruser /run/user/1000 && \
    chmod 700 /run/user/1000 && \
    mkdir -p /home/hypruser/.cache/hyprland && \
    mkdir -p /home/hypruser/.config/hypr && \
    mkdir -p /home/hypruser/.config/kitty && \
    mkdir -p /home/hypruser/.config/waybar && \
    mkdir -p /home/hypruser/.config/rofi && \
    mkdir -p /home/hypruser/.config/dunst && \
    mkdir -p /home/hypruser/.config/swww && \
    mkdir -p /home/hypruser/.rice && \
    chown -R hypruser:hypruser /home/hypruser

COPY hyprland.conf /home/hypruser/.config/hypr/hyprland.conf
COPY start.sh /start.sh

RUN touch /home/hypruser/.config/hypr/generated.conf && \
    chown hypruser:hypruser /home/hypruser/.config/hypr/generated.conf && \
    chown -R hypruser:hypruser /home/hypruser/.config && \
    chmod +x /start.sh

EXPOSE 5070
EXPOSE 6090

CMD ["/start.sh"]
