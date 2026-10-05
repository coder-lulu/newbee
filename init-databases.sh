#!/bin/bash

# ============================================
# NewBee 微服务数据库迁移工具
# 功能：批量调用各微服务的RPC InitDatabase接口
# 作者：系统自动生成
# ============================================

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_step() {
    echo -e "\n${CYAN}==>${NC} $1"
}

# 微服务配置 (服务名称:端口:proto包名:proto服务名)
SERVICES=(
    "core:9100:core:Core"
    "cmdb:9200:cmdb:Cmdb"
    "unified-io:9500:io:Io"
    "ops-center:9600:ops:Ops"
)

# 检查grpcurl是否安装
check_grpcurl() {
    if ! command -v grpcurl &> /dev/null; then
        log_error "grpcurl 未安装，请先安装: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest"
        exit 1
    fi
    log_success "grpcurl 已安装"
}

# 检查服务是否在线
check_service() {
    local service_name=$1
    local port=$2

    if nc -z 127.0.0.1 $port 2>/dev/null; then
        return 0  # 服务在线
    else
        return 1  # 服务离线
    fi
}

# 调用服务的initDatabase接口
init_database() {
    local service_name=$1
    local port=$2
    local package=$3
    local proto_service=$4

    log_step "初始化 ${service_name} 数据库"

    # 检查服务是否在线
    if ! check_service "$service_name" "$port"; then
        log_warning "${service_name} 服务未运行 (127.0.0.1:${port})，跳过"
        return 1
    fi

    log_info "${service_name} 服务在线，开始初始化数据库..."

    # 调用initDatabase接口
    local result
    result=$(grpcurl -plaintext \
        -d '{}' \
        127.0.0.1:${port} \
        ${package}.${proto_service}.initDatabase 2>&1)

    local exit_code=$?

    if [ $exit_code -eq 0 ]; then
        log_success "${service_name} 数据库初始化成功"
        echo "    响应: $result"
        return 0
    else
        log_error "${service_name} 数据库初始化失败"
        echo "    错误: $result"
        return 1
    fi
}

# 主函数
main() {
    echo -e "${CYAN}"
    echo "============================================"
    echo "  NewBee 微服务数据库迁移工具"
    echo "============================================"
    echo -e "${NC}"

    # 检查grpcurl
    check_grpcurl

    local success_count=0
    local fail_count=0
    local skip_count=0
    local total=${#SERVICES[@]}

    # 遍历所有服务
    for service_config in "${SERVICES[@]}"; do
        IFS=':' read -r service_name port package proto_service <<< "$service_config"

        if init_database "$service_name" "$port" "$package" "$proto_service"; then
            ((success_count++))
        elif [ $? -eq 1 ]; then
            ((skip_count++))
        else
            ((fail_count++))
        fi

        sleep 1  # 避免请求过快
    done

    # 统计结果
    echo -e "\n${CYAN}============================================${NC}"
    echo -e "${CYAN}  初始化结果统计${NC}"
    echo -e "${CYAN}============================================${NC}"
    echo -e "总服务数: ${total}"
    echo -e "${GREEN}成功: ${success_count}${NC}"
    echo -e "${YELLOW}跳过: ${skip_count}${NC}"
    echo -e "${RED}失败: ${fail_count}${NC}"
    echo -e "${CYAN}============================================${NC}\n"

    if [ $fail_count -gt 0 ]; then
        exit 1
    fi
}

# 显示帮助信息
show_help() {
    cat << EOF
用法: $0 [选项]

NewBee 微服务数据库迁移工具

选项:
    -h, --help          显示此帮助信息
    -s, --service NAME  只初始化指定服务 (core|cmdb|unified-io|ops-center)
    -l, --list          列出所有支持的服务

示例:
    $0                      # 初始化所有微服务
    $0 -s core              # 只初始化core服务
    $0 -s cmdb -s ops-center  # 初始化指定的多个服务

EOF
}

# 列出所有服务
list_services() {
    echo "支持的微服务:"
    for service_config in "${SERVICES[@]}"; do
        IFS=':' read -r service_name port package proto_service <<< "$service_config"
        echo "  - ${service_name} (端口: ${port}, 包: ${package}, 服务: ${proto_service})"
    done
}

# 只初始化指定服务
init_specific_service() {
    local target_service=$1
    local found=false

    for service_config in "${SERVICES[@]}"; do
        IFS=':' read -r service_name port package proto_service <<< "$service_config"

        if [ "$service_name" == "$target_service" ]; then
            found=true
            init_database "$service_name" "$port" "$package" "$proto_service"
            break
        fi
    done

    if [ "$found" == false ]; then
        log_error "服务 '${target_service}' 不存在"
        log_info "使用 -l 参数查看支持的服务"
        exit 1
    fi
}

# 解析命令行参数
if [ $# -eq 0 ]; then
    # 无参数，执行全部初始化
    main
else
    case "$1" in
        -h|--help)
            show_help
            exit 0
            ;;
        -l|--list)
            list_services
            exit 0
            ;;
        -s|--service)
            if [ -z "$2" ]; then
                log_error "请指定服务名称"
                show_help
                exit 1
            fi
            check_grpcurl
            init_specific_service "$2"
            exit 0
            ;;
        *)
            log_error "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
fi
