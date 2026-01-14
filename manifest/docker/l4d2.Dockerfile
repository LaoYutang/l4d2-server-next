FROM ubuntu:20.04

EXPOSE 27015
EXPOSE 27015/udp

# 环境准备
ENV DEBIAN_FRONTEND=noninteractive
RUN dpkg --add-architecture i386 && \
  apt-get update && \
  apt-get install -y \
  lib32gcc-s1 \
  lib32stdc++6 \
  libc6:i386 \
  libstdc++6:i386 \
  wget \
  vim \
  curl \
  ca-certificates && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/*
RUN mkdir -p /l4d2 /root/steamcmd

# 安装steamcmd并下载l4d2
WORKDIR /root/steamcmd
RUN wget https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz && \
  tar -zxvf steamcmd_linux.tar.gz && \
  rm steamcmd_linux.tar.gz
RUN for i in 1 2 3 4 5; do ./steamcmd.sh +@sSteamCmdForcePlatformType windows +force_install_dir /l4d2 +login anonymous +app_update 222860 validate +quit && break || sleep 15; done
RUN for i in 1 2 3; do ./steamcmd.sh +@sSteamCmdForcePlatformType linux +force_install_dir /l4d2 +login anonymous +app_update 222860 validate +quit && break || sleep 15; done

# 复制插件包
COPY ./cauldron/left4dead2/ /l4d2/left4dead2
RUN mkdir -p /l4d2-backup
RUN cp -r /l4d2/left4dead2/addons /l4d2-backup/
RUN cp -r /l4d2/left4dead2/cfg /l4d2-backup/

# 复制启动脚本
COPY ./start.sh /start.sh
RUN chmod +x /start.sh

CMD ["/start.sh"]