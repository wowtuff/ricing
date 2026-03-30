FROM archlinux:latest

RUN printf '%s\n' \
    'Server = https://geo.mirror.pkgbuild.com/$repo/os/$arch' \
    'Server = https://mirrors.kernel.org/archlinux/$repo/os/$arch' \
    > /etc/pacman.d/mirrorlist

RUN pacman-key --init && pacman-key --populate archlinux
RUN pacman -Syyu --noconfirm --disable-download-timeout && pacman -S --noconfirm --disable-download-timeout \
    bash \
    sudo \
    dbus \
    curl \
    git \
    python \
    python-pip \
    ca-certificates \
    xorg-server-xvfb \
    x11vnc \
    xorg-xauth \
    xorg-xrdb \
    xorg-xsetroot \
    xterm \
    dmenu \
    jack2 \
    firefox \
    kitty \
    fastfetch \
    mesa \
    xdg-user-dirs \
    ttf-dejavu \
    ttf-jetbrains-mono \
    noto-fonts \
    noto-fonts-emoji \
    xfce4 \
    xfce4-goodies \
    xfce4-terminal \
    thunar
RUN echo "en_US.UTF-8 UTF-8" > /etc/locale.gen && locale-gen && echo "LANG=en_US.UTF-8" > /etc/locale.conf
ENV LANG=en_US.UTF-8
ENV LC_ALL=en_US.UTF-8
RUN pip install --break-system-packages websockify
RUN git clone --depth 1 https://github.com/novnc/noVNC.git /opt/novnc && ln -sf /opt/novnc/vnc.html /opt/novnc/index.html
RUN useradd -m -G wheel,video,audio -s /bin/bash hypruser && echo "hypruser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers
RUN mkdir -p /run/user/1000 /home/hypruser/.config /home/hypruser/.rice && chown -R hypruser:hypruser /run/user/1000 /home/hypruser
EXPOSE 5070
EXPOSE 6090
CMD ["/bin/bash"]
