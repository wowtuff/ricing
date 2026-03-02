FROM archlinux:latest

#system bootstrap

#keyring
RUN pacman-key --init && pacman-key --populate archlinux

#system sync and upgrade/update, noconfirm for auto 'Y' in Y/n situations
RUN pacman -Syu --noconfirm

# locale
RUN pacman -S --noconfirm glibc && \
    echo "en_US.UTF-8 UTF-8" > /etc/locale.gen && locale-gen && echo "LANG=en_US.UTF-8" > /etc/locale.conf

# core dependencies
RUN pacman -S --noconfirm \
    wayland \
    wayland-protocols \
    xorg-xwayland \
    libinput \
    seatd \
    hyprland \
    xdg-desktop-portal-hyprland \
    wayvnc \
    python \
    python-pip \
    foot \
    dbus \
    bash \
    sudo \
    git \
    wget \
    ttf-jetbrains-mono \
    noto-fonts \
    waybar \
    rofi-wayland \
    swww \
    dunst

# websockify for novnc
RUN pip install websockify --break-system-packages

# novnc for streaming to browser
RUN git clone --depth 1 https://github.com/novnc/noVNC.git /opt/novnc && ln -s /opt/novnc/vnc.html /opt/novnc/index.html

# user setup
RUN useradd -m -G wheel,seat,video,input,audio -s /bin/bash hypruser && echo "hypruser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers && groupadd -f seat

#runtime
RUN mkdir -p /run/user/1000 && chown hypruser:hypruser /run/user/1000 && mkdir -p /home/hypruser/.cache/hyprland && chown -R hypruser:hypruser /home/hypruser

#hyprland config
RUN mkdir -p /home/hypruser/.config/hypr
COPY hyprland.conf /home/hypruser/.config/hypr/hyprland.conf
RUN chown -R hypruser:hypruser /home/hypruser/.config

#start script
COPY start.sh /start.sh
RUN chmod +x /start.sh
EXPOSE 6090
EXPOSE 5070
CMD ["/start.sh"]
