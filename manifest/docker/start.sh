#!/bin/bash

echo "检查并初始化插件文件..."

# 控制tick配置文件，环境变量L4D2_TICK
REAL_TICK=${L4D2_TICK:-30}  # 默认值为30
if [ "${L4D2_TICK}" = "60" ]; then
    echo "设置tickrate为60"
    cp /l4d2/left4dead2/cfg/server.cfg.60tick /l4d2/left4dead2/cfg/server.cfg
    REAL_TICK=60
elif [ "${L4D2_TICK}" = "100" ]; then
    echo "设置tickrate为100"
    cp /l4d2/left4dead2/cfg/server.cfg.100tick /l4d2/left4dead2/cfg/server.cfg
    REAL_TICK=100
else
    echo "设置tickrate为30"
    cp /l4d2/left4dead2/cfg/server.cfg.30tick /l4d2/left4dead2/cfg/server.cfg
fi

# 获取环境变量中的RCON密码，写入到server.cfg
if [ -n "${L4D2_RCON_PASSWORD}" ]; then
    echo "设置RCON密码..."
    sed -i "s/^rcon_password .*/rcon_password \"${L4D2_RCON_PASSWORD}\"/" /l4d2/left4dead2/cfg/server.cfg
else
    echo "警告：未设置RCON密码，使用默认密码"
    sed -i "s/^rcon_password .*/rcon_password \"laoyutangnb!\"/" /l4d2/left4dead2/cfg/server.cfg
fi

# 设置服务器端口，环境变量L4D2_PORT
SERVER_PORT=${L4D2_PORT:-27015}
echo "设置服务器端口为: ${SERVER_PORT}"

echo "文件检查和初始化完成，启动服务器..."

# 启动L4D2服务器
cd /l4d2 && ./srcds_run -game left4dead2 -insecure -tickrate "${REAL_TICK}" -condebug +hostport "${SERVER_PORT}" +exec server.cfg
