# 一键安装服务器与面板
set -e

# 检测是否有docker
if ! command -v docker &> /dev/null; then
  # 安装docker
  echo "Docker 未安装，正在安装..."
  if curl -s ipinfo.io | grep -q '"country": "CN"'; then
    export DOWNLOAD_URL="https://mirrors.tuna.tsinghua.edu.cn/docker-ce"
  fi
  curl -fsSL https://get.docker.com -o get-docker.sh 
  bash get-docker.sh 
  rm get-docker.sh
  echo "Docker 安装完成。"
fi

# 询问管理密码并确认
read -r -s -p "请设置管理密码: " admin_password
echo
read -r -s -p "请确认管理密码: " admin_password_confirm
echo

if [ "$admin_password" != "$admin_password_confirm" ]; then
    echo "两次输入的密码不一致，请重新运行"
    exit 1
fi

# 询问游戏端口，默认为27015
read -r -p "请输入游戏端口 (默认: 27015): " game_port
game_port=${game_port:-27015}

# 询问管理面板端口，默认为27020
read -r -p "请输入管理面板端口 (默认: 27020): " manager_port
manager_port=${manager_port:-27020}

# 生成随机RCON密码
L4D2_RCON_PASSWORD=$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 16)

mkdir -p /data/l4d2
# 写入docker-compose文件
cat > /data/l4d2/docker-compose.yaml << EOF
volumes:
  addons:
  cfg:

networks:
  l4d2-network:

services:
  l4d2:
    image: laoyutang/l4d2:nightly
    container_name: l4d2
    restart: unless-stopped
    ports:
      - "$game_port:27015"
      - "$game_port:27015/udp"
    volumes:
      - addons:/l4d2/left4dead2/addons
      - cfg:/l4d2/left4dead2/cfg
    networks:
      - l4d2-network
    environment:
      - L4D2_TICK=60 # 30,60,100
      - L4D2_RCON_PASSWORD=$L4D2_RCON_PASSWORD
    logging:
      options:
        max-size: "50m" 
        max-file: "3"

  l4d2-manager:
    image: laoyutang/l4d2-manager:latest
    container_name: l4d2-manager
    restart: unless-stopped
    ports:
      - "$manager_port:27020"
    volumes:
      - addons:/addons
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - L4D2_RESTART_BY_RCON=true
      - L4D2_MANAGER_PASSWORD=$admin_password
      - L4D2_RCON_PASSWORD=$L4D2_RCON_PASSWORD
      - L4D2_RCON_URL=l4d2:27015
    networks:
      - l4d2-network
    logging:
      options:
        max-size: "50m" 
        max-file: "3"
EOF

# ipinfo检测是否是国内环境，国内则增加compose中的镜像
if curl -s ipinfo.io | grep -q '"country": "CN"'; then
  # 询问是否使用镜像源
  read -r -p "检测到国内环境，是否使用国内镜像源？(y/n): " use_mirror
  if [[ "$use_mirror" =~ ^[Yy]$ ]]; then
    # 输入镜像源
    read -r -p "请输入国内镜像源地址 (默认: docker.1ms.run): " mirror_url
    mirror_url=${mirror_url:-docker.1ms.run}
    echo "正在配置compose文件使用国内镜像源..."
    sed -i "s|laoyutang/l4d2:nightly|$mirror_url/laoyutang/l4d2:nightly|" /data/l4d2/docker-compose.yaml
    sed -i "s|laoyutang/l4d2-manager:latest|$mirror_url/laoyutang/l4d2-manager:latest|" /data/l4d2/docker-compose.yaml
  fi
fi

# 启动docker-compose
cd /data/l4d2
docker compose up -d

# 增加软连接
ln -s /var/lib/docker/volumes/l4d2_addons/_data ./addons
ln -s /var/lib/docker/volumes/l4d2_cfg/_data ./cfg

# 输出提示信息
echo "L4D2 服务器和管理面板已安装并启动。"
