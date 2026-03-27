FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    sudo \
    dbus \
    dbus-x11 \
    curl \
    git \
    python3 \
    python3-pip \
    ca-certificates \
    xvfb \
    x11vnc \
    xauth \
    x11-utils \
    x11-xserver-utils \
    xterm \
    dmenu \
    firefox-esr \
    i3-wm \
    i3status \
    fonts-dejavu-core \
    fonts-jetbrains-mono \
    && rm -rf /var/lib/apt/lists/*
RUN python3 -m pip install --break-system-packages websockify
RUN git clone --depth 1 https://github.com/novnc/noVNC.git /opt/novnc && ln -sf /opt/novnc/vnc.html /opt/novnc/index.html
RUN useradd -m -G sudo,audio,video -s /bin/bash hypruser && echo "hypruser ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/hypruser && chmod 0440 /etc/sudoers.d/hypruser
RUN mkdir -p /run/user/1000 /home/hypruser/.config/i3 /home/hypruser/.rice && chown -R hypruser:hypruser /run/user/1000 /home/hypruser
EXPOSE 5070
EXPOSE 6090
CMD ["/bin/bash"]
