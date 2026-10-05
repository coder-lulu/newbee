#!/bin/bash

# 服务名与目录映射配置
declare -A SERVICE_DIR_MAP=(
    ["core"]="core"
    ["cmdb"]="cmdb"
    ["ops"]="ops-center"
    ["io"]="unified-io"
    ["worker"]="worker"
    ["agent"]="nb-agent"
    # 可以继续添加其他服务映射
    # ["服务名"]="服务目录"
)

# 服务进程ID存储目录
PID_DIR="/tmp/service_pids"
mkdir -p "$PID_DIR"

# 显示使用方法
usage() {
    echo "Usage: $0 {start|stop|restart|status} -s <service_name>"
    echo "       $0 {start|stop|restart|status} all"
    echo ""
    echo "Examples:"
    echo "  $0 start -s core     # 启动core服务"
    echo "  $0 stop -s core      # 停止core服务"
    echo "  $0 restart -s core   # 重启core服务"
    echo "  $0 status -s core    # 查看core服务状态"
    echo "  $0 start all         # 启动所有服务"
    exit 1
}

# 检查服务名是否有效
check_service_name() {
    local service_name=$1
    
    if [ "$service_name" = "all" ]; then
        return 0
    fi
    
    if [ -z "${SERVICE_DIR_MAP[$service_name]}" ]; then
        echo "错误: 未知的服务名 '$service_name'"
        echo "可用的服务名: ${!SERVICE_DIR_MAP[@]}"
        exit 1
    fi
}

# 获取进程树的所有PID（包括子进程）
get_all_pids() {
    local pid=$1
    local pids="$pid"
    
    # 获取所有子进程
    local children=$(pgrep -P "$pid" 2>/dev/null)
    for child in $children; do
        pids="$pids $child"
        # 递归获取孙子进程
        pids="$pids $(get_all_pids "$child")"
    done
    
    echo "$pids"
}

# 启动单个服务类型（使用nohup在后台运行）
start_service_type() {
    local service_name=$1
    local service_type=$2
    local service_dir="${SERVICE_DIR_MAP[$service_name]}"
    
    # 构建服务文件路径
    local service_file=""
    local config_file=""
    
    if [ "$service_type" = "rpc" ]; then
        service_file="${service_dir}/rpc/${service_name}.go"
        config_file="${service_dir}/rpc/etc/${service_name}.yaml"
    else
        service_file="${service_dir}/api/${service_name}.go"
        config_file="${service_dir}/api/etc/${service_name}.yaml"
    fi
    
    # 检查服务文件是否存在
    if [ ! -f "$service_file" ]; then
        echo "警告: 服务文件不存在: $service_file"
        return 1
    fi
    
    # 检查配置文件是否存在
    if [ ! -f "$config_file" ]; then
        echo "警告: 配置文件不存在: $config_file"
        echo "将使用默认配置运行"
    fi
    
    # 检查服务是否已经在运行
    local pid_file="$PID_DIR/${service_name}_${service_type}.pid"
    if [ -f "$pid_file" ]; then
        local main_pid=$(cat "$pid_file")
        if ps -p "$main_pid" > /dev/null 2>&1; then
            echo "ℹ  ${service_name} ${service_type} 服务已经在运行 (PID: $main_pid)"
            return 0
        fi
    fi
    
    # 启动服务
    echo "正在启动 ${service_name} ${service_type} 服务..."
    echo "执行命令: go run $service_file -f $config_file"
    
    # 使用nohup在后台运行，并重定向输出到日志文件
    local log_file="/tmp/${service_name}_${service_type}.log"
    nohup go run "$service_file" -f "$config_file" > "$log_file" 2>&1 &
    local pid=$!
    
    # 保存主进程PID到文件
    echo "$pid" > "$pid_file"
    
    # 等待几秒检查服务是否启动成功
    sleep 3
    
    # 检查进程是否还在运行
    if ps -p "$pid" > /dev/null 2>&1; then
        echo "✓ ${service_name} ${service_type} 服务启动成功 (主进程PID: $pid)"
        echo "  日志文件: $log_file"
        return 0
    else
        echo "✗ ${service_name} ${service_type} 服务启动失败"
        echo "  查看日志: tail -f $log_file"
        rm -f "$pid_file"
        return 1
    fi
}

# 启动服务
start_service() {
    local service_name=$1
    
    if [ "$service_name" = "all" ]; then
        # 启动所有服务
        for svc in "${!SERVICE_DIR_MAP[@]}"; do
            echo "========================================"
            echo "启动服务: $svc"
            start_single_service "$svc"
            echo ""
        done
    else
        # 启动指定服务
        start_single_service "$service_name"
    fi
}

# 启动单个服务（包含rpc和api）
start_single_service() {
    local service_name=$1
    
    echo "开始启动服务: $service_name"
    
    # 先启动rpc服务
    if ! start_service_type "$service_name" "rpc"; then
        echo "RPC服务启动失败，跳过API服务启动"
        return 1
    fi
    
    # 等待RPC服务完全启动
    echo "等待RPC服务完全启动..."
    sleep 5
    
    # 再启动api服务
    if ! start_service_type "$service_name" "api"; then
        echo "API服务启动失败"
        return 1
    fi
    
    echo "✓ 服务 $service_name 启动完成"
    return 0
}

# 停止单个服务类型
stop_service_type() {
    local service_name=$1
    local service_type=$2
    local pid_file="$PID_DIR/${service_name}_${service_type}.pid"
    
    if [ -f "$pid_file" ]; then
        local main_pid=$(cat "$pid_file")
        
        if [ -n "$main_pid" ] && ps -p "$main_pid" > /dev/null 2>&1; then
            echo "正在停止 ${service_name} ${service_type} 服务 (主进程PID: $main_pid)..."
            
            # 获取所有相关进程（包括子进程）
            local all_pids=$(get_all_pids "$main_pid")
            
            # 先尝试正常终止
            for pid in $all_pids; do
                if ps -p "$pid" > /dev/null 2>&1; then
                    kill "$pid" 2>/dev/null
                fi
            done
            
            # 等待进程结束（最多10秒）
            local max_wait=10
            local waited=0
            while ps -p "$main_pid" > /dev/null 2>&1 && [ $waited -lt $max_wait ]; do
                sleep 1
                ((waited++))
            done
            
            # 如果进程仍在运行，强制杀死
            if ps -p "$main_pid" > /dev/null 2>&1; then
                echo "强制终止进程 $main_pid..."
                for pid in $all_pids; do
                    if ps -p "$pid" > /dev/null 2>&1; then
                        kill -9 "$pid" 2>/dev/null
                    fi
                done
                sleep 1
            fi
            
            echo "✓ ${service_name} ${service_type} 服务已停止"
        else
            echo "ℹ  ${service_name} ${service_type} 服务未在运行"
        fi
        
        # 删除PID文件
        rm -f "$pid_file"
        return 0
    else
        echo "ℹ  ${service_name} ${service_type} 服务未在运行"
        return 1
    fi
}

# 停止单个服务
stop_single_service() {
    local service_name=$1
    local stopped_count=0
    
    echo "正在停止服务: $service_name"
    
    # 先停止api服务
    if stop_service_type "$service_name" "api"; then
        ((stopped_count++))
    fi
    
    # 等待api服务完全停止
    sleep 2
    
    # 再停止rpc服务
    if stop_service_type "$service_name" "rpc"; then
        ((stopped_count++))
    fi
    
    if [ $stopped_count -gt 0 ]; then
        echo "✓ 服务 $service_name 已停止"
    else
        echo "ℹ  服务 $service_name 未在运行"
    fi
}

# 停止服务
stop_service() {
    local service_name=$1
    
    if [ "$service_name" = "all" ]; then
        # 停止所有服务
        for svc in "${!SERVICE_DIR_MAP[@]}"; do
            echo "========================================"
            echo "停止服务: $svc"
            stop_single_service "$svc"
            echo ""
        done
    else
        # 停止指定服务
        stop_single_service "$service_name"
    fi
}

# 重启服务
restart_service() {
    local service_name=$1
    
    echo "重启服务: $service_name"
    echo "----------------------------------------"
    
    # 先停止服务
    if [ "$service_name" = "all" ]; then
        stop_service "all"
    else
        stop_single_service "$service_name"
    fi
    
    echo ""
    echo "----------------------------------------"
    echo "等待服务完全停止..."
    sleep 5
    
    # 再启动服务
    if [ "$service_name" = "all" ]; then
        start_service "all"
    else
        start_single_service "$service_name"
    fi
    
    echo "✓ 服务 $service_name 重启完成"
}

# 查看服务状态
status_service() {
    local service_name=$1
    
    if [ "$service_name" = "all" ]; then
        # 查看所有服务状态
        echo "服务状态汇总:"
        echo "========================================"
        for svc in "${!SERVICE_DIR_MAP[@]}"; do
            status_single_service "$svc"
            echo ""
        done
    else
        # 查看指定服务状态
        status_single_service "$service_name"
    fi
}

# 查看单个服务状态
status_single_service() {
    local service_name=$1
    
    echo "服务: $service_name"
    echo "----------------------------------------"
    
    local rpc_running=false
    local api_running=false
    
    # 检查RPC服务
    local rpc_pid_file="$PID_DIR/${service_name}_rpc.pid"
    if [ -f "$rpc_pid_file" ]; then
        local rpc_pid=$(cat "$rpc_pid_file")
        if ps -p "$rpc_pid" > /dev/null 2>&1; then
            echo "RPC服务: 运行中 (PID: $rpc_pid)"
            rpc_running=true
            
            # 显示相关进程
            echo "  相关进程:"
            local rpc_pids=$(get_all_pids "$rpc_pid")
            for pid in $rpc_pids; do
                if ps -p "$pid" > /dev/null 2>&1; then
                    local cmd=$(ps -p "$pid" -o cmd= 2>/dev/null | head -c 80)
                    echo "    - PID $pid: $cmd"
                fi
            done
        else
            echo "RPC服务: 已停止 (PID文件存在但进程不存在)"
            rm -f "$rpc_pid_file"
        fi
    else
        echo "RPC服务: 未运行"
    fi
    
    # 检查API服务
    local api_pid_file="$PID_DIR/${service_name}_api.pid"
    if [ -f "$api_pid_file" ]; then
        local api_pid=$(cat "$api_pid_file")
        if ps -p "$api_pid" > /dev/null 2>&1; then
            echo "API服务: 运行中 (PID: $api_pid)"
            api_running=true
            
            # 显示相关进程
            echo "  相关进程:"
            local api_pids=$(get_all_pids "$api_pid")
            for pid in $api_pids; do
                if ps -p "$pid" > /dev/null 2>&1; then
                    local cmd=$(ps -p "$pid" -o cmd= 2>/dev/null | head -c 80)
                    echo "    - PID $pid: $cmd"
                fi
            done
        else
            echo "API服务: 已停止 (PID文件存在但进程不存在)"
            rm -f "$api_pid_file"
        fi
    else
        echo "API服务: 未运行"
    fi
    
    # 显示日志文件位置
    echo "----------------------------------------"
    echo "日志文件:"
    echo "  RPC: /tmp/${service_name}_rpc.log"
    echo "  API: /tmp/${service_name}_api.log"
}

# 主程序
main() {
    # 检查参数数量
    if [ $# -lt 1 ]; then
        usage
    fi
    
    local action=$1
    local service_name=""
    
    # 解析参数
    case $action in
        start|stop|restart|status)
            shift
            # 解析选项
            while getopts "s:" opt; do
                case $opt in
                    s)
                        service_name="$OPTARG"
                        ;;
                    *)
                        usage
                        ;;
                esac
            done
            
            # 检查是否提供了服务名
            if [ -z "$service_name" ] && [ $# -ge 1 ] && [ "${!#}" != "-s" ]; then
                service_name="${!#}"
            fi
            
            if [ -z "$service_name" ]; then
                echo "错误: 请指定服务名或使用 'all'"
                usage
            fi
            
            # 检查服务名有效性
            check_service_name "$service_name"
            
            # 执行对应操作
            case $action in
                start)
                    start_service "$service_name"
                    ;;
                stop)
                    stop_service "$service_name"
                    ;;
                restart)
                    restart_service "$service_name"
                    ;;
                status)
                    status_service "$service_name"
                    ;;
            esac
            ;;
        *)
            usage
            ;;
    esac
}

# 运行主程序
main "$@"
