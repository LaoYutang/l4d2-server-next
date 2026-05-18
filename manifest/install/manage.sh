#!/bin/bash
# ═══════════════════════════════════════════════════════════
#  L4D2 服务器管理工具
#  管理 Docker 环境、镜像拉取、多实例服务器部署
# ═══════════════════════════════════════════════════════════
set -o pipefail

# ─── 常量 ───
readonly SCRIPT_VERSION="1.0.0"
readonly BASE_DIR="/data/l4d2"
readonly CONFIG_DIR="${BASE_DIR}/.config"
readonly INSTANCE_DIR="${CONFIG_DIR}/instances"
readonly MIRROR_CONF="${CONFIG_DIR}/mirror.conf"
readonly COMPOSE_FILE="${BASE_DIR}/docker-compose.yaml"
readonly PROJECT_NAME="l4d2"
readonly IMAGE_GAME="laoyutang/l4d2-pure:latest"
readonly IMAGE_MANAGER="laoyutang/l4d2-manager-next:latest"
readonly DEFAULT_GAME_PORT=27015
readonly DEFAULT_MANAGER_PORT=27020

# ─── 颜色 ───
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly CYAN='\033[0;36m'
readonly BOLD='\033[1m'
readonly DIM='\033[2m'
readonly NC='\033[0m'

# ═══════════════════════════════════════════
#  UI 工具函数
# ═══════════════════════════════════════════

print_banner() {
    echo -e "${CYAN}"
    echo "╔══════════════════════════════════════════════════════╗"
    echo "║                                                      ║"
    echo "║         L4D2 服务器管理工具  v${SCRIPT_VERSION}                  ║"
    echo "║                                                      ║"
    echo "╚══════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

print_section() {
    local title="$1"
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BOLD}${CYAN}  $title${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# 计算字符串显示宽度（CJK 双宽字符计为 2，ASCII 计为 1）
# 公式：display_width = char_count + (byte_count - char_count) / 2
str_display_width() {
    local str="$1"
    local bytes chars
    bytes=$(printf '%s' "$str" | wc -c)
    chars=${#str}
    echo $(( chars + (bytes - chars) / 2 ))
}

print_menu_item() {
    local num="$1" text="$2" extra="${3:-}"
    if [[ -n "$extra" ]]; then
        local width pad
        width=$(str_display_width "$text")
        pad=$(( 24 - width ))
        [[ $pad -lt 1 ]] && pad=1
        printf "  ${CYAN}%s${NC}) %s%${pad}s ${DIM}%s${NC}\n" "$num" "$text" "" "$extra"
    else
        echo -e "  ${CYAN}${num}${NC}) ${text}"
    fi
}

print_success() { echo -e "  ${GREEN}✔ $1${NC}"; }
print_error()   { echo -e "  ${RED}✘ $1${NC}"; }
print_warn()    { echo -e "  ${YELLOW}⚠ $1${NC}"; }
print_info()    { echo -e "  ${BLUE}➜ $1${NC}"; }

# 确认操作 (返回 0=是, 1=否)
confirm() {
    local prompt="$1" default="${2:-n}"
    local yn="y/N"
    [[ "$default" == "y" ]] && yn="Y/n"
    read -r -p "$(echo -e "  ${YELLOW}? ${NC}${prompt} [${yn}]: ")" answer
    answer="${answer:-$default}"
    [[ "$answer" =~ ^[Yy]$ ]]
}

# 读取输入 (带默认值)
read_input() {
    local prompt="$1" default="$2" result
    if [[ -n "$default" ]]; then
        read -r -p "$(echo -e "  ${CYAN}▸ ${NC}${prompt} ${DIM}(默认: ${default})${NC}: ")" result
        echo "${result:-$default}"
    else
        read -r -p "$(echo -e "  ${CYAN}▸ ${NC}${prompt}: ")" result
        echo "$result"
    fi
}

# 读取密码 (带确认)
read_password() {
    local prompt="$1" pass1 pass2
    while true; do
        read -r -s -p "$(echo -e "  ${CYAN}▸ ${NC}${prompt}: ")" pass1
        echo >&2
        read -r -s -p "$(echo -e "  ${CYAN}▸ ${NC}请确认密码: ")" pass2
        echo >&2
        if [[ "$pass1" == "$pass2" ]]; then
            if [[ -z "$pass1" ]]; then
                print_error "密码不能为空" >&2
                continue
            fi
            printf '%s' "$pass1"
            return
        fi
        print_error "两次输入的密码不一致，请重新输入" >&2
    done
}

press_enter() {
    echo ""
    read -r -p "$(echo -e "  ${DIM}按 Enter 键继续...${NC}")"
}

# ═══════════════════════════════════════════
#  配置管理函数
# ═══════════════════════════════════════════

ensure_dirs() {
    mkdir -p "$CONFIG_DIR" "$INSTANCE_DIR"
}

# 索引 → 实例名 (0=l4d2, 1=l4d2-1, 2=l4d2-2 ...)
get_instance_name() {
    local index="$1"
    if [[ "$index" -eq 0 ]]; then
        echo "l4d2"
    else
        echo "l4d2-${index}"
    fi
}

# 列出所有实例名（按索引顺序：l4d2, l4d2-1, l4d2-2 ...）
list_instance_names() {
    [[ -d "$INSTANCE_DIR" ]] || return
    local index=0
    while true; do
        local name
        name=$(get_instance_name "$index")
        if [[ -f "${INSTANCE_DIR}/${name}.conf" ]]; then
            echo "$name"
        else
            # 跳过空缺索引，但连续 3 个不存在则停止
            local gap=0
            for ((g=1; g<=3; g++)); do
                local next_name
                next_name=$(get_instance_name $((index + g)))
                if [[ -f "${INSTANCE_DIR}/${next_name}.conf" ]]; then
                    gap=1
                    break
                fi
            done
            [[ "$gap" -eq 0 ]] && break
        fi
        ((index++))
    done
}

# 统计实例数量
count_instances() {
    local count=0
    if [[ -d "$INSTANCE_DIR" ]]; then
        for conf in "$INSTANCE_DIR"/*.conf; do
            [[ -f "$conf" ]] && ((count++))
        done
    fi
    echo "$count"
}

# 获取下一个可用索引
get_next_instance_index() {
    local index=0
    while [[ -f "${INSTANCE_DIR}/$(get_instance_name $index).conf" ]]; do
        ((index++))
    done
    echo "$index"
}

# 保存实例配置 (使用 printf %q 安全转义)
save_instance_config() {
    local name="$1"
    local conf="${INSTANCE_DIR}/${name}.conf"
    {
        printf 'GAME_PORT=%s\n' "$GAME_PORT"
        printf 'MANAGER_PORT=%s\n' "$MANAGER_PORT"
        printf 'ADMIN_PASSWORD=%q\n' "$ADMIN_PASSWORD"
        printf 'RCON_PASSWORD=%q\n' "$RCON_PASSWORD"
        printf 'TICK=%s\n' "$TICK"
        printf 'VAC=%s\n' "$VAC"
        printf 'HISTORY_METRICS=%s\n' "$HISTORY_METRICS"
        printf 'STEAM_API_KEY=%q\n' "$STEAM_API_KEY"
        printf 'BIND_MOUNT=%s\n' "${BIND_MOUNT:-false}"
        printf 'BIND_DATA_DIR=%q\n' "${BIND_DATA_DIR:-}"
        printf 'BIND_PLUGINS_DIR=%q\n' "${BIND_PLUGINS_DIR:-}"
    } > "$conf"
}

# 加载镜像加速源配置
load_mirror() {
    MIRROR_URL="docker.1ms.run"
    if [[ -f "$MIRROR_CONF" ]]; then
        # shellcheck source=/dev/null
        source "$MIRROR_CONF"
    fi
}

# 去除镜像源地址中的协议头
normalize_mirror_url() {
    local url="$1"
    url="${url#http://}"
    url="${url#https://}"
    echo "$url"
}

# 保存镜像加速源配置
save_mirror() {
    MIRROR_URL=$(normalize_mirror_url "$MIRROR_URL")
    printf 'MIRROR_URL=%q\n' "$MIRROR_URL" > "$MIRROR_CONF"
}

# 获取带加速源前缀的完整镜像名
get_image() {
    local image="$1"
    load_mirror
    local mirror_url
    mirror_url=$(normalize_mirror_url "$MIRROR_URL")
    if [[ -n "$mirror_url" ]]; then
        echo "${mirror_url}/${image}"
    else
        echo "$image"
    fi
}

# 检查端口是否已被其他实例使用
check_port_conflict() {
    local port="$1" skip_name="${2:-}"
    for conf in "$INSTANCE_DIR"/*.conf; do
        [[ -f "$conf" ]] || continue
        local conf_name
        conf_name=$(basename "$conf" .conf)
        [[ "$conf_name" == "$skip_name" ]] && continue
        local GAME_PORT="" MANAGER_PORT=""
        # shellcheck source=/dev/null
        source "$conf"
        if [[ "$GAME_PORT" == "$port" || "$MANAGER_PORT" == "$port" ]]; then
            echo "$conf_name"
            return 0
        fi
    done
    return 1
}

# ═══════════════════════════════════════════
#  Docker Compose 生成
# ═══════════════════════════════════════════

# 将任意字符串转义为 YAML 双引号字符串 (转义 \ 和 ")
# 生成 YAML 列表格式的环境变量行：      - KEY="value"
list_env() {
    local key="$1" val="$2"
    val="${val//\\/\\\\}"   # \ → \\
    val="${val//\"/\\\"}"   # " → \"
    printf '%s\n' "      - ${key}=${val}"
}

generate_compose() {
    local instances
    instances=$(list_instance_names)

    if [[ -z "$instances" ]]; then
        rm -f "$COMPOSE_FILE"
        return
    fi

    local game_image manager_image
    game_image=$(get_image "$IMAGE_GAME")
    manager_image=$(get_image "$IMAGE_MANAGER")

    # 逐行写入，使用 printf '%s\n' 避免 echo -e 解释字符串内部的转义序列
    {
        printf '%s\n' "volumes:"
        for name in $instances; do
            local BIND_MOUNT BIND_DATA_DIR BIND_PLUGINS_DIR
            # shellcheck source=/dev/null
            source "${INSTANCE_DIR}/${name}.conf"
            if [[ "${BIND_MOUNT:-false}" == "true" ]]; then
                printf '%s\n' "  ${name}-data:"
                printf '%s\n' "    driver: local"
                printf '%s\n' "    driver_opts:"
                printf '%s\n' "      type: none"
                printf '%s\n' "      o: bind"
                printf '%s\n' "      device: ${BIND_DATA_DIR}"
                printf '%s\n' "  ${name}-plugins:"
                printf '%s\n' "    driver: local"
                printf '%s\n' "    driver_opts:"
                printf '%s\n' "      type: none"
                printf '%s\n' "      o: bind"
                printf '%s\n' "      device: ${BIND_PLUGINS_DIR}"
                printf '%s\n' "  ${name}-manager-data:"
            else
                printf '%s\n' "  ${name}-data:"
                printf '%s\n' "  ${name}-plugins:"
                printf '%s\n' "  ${name}-manager-data:"
            fi
        done
        printf '\n'
        printf '%s\n' "networks:"
        printf '%s\n' "  l4d2-network:"
        printf '\n'
        printf '%s\n' "services:"

        for name in $instances; do
            local GAME_PORT MANAGER_PORT ADMIN_PASSWORD RCON_PASSWORD TICK VAC HISTORY_METRICS STEAM_API_KEY BIND_MOUNT BIND_DATA_DIR BIND_PLUGINS_DIR
            # shellcheck source=/dev/null
            source "${INSTANCE_DIR}/${name}.conf"

            printf '%s\n' "  ${name}:"
            printf '%s\n' "    image: ${game_image}"
            printf '%s\n' "    container_name: ${name}"
            printf '%s\n' "    restart: unless-stopped"
            printf '%s\n' "    ports:"
            printf '%s\n' "      - \"${GAME_PORT}:${GAME_PORT}\""
            printf '%s\n' "      - \"${GAME_PORT}:${GAME_PORT}/udp\""
            printf '%s\n' "    volumes:"
            printf '%s\n' "      - ${name}-data:/l4d2/left4dead2"
            printf '%s\n' "      - /etc/localtime:/etc/localtime:ro"
            printf '%s\n' "      - /etc/timezone:/etc/timezone:ro"
            printf '%s\n' "    networks:"
            printf '%s\n' "      - l4d2-network"
            printf '%s\n' "    security_opt:"
            printf '%s\n' "      - seccomp:unconfined # Docker 29.3+ 默认 seccomp 与 32 位 srcds 不兼容，低版本可省略此项"
            printf '%s\n' "    environment:"
            list_env "L4D2_TICK" "${TICK}"
            list_env "L4D2_VAC" "${VAC}"
            list_env "L4D2_PORT" "${GAME_PORT}"
            list_env "L4D2_RCON_PASSWORD" "${RCON_PASSWORD}"
            printf '%s\n' "    logging:"
            printf '%s\n' "      options:"
            printf '%s\n' "        max-size: \"50m\""
            printf '%s\n' "        max-file: \"3\""
            printf '\n'

            printf '%s\n' "  ${name}-manager:"
            printf '%s\n' "    image: ${manager_image}"
            printf '%s\n' "    container_name: ${name}-manager"
            printf '%s\n' "    restart: unless-stopped"
            printf '%s\n' "    ports:"
            printf '%s\n' "      - \"${MANAGER_PORT}:27020\""
            printf '%s\n' "    volumes:"
            printf '%s\n' "      - ${name}-data:/left4dead2"
            printf '%s\n' "      - ${name}-plugins:/plugins"
            printf '%s\n' "      - ${name}-manager-data:/data"
            printf '%s\n' "      - /var/run/docker.sock:/var/run/docker.sock"
            printf '%s\n' "      - /proc:/host/proc:ro"
            printf '%s\n' "      - /etc/localtime:/etc/localtime:ro"
            printf '%s\n' "      - /etc/timezone:/etc/timezone:ro"
            printf '%s\n' "    environment:"
            list_env "L4D2_RESTART_BY_RCON" "true"
            list_env "L4D2_MANAGER_PASSWORD" "${ADMIN_PASSWORD}"
            list_env "L4D2_RCON_PASSWORD" "${RCON_PASSWORD}"
            list_env "L4D2_RCON_URL" "${name}:${GAME_PORT}"
            list_env "L4D2_GAME_PATH" "/left4dead2"
            list_env "L4D2_HISTORY_METRICS" "${HISTORY_METRICS}"
            list_env "STEAM_API_KEY" "${STEAM_API_KEY}"
            printf '%s\n' "    networks:"
            printf '%s\n' "      - l4d2-network"
            printf '%s\n' "    logging:"
            printf '%s\n' "      options:"
            printf '%s\n' "        max-size: \"50m\""
            printf '%s\n' "        max-file: \"3\""
            printf '\n'
        done
    } > "$COMPOSE_FILE"

    print_success "docker-compose.yaml 已生成"
}

# ═══════════════════════════════════════════
#  Docker 管理函数
# ═══════════════════════════════════════════

check_docker() {
    command -v docker &>/dev/null
}

require_docker() {
    if ! check_docker; then
        print_error "Docker 未安装，请先在 [Docker 环境管理] 中安装"
        return 1
    fi
    return 0
}

install_docker() {
    if check_docker; then
        print_warn "Docker 已安装"
        docker --version
        return
    fi

    print_info "正在安装 Docker..."

    # 检测国内环境
    if curl -s --connect-timeout 5 ipinfo.io 2>/dev/null | grep -q '"country": "CN"'; then
        print_info "检测到国内环境，使用清华镜像源安装"
        export DOWNLOAD_URL="https://mirrors.tuna.tsinghua.edu.cn/docker-ce"
    fi

    local attempt
    for attempt in 1 2 3 4 5; do
        print_info "下载安装脚本 (第 ${attempt}/5 次)..."
        if curl -fsSL --connect-timeout 15 --retry 0 https://get.docker.com -o /tmp/get-docker.sh; then
            break
        fi
        if [[ $attempt -eq 5 ]]; then
            print_error "下载失败，已重试 5 次，请检查网络"
            return 1
        fi
        print_warn "下载失败，3 秒后重试..."
        sleep 3
    done
    if bash /tmp/get-docker.sh; then
        rm -f /tmp/get-docker.sh
        systemctl enable docker 2>/dev/null || true
        systemctl start docker 2>/dev/null || true
        print_success "Docker 安装完成"
        docker --version
    else
        rm -f /tmp/get-docker.sh
        print_error "Docker 安装失败，请检查网络或手动安装"
    fi
}

show_docker_status() {
    if ! check_docker; then
        print_error "Docker 未安装"
        return
    fi

    echo ""
    echo -e "  ${BOLD}Docker 版本${NC}"
    echo -n "    "; docker --version
    echo ""
    echo -e "  ${BOLD}Docker Compose 版本${NC}"
    echo -n "    "; docker compose version 2>/dev/null || print_error "Docker Compose 不可用"
    echo ""
    echo -e "  ${BOLD}服务状态${NC}"
    echo -n "    "; systemctl is-active docker 2>/dev/null || echo "无法获取 (非 systemd 环境)"
    echo ""
    echo -e "  ${BOLD}磁盘使用${NC}"
    docker system df 2>/dev/null | sed 's/^/    /'
}

# ═══════════════════════════════════════════
#  镜像管理函数
# ═══════════════════════════════════════════

pull_image() {
    local image="$1" display_name="$2"
    local full_image
    full_image=$(get_image "$image")

    # 如本地有相同镜像的其他 registry 标签，先静默迁移（不影响正常更新拉取）
    ensure_image_tag "$image" > /dev/null 2>&1 || true

    print_info "正在拉取 ${display_name}: ${full_image}"
    echo ""
    if docker pull "$full_image"; then
        echo ""
        print_success "${display_name} 拉取/更新成功"
    else
        echo ""
        print_error "${display_name} 拉取失败，请检查网络或镜像源配置"
        return 1
    fi
}

show_image_info() {
    local image="$1" display_name="$2"
    local full_image
    full_image=$(get_image "$image")

    echo -e "  ${BOLD}${display_name}${NC}"
    echo -e "    镜像名: ${full_image}"
    if docker image inspect "$full_image" &>/dev/null; then
        local created size
        created=$(docker image inspect "$full_image" --format '{{.Created}}' 2>/dev/null | cut -dT -f1)
        size=$(docker image inspect "$full_image" --format '{{.Size}}' 2>/dev/null)
        size=$((size / 1024 / 1024))
        echo -e "    创建日期: ${GREEN}${created}${NC}"
        echo -e "    大小: ${GREEN}${size} MB${NC}"
    else
        echo -e "    状态: ${DIM}未拉取${NC}"
    fi
}

# 确保本地存在指定 registry 的镜像标签；如不存在，尝试从本地其他 registry 标签迁移
ensure_image_tag() {
    local image="$1"
    local full_image
    full_image=$(get_image "$image")

    # 已存在则无需操作
    if docker image inspect "$full_image" &>/dev/null; then
        return 0
    fi

    # 查找本地已有的相同镜像（匹配任何 registry 前缀）
    local existing_image
    existing_image=$(docker images --format "{{.Repository}}:{{.Tag}}" | grep -E "(^|/)${image}$" | head -1)

    if [[ -n "$existing_image" && "$existing_image" != "$full_image" ]]; then
        print_info "迁移镜像标签: ${existing_image} -> ${full_image}"
        docker tag "$existing_image" "$full_image"
        return 0
    fi

    return 1
}

# 批量迁移游戏服务器和管理面板镜像标签
migrate_image_tags() {
    ensure_image_tag "$IMAGE_GAME"
    ensure_image_tag "$IMAGE_MANAGER"
}

# ═══════════════════════════════════════════
#  实例管理函数
# ═══════════════════════════════════════════

# 从 compose 文件中提取某个服务的块内容（从服务 key 行到下一个同级服务前）
_get_compose_service_block() {
    local file="$1" service="$2"
    awk -v key="  ${service}:" '
        $0 == key { in_block=1; next }
        in_block && /^  [a-zA-Z]/ { in_block=0 }
        in_block { print }
    ' "$file"
}

# 从服务块内容中提取某个环境变量的值（自动去除行内注释和引号）
_get_compose_env() {
    local block="$1" varname="$2"
    echo "$block" | grep "${varname}=" \
        | sed "s/.*${varname}=//" \
        | sed 's/[[:space:]][[:space:]]*#.*//' \
        | tr -d '"' | tr -d "'" \
        | xargs 2>/dev/null \
        | head -1
}

# 尝试导入已有的 docker-compose.yaml 配置（支持多实例）
try_import_existing() {
    # 已有配置目录，无需导入
    if [[ -d "$INSTANCE_DIR" ]] && [[ "$(count_instances)" -gt 0 ]]; then
        return
    fi

    # 没有旧的 compose 文件，无需导入
    [[ -f "$COMPOSE_FILE" ]] || return

    print_info "检测到已有的 docker-compose.yaml，正在导入配置..."
    ensure_dirs

    # 检测镜像加速源（一次性全局检测）
    local image_line
    image_line=$(grep -m1 'l4d2-pure' "$COMPOSE_FILE" 2>/dev/null | sed 's/.*image: *//' | tr -d '"'"'" || echo "")
    if [[ "$image_line" == *"/"*"/"*"/"* ]]; then
        local mirror
        mirror=$(echo "$image_line" | sed "s|/laoyutang/.*||")
        mirror=$(normalize_mirror_url "$mirror")
        if [[ -n "$mirror" ]]; then
            MIRROR_URL="$mirror"
            save_mirror
            print_info "检测到镜像加速源: ${mirror}"
        fi
    fi

    # 发现所有游戏实例服务名（匹配 "  l4d2:" 或 "  l4d2-N:" 格式，排除 -manager）
    local instance_names
    instance_names=$(grep -E '^  l4d2(-[0-9]+)?:' "$COMPOSE_FILE" 2>/dev/null | sed -E 's/^  (.*):$/\1/')

    if [[ -z "$instance_names" ]]; then
        print_warn "未能从 docker-compose.yaml 中识别到实例，跳过导入"
        return
    fi

    local imported=0
    for name in $instance_names; do
        local game_block manager_block
        game_block=$(_get_compose_service_block "$COMPOSE_FILE" "$name")
        manager_block=$(_get_compose_service_block "$COMPOSE_FILE" "${name}-manager")

        GAME_PORT=$(_get_compose_env "$game_block" "L4D2_PORT")
        GAME_PORT="${GAME_PORT:-27015}"

        TICK=$(_get_compose_env "$game_block" "L4D2_TICK")
        TICK="${TICK:-100}"

        VAC=$(_get_compose_env "$game_block" "L4D2_VAC")
        VAC="${VAC:-false}"

        RCON_PASSWORD=$(_get_compose_env "$game_block" "L4D2_RCON_PASSWORD")

        ADMIN_PASSWORD=$(_get_compose_env "$manager_block" "L4D2_MANAGER_PASSWORD")

        HISTORY_METRICS=$(_get_compose_env "$manager_block" "L4D2_HISTORY_METRICS")
        HISTORY_METRICS="${HISTORY_METRICS:-false}"

        STEAM_API_KEY=$(_get_compose_env "$manager_block" "STEAM_API_KEY")

        # 面板端口：在 manager 服务块的 ports 中找 HOST_PORT:27020
        MANAGER_PORT=$(echo "$manager_block" | grep -oE '[0-9]+:27020' | cut -d: -f1 | head -1)
        MANAGER_PORT="${MANAGER_PORT:-27020}"

        save_instance_config "$name"
        print_success "已导入实例 [${name}]: 游戏端口 ${GAME_PORT}，面板端口 ${MANAGER_PORT}"

        # 迁移旧版软链接：删除旧的 addons/cfg 链接，建立 left4dead2 链接
        if [[ "$name" == "l4d2" ]]; then
            local old_addons="${BASE_DIR}/addons"
            local old_cfg="${BASE_DIR}/cfg"
            if [[ -L "$old_addons" ]] || [[ -L "$old_cfg" ]]; then
                rm -f "$old_addons" "$old_cfg"
                print_info "已移除旧版 addons/cfg 软链接"
            fi
            create_symlinks "$name"
            print_info "已创建 left4dead2 软链接"
        fi

        ((imported++))
    done

    if [[ $imported -gt 0 ]]; then
        print_success "共导入 ${imported} 个实例配置"
        generate_compose
    fi
}

# 一键安装启动（首次快捷入口）
quick_setup() {
    clear
    print_banner
    print_section "一键安装启动"
    echo ""

    # ── Step 1: 安装 Docker ──
    if ! check_docker; then
        print_info "Step 1/3  安装 Docker"
        echo ""
        install_docker
        echo ""
        if ! check_docker; then
            print_error "Docker 安装失败，无法继续"
            press_enter
            return
        fi
    else
        print_success "Step 1/3  Docker 已安装，跳过"
        echo ""
    fi

    # ── Step 2: 配置镜像加速源 ──
    load_mirror
    print_info "Step 2/3  配置镜像加速源"
    echo ""
    if [[ -n "$MIRROR_URL" ]]; then
        echo -e "  当前加速源: ${GREEN}${MIRROR_URL}${NC}"
        if ! confirm "是否修改加速源" "n"; then
            true  # 保持不变
        else
            local new_mirror
            new_mirror=$(read_input "输入加速源地址 (留空使用官方源)" "$MIRROR_URL")
            MIRROR_URL="$new_mirror"
            save_mirror
        fi
    else
        echo -e "  默认加速源: ${GREEN}${DEFAULT_MIRROR}${NC}"
        local new_mirror
        new_mirror=$(read_input "镜像加速源 (回车使用默认)" "$DEFAULT_MIRROR")
        MIRROR_URL="${new_mirror:-$DEFAULT_MIRROR}"
        save_mirror
    fi
    echo ""

    # ── Step 3: 配置并启动首个实例 ──
    print_info "Step 3/3  配置服务实例"
    echo ""

    # 管理密码
    ADMIN_PASSWORD=$(read_password "请设置管理密码")
    echo ""

    # 游戏端口
    GAME_PORT=$(read_input "游戏端口" "$DEFAULT_GAME_PORT")

    # 管理面板端口
    MANAGER_PORT=$(read_input "管理面板端口" "$DEFAULT_MANAGER_PORT")

    # Tick 率
    echo ""
    echo -e "  ${BOLD}Tick 率:${NC}"
    print_menu_item "1" "30  tick"
    print_menu_item "2" "60  tick"
    print_menu_item "3" "100 tick" "推荐"
    print_menu_item "4" "128 tick"
    local tick_choice
    tick_choice=$(read_input "选择 Tick 率" "3")
    case "$tick_choice" in
        1) TICK=30 ;;
        2) TICK=60 ;;
        4) TICK=128 ;;
        *) TICK=100 ;;
    esac

    VAC="false"
    HISTORY_METRICS="false"
    STEAM_API_KEY=""
    RCON_PASSWORD=$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 16)

    # 确认
    local name="l4d2"
    echo ""
    print_section "配置确认"
    echo ""
    echo -e "  实例名称:      ${BOLD}${name}${NC}"
    echo -e "  游戏端口:      ${BOLD}${GAME_PORT}${NC}"
    echo -e "  管理面板端口:  ${BOLD}${MANAGER_PORT}${NC}"
    echo -e "  Tick 率:       ${BOLD}${TICK}${NC}"
    echo -e "  RCON 密码:     ${BOLD}${RCON_PASSWORD}${NC} ${DIM}(自动生成)${NC}"
    echo ""

    if ! confirm "确认并开始安装" "y"; then
        print_warn "已取消"
        press_enter
        return
    fi

    save_instance_config "$name"
    generate_compose

    echo ""
    cd "$BASE_DIR" || return
    print_info "正在拉取镜像并启动服务 (首次启动需拉取镜像，请耐心等待)..."
    echo ""
    if docker compose up -d; then
        create_symlinks "$name"
        echo ""
        print_success "安装完成！"
        echo ""
        echo -e "  ${GREEN}游戏地址:${NC}  服务器IP:${BOLD}${GAME_PORT}${NC}"
        echo -e "  ${GREEN}管理面板:${NC}  http://服务器IP:${BOLD}${MANAGER_PORT}${NC}"
    else
        echo ""
        print_error "启动失败，请检查上方错误信息"
        print_warn "配置已保存，修复问题后可在 [服务实例管理] 中重试"
    fi
    press_enter
}

# 添加新实例
add_instance() {
    require_docker || return

    local index
    index=$(get_next_instance_index)
    local name
    name=$(get_instance_name "$index")

    # 计算建议端口
    local suggest_game=$DEFAULT_GAME_PORT
    local suggest_manager=$DEFAULT_MANAGER_PORT
    if [[ "$index" -gt 0 ]]; then
        suggest_game=$((DEFAULT_GAME_PORT + index * 10))
        suggest_manager=$((DEFAULT_MANAGER_PORT + index * 10))
    fi

    print_section "添加新服务实例 [${name}]"
    echo ""

    # ── 1. 管理密码 ──
    ADMIN_PASSWORD=$(read_password "请设置管理密码")
    echo ""

    # ── 2. 端口 ──
    while true; do
        GAME_PORT=$(read_input "游戏端口" "$suggest_game")
        local conflict
        if conflict=$(check_port_conflict "$GAME_PORT"); then
            print_error "端口 ${GAME_PORT} 已被实例 [${conflict}] 使用"
            continue
        fi
        break
    done

    while true; do
        MANAGER_PORT=$(read_input "管理面板端口" "$suggest_manager")
        local conflict
        if conflict=$(check_port_conflict "$MANAGER_PORT"); then
            print_error "端口 ${MANAGER_PORT} 已被实例 [${conflict}] 使用"
            continue
        fi
        if [[ "$MANAGER_PORT" == "$GAME_PORT" ]]; then
            print_error "面板端口不能与游戏端口相同"
            continue
        fi
        break
    done

    # ── 3. Tick 率 ──
    echo ""
    echo -e "  ${BOLD}Tick 率设置:${NC}"
    print_menu_item "1" "30  tick"
    print_menu_item "2" "60  tick"
    print_menu_item "3" "100 tick" "推荐"
    print_menu_item "4" "128 tick"
    local tick_choice
    tick_choice=$(read_input "选择 Tick 率" "3")
    case "$tick_choice" in
        1) TICK=30 ;;
        2) TICK=60 ;;
        4) TICK=128 ;;
        *) TICK=100 ;;
    esac

    # ── 4. VAC ──
    echo ""
    if confirm "是否开启 VAC 验证" "n"; then
        VAC="true"
    else
        VAC="false"
    fi

    # ── 5. 性能监控 ──
    if confirm "是否开启历史性能监控 (需持久化数据)" "n"; then
        HISTORY_METRICS="true"
    else
        HISTORY_METRICS="false"
    fi

    # ── 6. Steam API Key ──
    echo ""
    echo -e "  ${DIM}Steam API Key 用于查询玩家游戏时长${NC}"
    echo -e "  ${DIM}获取地址: https://steamcommunity.com/dev/apikey${NC}"
    STEAM_API_KEY=$(read_input "Steam API Key (可选，回车跳过)" "")

    # ── 7. 具名卷 bind 模式 ──
    echo ""
    echo -e "  ${YELLOW}⚠  高级选项：Bind 挂载模式${NC}"
    echo -e "  ${DIM}用途：将游戏/插件数据目录挂载到其他数据盘（如 /mnt/data）或多个实例绑定相同目录${NC}"
    echo -e "  ${DIM}注意：目录必须提前存在且权限正确，配置后不可更改。实例删除后，绑定路径不会自动删除${NC}"
    echo -e "  ${DIM}如无特殊需求（如独立数据盘），请直接回车跳过${NC}"
    BIND_MOUNT="false"
    BIND_DATA_DIR=""
    BIND_PLUGINS_DIR=""
    if confirm "是否启用 bind 挂载（不了解请选 N）" "n"; then
        BIND_MOUNT="true"
        local default_data="/data/l4d2/${name}-data"
        local default_plugins="/data/l4d2/${name}-plugins"
        BIND_DATA_DIR=$(read_input "游戏数据目录 (将自动创建)" "$default_data")
        BIND_PLUGINS_DIR=$(read_input "插件数据目录 (将自动创建)" "$default_plugins")
        mkdir -p "$BIND_DATA_DIR" "$BIND_PLUGINS_DIR" 2>/dev/null || true
    fi

    # ── 8. 自动生成 RCON 密码 ──
    RCON_PASSWORD=$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 16)

    # ── 9. 确认 ──
    print_section "配置确认 [${name}]"
    echo ""
    echo -e "  实例名称:      ${BOLD}${name}${NC}"
    echo -e "  游戏端口:      ${BOLD}${GAME_PORT}${NC}"
    echo -e "  管理面板端口:  ${BOLD}${MANAGER_PORT}${NC}"
    echo -e "  Tick 率:       ${BOLD}${TICK}${NC}"
    echo -e "  VAC 验证:      ${BOLD}${VAC}${NC}"
    echo -e "  历史性能监控:  ${BOLD}${HISTORY_METRICS}${NC}"
    echo -e "  RCON 密码:     ${BOLD}${RCON_PASSWORD}${NC} ${DIM}(自动生成)${NC}"
    if [[ "$BIND_MOUNT" == "true" ]]; then
        echo -e "  Bind 数据目录: ${BOLD}${BIND_DATA_DIR}${NC}"
        echo -e "  Bind 插件目录: ${BOLD}${BIND_PLUGINS_DIR}${NC}"
    fi
    echo ""

    if ! confirm "确认创建此实例" "y"; then
        print_warn "已取消"
        return
    fi

    # ── 保存配置 ──
    save_instance_config "$name"
    print_success "实例配置已保存"
    generate_compose

    # ── 部署 ──
    print_section "部署实例 [${name}]"
    echo ""
    cd "$BASE_DIR" || return
    print_info "正在启动服务 (首次启动需拉取镜像，请耐心等待)..."
    echo ""
    if docker compose up -d; then
        create_symlinks "$name"
        echo ""
        print_success "实例 [${name}] 已创建并启动"
        echo ""
        echo -e "  ${GREEN}游戏地址:${NC}  服务器IP:${BOLD}${GAME_PORT}${NC}"
        echo -e "  ${GREEN}管理面板:${NC}  http://服务器IP:${BOLD}${MANAGER_PORT}${NC}"
    else
        echo ""
        print_error "启动失败，请检查上方错误信息"
        print_warn "配置已保存，修复问题后可重新启动"
    fi
}

# 创建便捷软链接 (指向 left4dead2 整个目录)
create_symlinks() {
    local name="$1"
    local volume_prefix="${PROJECT_NAME}_${name}"

    if [[ "$name" == "l4d2" ]]; then
        # 第一个实例：链接到 /data/l4d2/ 根目录 (兼容旧版)
        ln -sf "/var/lib/docker/volumes/${volume_prefix}-data/_data" "${BASE_DIR}/left4dead2" 2>/dev/null || true
    else
        # 后续实例：链接名为 left4dead2-N（与实例名 l4d2-N 对应）
        local link_name
        link_name="left4dead2-${name#l4d2-}"
        ln -sf "/var/lib/docker/volumes/${volume_prefix}-data/_data" "${BASE_DIR}/${link_name}" 2>/dev/null || true
    fi
}

# 选择实例 (通用选择器，返回实例名；所有展示输出到 stderr，仅实例名输出到 stdout)
select_instance() {
    local prompt="${1:-选择实例}"
    local instances
    instances=$(list_instance_names)

    if [[ -z "$instances" ]]; then
        print_warn "当前没有任何服务实例" >&2
        return 1
    fi

    echo "" >&2
    local i=1
    local instance_array=()
    for name in $instances; do
        local GAME_PORT="" MANAGER_PORT=""
        # shellcheck source=/dev/null
        source "${INSTANCE_DIR}/${name}.conf"
        print_menu_item "$i" "[${name}]" "游戏:${GAME_PORT} 面板:${MANAGER_PORT}" >&2
        instance_array+=("$name")
        ((i++))
    done
    print_menu_item "0" "返回" >&2
    echo "" >&2

    local choice
    read -r -p "$(echo -e "  ${CYAN}▸ ${NC}${prompt}: ")" choice >&2

    if [[ "$choice" == "0" || -z "$choice" ]]; then
        return 1
    fi

    local idx=$((choice - 1))
    if [[ $idx -lt 0 || $idx -ge ${#instance_array[@]} ]]; then
        print_error "无效的选择" >&2
        return 1
    fi

    printf '%s' "${instance_array[$idx]}"
}

# 修改实例配置
# 修改实例 - 游戏服务器子菜单
_edit_game_server() {
    local target="$1"
    # 使用 nameref 引用外层变量
    while true; do
        clear
        print_banner
        print_section "修改游戏服务器设置 [${target}]"
        echo ""
        echo -e "  ${CYAN}当前配置:${NC}"
        echo -e "    游戏端口:  ${BOLD}${GAME_PORT}${NC}"
        echo -e "    Tick 率:   ${BOLD}${TICK}${NC}"
        echo -e "    VAC 验证:  ${BOLD}${VAC}${NC}"
        echo ""
        print_menu_item "1" "修改游戏端口"        "当前: ${GAME_PORT}"
        print_menu_item "2" "修改 Tick 率"        "当前: ${TICK}"
        print_menu_item "3" "修改 VAC 验证"       "当前: ${VAC}"
        echo ""
        print_menu_item "0" "返回"
        echo ""

        local choice
        choice=$(read_input "请选择" "0")

        case "$choice" in
            1)
                echo ""
                echo -e "  ${DIM}当前游戏端口: ${BOLD}${GAME_PORT}${NC}"
                while true; do
                    local input
                    input=$(read_input "输入新端口" "$GAME_PORT")
                    if [[ "$input" == "$GAME_PORT" ]]; then
                        break
                    fi
                    local conflict
                    if conflict=$(check_port_conflict "$input" "$target"); then
                        print_error "端口 ${input} 已被实例 [${conflict}] 使用"
                        continue
                    fi
                    GAME_PORT="$input"
                    print_success "游戏端口已设为 ${GAME_PORT}"
                    break
                done
                press_enter ;;
            2)
                echo ""
                echo -e "  ${DIM}当前 Tick 率: ${BOLD}${TICK}${NC}"
                echo ""
                print_menu_item "1" "30  tick"
                print_menu_item "2" "60  tick"
                print_menu_item "3" "100 tick" "推荐"
                print_menu_item "4" "128 tick"
                print_menu_item "0" "不修改"
                echo ""
                local tc
                tc=$(read_input "选择" "0")
                case "$tc" in
                    1) TICK=30;  print_success "Tick 率已设为 30" ;;
                    2) TICK=60;  print_success "Tick 率已设为 60" ;;
                    3) TICK=100; print_success "Tick 率已设为 100" ;;
                    4) TICK=128; print_success "Tick 率已设为 128" ;;
                esac
                press_enter ;;
            3)
                echo ""
                echo -e "  ${DIM}当前 VAC 验证: ${BOLD}${VAC}${NC}"
                echo ""
                if confirm "是否开启 VAC 验证" "$([[ "$VAC" == "true" ]] && echo y || echo n)"; then
                    VAC="true"
                else
                    VAC="false"
                fi
                print_success "VAC 验证已设为 ${VAC}"
                press_enter ;;
            0) return ;;
            *) print_error "无效选择"; press_enter ;;
        esac
    done
}

# 修改实例 - 管理面板子菜单
_edit_manager() {
    local target="$1"
    while true; do
        clear
        print_banner
        print_section "修改管理面板设置 [${target}]"
        echo ""
        echo -e "  ${CYAN}当前配置:${NC}"
        echo -e "    面板端口:      ${BOLD}${MANAGER_PORT}${NC}"
        echo -e "    历史性能监控:  ${BOLD}${HISTORY_METRICS}${NC}"
        echo -e "    Steam API Key: ${BOLD}${STEAM_API_KEY:-(未设置)}${NC}"
        echo ""
        print_menu_item "1" "修改面板端口"        "当前: ${MANAGER_PORT}"
        print_menu_item "2" "修改管理密码"        "当前已设置"
        print_menu_item "3" "修改历史性能监控"    "当前: ${HISTORY_METRICS}"
        print_menu_item "4" "修改 Steam API Key"  "当前: ${STEAM_API_KEY:-(未设置)}"
        echo ""
        print_menu_item "0" "返回"
        echo ""

        local choice
        choice=$(read_input "请选择" "0")

        case "$choice" in
            1)
                echo ""
                echo -e "  ${DIM}当前面板端口: ${BOLD}${MANAGER_PORT}${NC}"
                while true; do
                    local input
                    input=$(read_input "输入新端口" "$MANAGER_PORT")
                    if [[ "$input" == "$MANAGER_PORT" ]]; then
                        break
                    fi
                    local conflict
                    if conflict=$(check_port_conflict "$input" "$target"); then
                        print_error "端口 ${input} 已被实例 [${conflict}] 使用"
                        continue
                    fi
                    if [[ "$input" == "$GAME_PORT" ]]; then
                        print_error "面板端口不能与游戏端口 ${GAME_PORT} 相同"
                        continue
                    fi
                    MANAGER_PORT="$input"
                    print_success "面板端口已设为 ${MANAGER_PORT}"
                    break
                done
                press_enter ;;
            2)
                echo ""
                local p1 p2
                read -r -s -p "$(echo -e "  ${CYAN}▸ ${NC}输入新密码: ")" p1; echo >&2
                if [[ -z "$p1" ]]; then
                    print_warn "密码为空，保持不变"
                else
                    read -r -s -p "$(echo -e "  ${CYAN}▸ ${NC}确认新密码: ")" p2; echo >&2
                    if [[ "$p1" != "$p2" ]]; then
                        print_error "两次输入不一致，密码保持不变"
                    else
                        ADMIN_PASSWORD="$p1"
                        print_success "管理密码已更新"
                    fi
                fi
                press_enter ;;
            3)
                echo ""
                echo -e "  ${DIM}当前历史性能监控: ${BOLD}${HISTORY_METRICS}${NC}"
                echo ""
                if confirm "是否开启历史性能监控" "$([[ "$HISTORY_METRICS" == "true" ]] && echo y || echo n)"; then
                    HISTORY_METRICS="true"
                else
                    HISTORY_METRICS="false"
                fi
                print_success "历史性能监控已设为 ${HISTORY_METRICS}"
                press_enter ;;
            4)
                echo ""
                echo -e "  ${DIM}当前 Steam API Key: ${BOLD}${STEAM_API_KEY:-(未设置)}${NC}"
                echo -e "  ${DIM}获取地址: https://steamcommunity.com/dev/apikey${NC}"
                local new_key
                new_key=$(read_input "输入新值 (回车清空)" "")
                STEAM_API_KEY="$new_key"
                if [[ -n "$STEAM_API_KEY" ]]; then
                    print_success "Steam API Key 已更新"
                else
                    print_success "Steam API Key 已清空"
                fi
                press_enter ;;
            0) return ;;
            *) print_error "无效选择"; press_enter ;;
        esac
    done
}

# 修改实例配置入口
edit_instance() {
    require_docker || return

    print_section "修改服务实例配置"

    local target
    target=$(select_instance "选择要修改的实例") || return

    # 加载当前配置到局部变量
    local GAME_PORT MANAGER_PORT ADMIN_PASSWORD RCON_PASSWORD TICK VAC HISTORY_METRICS STEAM_API_KEY
    # shellcheck source=/dev/null
    source "${INSTANCE_DIR}/${target}.conf"

    while true; do
        clear
        print_banner
        print_section "修改实例 [${target}]"
        echo ""
        echo -e "  ${CYAN}当前概览:${NC}"
        echo -e "    游戏端口: ${BOLD}${GAME_PORT}${NC}   Tick: ${BOLD}${TICK}${NC}   VAC: ${BOLD}${VAC}${NC}"
        echo -e "    面板端口: ${BOLD}${MANAGER_PORT}${NC}   监控: ${BOLD}${HISTORY_METRICS}${NC}"
        echo ""
        print_menu_item "1" "游戏服务器设置"   "端口 / Tick / VAC"
        print_menu_item "2" "管理面板设置"     "端口 / 密码 / 监控 / Steam Key"
        echo ""
        print_menu_item "s" "保存并重启实例"
        print_menu_item "0" "放弃并返回"
        echo ""

        local choice
        choice=$(read_input "请选择" "0")

        case "$choice" in
            1) _edit_game_server "$target" ;;
            2) _edit_manager "$target" ;;
            s|S)
                if ! confirm "确认保存配置并重启实例 [${target}]" "y"; then
                    print_warn "已取消，继续编辑"
                    press_enter
                    continue
                fi
                save_instance_config "$target"
                print_success "配置已保存"
                generate_compose
                cd "$BASE_DIR" || return
                print_info "正在重启实例 [${target}]..."
                echo ""
                if docker compose up -d; then
                    echo ""
                    print_success "实例 [${target}] 已更新并重启"
                else
                    echo ""
                    print_error "重启失败，请检查上方错误信息"
                fi
                press_enter
                return ;;
            0) print_warn "已放弃修改"; return ;;
            *) print_error "无效选择"; press_enter ;;
        esac
    done
}

# 删除实例
remove_instance() {
    require_docker || return

    print_section "删除服务实例"

    local target
    target=$(select_instance "选择要删除的实例") || return

    echo ""
    print_warn "即将删除实例 [${target}] 及其所有游戏数据！"
    print_warn "此操作不可恢复！"
    echo ""

    if ! confirm "确认删除实例 [${target}]" "n"; then
        print_warn "已取消"
        return
    fi

    cd "$BASE_DIR" || return

    # 先直接停止并删除目标容器（不依赖 compose 文件，避免顺序问题）
    print_info "正在停止并删除容器..."
    docker stop "${target}" "${target}-manager" 2>/dev/null || true
    docker rm -f "${target}" "${target}-manager" 2>/dev/null || true

    # 删除配置并重新生成 compose
    rm -f "${INSTANCE_DIR}/${target}.conf"
    generate_compose

    # 删除 volume（容器已清理，volume 可安全删除）
    local volume_prefix="${PROJECT_NAME}_${target}"
    print_info "正在删除数据卷..."
    docker volume rm "${volume_prefix}-data" 2>/dev/null || true
    docker volume rm "${volume_prefix}-plugins" 2>/dev/null || true
    docker volume rm "${volume_prefix}-manager-data" 2>/dev/null || true

    # 删除软链接
    if [[ "$target" == "l4d2" ]]; then
        rm -f "${BASE_DIR}/left4dead2"
    else
        rm -f "${BASE_DIR}/left4dead2-${target#l4d2-}"
    fi

    print_success "实例 [${target}] 已删除"
}

# 显示所有实例状态
show_instances() {
    local instances
    instances=$(list_instance_names)

    if [[ -z "$instances" ]]; then
        print_warn "当前没有任何服务实例"
        return
    fi

    print_section "服务实例列表"
    echo ""

    local i=1
    for name in $instances; do
        local GAME_PORT="" MANAGER_PORT="" TICK="" VAC=""
        # shellcheck source=/dev/null
        source "${INSTANCE_DIR}/${name}.conf"

        # 容器状态
        local game_status manager_status
        game_status=$(docker inspect -f '{{.State.Status}}' "$name" 2>/dev/null || echo "未创建")
        manager_status=$(docker inspect -f '{{.State.Status}}' "${name}-manager" 2>/dev/null || echo "未创建")

        local game_color="${RED}" manager_color="${RED}"
        [[ "$game_status" == "running" ]] && game_color="${GREEN}"
        [[ "$manager_status" == "running" ]] && manager_color="${GREEN}"

        echo -e "  ${BOLD}${CYAN}[${i}] ${name}${NC}"
        echo -e "      游戏服务器: ${game_color}${game_status}${NC}    端口: ${GAME_PORT}"
        echo -e "      管理面板:   ${manager_color}${manager_status}${NC}    端口: ${MANAGER_PORT}"
        echo -e "      Tick: ${TICK}  |  VAC: ${VAC}"
        echo ""
        ((i++))
    done
}

# ═══════════════════════════════════════════
#  服务控制函数 (启动/停止/重启)
# ═══════════════════════════════════════════

restart_instance() {
    require_docker || return

    if [[ ! -f "$COMPOSE_FILE" ]]; then
        print_error "未找到 docker-compose.yaml，请先添加服务实例"
        return
    fi

    print_section "重启服务实例"

    local target
    target=$(select_instance "选择要重启的实例") || return

    cd "$BASE_DIR" || return
    print_info "正在重启 [${target}]..."
    docker compose restart "${target}" "${target}-manager"

    echo ""
    print_success "[${target}] 重启完成"
}

# ═══════════════════════════════════════════
#  菜单函数
# ═══════════════════════════════════════════

menu_docker() {
    while true; do
        clear
        print_banner
        print_section "Docker 环境管理"
        echo ""

        if check_docker; then
            echo -e "  状态: ${GREEN}● 已安装${NC}"
            local ver
            ver=$(docker --version 2>/dev/null | sed 's/Docker version //' || echo "未知")
            echo -e "  版本: ${ver}"
        else
            echo -e "  状态: ${RED}● 未安装${NC}"
        fi
        echo ""

        print_menu_item "1" "查看 Docker 详细信息"
        print_menu_item "2" "安装 Docker"
        echo ""
        print_menu_item "0" "返回主菜单"
        echo ""

        local choice
        choice=$(read_input "请选择" "0")

        case "$choice" in
            1) show_docker_status; press_enter ;;
            2) install_docker; press_enter ;;
            0) return ;;
            *) print_error "无效选择"; press_enter ;;
        esac
    done
}

menu_images() {
    while true; do
        clear
        print_banner
        print_section "镜像管理"
        echo ""

        load_mirror
        if [[ -n "$MIRROR_URL" ]]; then
            echo -e "  加速源: ${GREEN}${MIRROR_URL}${NC}"
        else
            echo -e "  加速源: ${DIM}未设置 (Docker Hub 官方源)${NC}"
        fi
        echo ""

        print_menu_item "1" "拉取/更新游戏服务器镜像" "${IMAGE_GAME}"
        print_menu_item "2" "拉取/更新管理面板镜像" "${IMAGE_MANAGER}"
        print_menu_item "3" "拉取/更新全部镜像"
        print_menu_item "4" "设置镜像加速源"
        print_menu_item "5" "查看本地镜像信息"
        print_menu_item "6" "重建所有实例 (应用镜像更新)"
        echo ""
        print_menu_item "0" "返回主菜单"
        echo ""

        local choice
        choice=$(read_input "请选择" "0")

        case "$choice" in
            1)
                require_docker && pull_image "$IMAGE_GAME" "游戏服务器"
                press_enter ;;
            2)
                require_docker && pull_image "$IMAGE_MANAGER" "管理面板"
                press_enter ;;
            3)
                if require_docker; then
                    pull_image "$IMAGE_GAME" "游戏服务器"
                    pull_image "$IMAGE_MANAGER" "管理面板"
                fi
                press_enter ;;
            4)
                echo ""
                local new_mirror
                new_mirror=$(read_input "输入加速源地址 (留空清除当前设置)" "")
                MIRROR_URL="$new_mirror"
                save_mirror
                if [[ -n "$MIRROR_URL" ]]; then
                    print_success "加速源已设置为: ${MIRROR_URL}"
                else
                    print_success "加速源已清除"
                fi
                # 迁移已有镜像标签到新 registry
                if require_docker; then
                    migrate_image_tags
                fi
                # 更新 compose 文件中的镜像地址
                if [[ "$(count_instances)" -gt 0 ]]; then
                    echo ""
                    if confirm "是否同步更新 docker-compose.yaml 中的镜像地址" "y"; then
                        generate_compose
                    fi
                fi
                press_enter ;;
            5)
                if require_docker; then
                    echo ""
                    show_image_info "$IMAGE_GAME" "游戏服务器"
                    echo ""
                    show_image_info "$IMAGE_MANAGER" "管理面板"
                fi
                press_enter ;;
            6)
                if require_docker; then
                    if [[ "$(count_instances)" -eq 0 ]]; then
                        print_warn "当前没有任何服务实例"
                    else
                        cd "$BASE_DIR" || return
                        print_info "正在重建所有实例..."
                        echo ""
                        if docker compose up -d; then
                            echo ""
                            print_success "所有实例已重建并启动"
                            echo ""
                            print_info "正在清理 dangling 镜像..."
                            if docker image prune -f > /dev/null 2>&1; then
                                print_success "dangling 镜像清理完成"
                            else
                                print_warn "dangling 镜像清理失败，可手动执行: docker image prune -f"
                            fi
                        else
                            echo ""
                            print_error "重建失败，请检查上方错误信息"
                        fi
                    fi
                fi
                press_enter ;;
            0) return ;;
            *) print_error "无效选择"; press_enter ;;
        esac
    done
}

menu_instances() {
    while true; do
        clear
        print_banner
        print_section "服务实例管理"
        echo ""

        local count
        count=$(count_instances)
        echo -e "  当前实例数: ${BOLD}${count}${NC}"
        echo ""

        print_menu_item "1" "添加新服务实例"
        print_menu_item "2" "修改服务实例配置"
        print_menu_item "3" "删除服务实例"
        print_menu_item "4" "查看所有实例状态"
        print_menu_item "5" "重启指定实例"
        echo ""
        print_menu_item "0" "返回主菜单"
        echo ""

        local choice
        choice=$(read_input "请选择" "0")

        case "$choice" in
            1) add_instance; press_enter ;;
            2) edit_instance; press_enter ;;
            3) remove_instance; press_enter ;;
            4) show_instances; press_enter ;;
            5) restart_instance; press_enter ;;
            0) return ;;
            *) print_error "无效选择"; press_enter ;;
        esac
    done
}

# ═══════════════════════════════════════════
#  主菜单
# ═══════════════════════════════════════════

main() {
    # 初始化
    ensure_dirs
    try_import_existing

    while true; do
        clear
        print_banner

        # 快速状态概览
        if check_docker; then
            local ver
            ver=$(docker --version 2>/dev/null | sed 's/Docker version /v/' | cut -d, -f1 || echo "")
            echo -e "  Docker: ${GREEN}● 已安装${NC} ${DIM}${ver}${NC}    实例: ${BOLD}$(count_instances)${NC}"
        else
            echo -e "  Docker: ${RED}● 未安装${NC}"
        fi

        load_mirror
        if [[ -n "$MIRROR_URL" ]]; then
            echo -e "  加速源: ${GREEN}${MIRROR_URL}${NC}"
        fi
        echo ""

        print_menu_item "1" "Docker 环境管理"     "安装/查看 Docker"
        print_menu_item "2" "镜像管理"             "拉取/更新/加速源"
        print_menu_item "3" "服务实例管理"         "添加/删除/启停"
        print_menu_item "4" "查看运行状态"         "总览"
        if [[ "$(count_instances)" -eq 0 ]]; then
            echo ""
            print_menu_item "9" "一键安装启动"     "快速部署首个服务器"
        fi
        echo ""
        print_menu_item "0" "退出"
        echo ""

        local choice
        choice=$(read_input "请选择" "0")

        case "$choice" in
            1) menu_docker ;;
            2) menu_images ;;
            3) menu_instances ;;
            4)
                if check_docker; then
                    show_instances
                else
                    print_error "Docker 未安装"
                fi
                press_enter ;;
            9)
                if [[ "$(count_instances)" -eq 0 ]]; then
                    quick_setup
                else
                    print_error "无效选择"; press_enter
                fi ;;
            0)
                exit 0 ;;
            *) print_error "无效选择"; press_enter ;;
        esac
    done
}

main "$@"
