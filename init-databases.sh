#!/usr/bin/env bash
# Initialize running Newbee RPC services. Database creation is a separate step.
set -euo pipefail

ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
SERVICES=(core cmdb ops-center unified-io job)
SELECTED=()
DRY_RUN=false
GRPCURL=${GRPCURL:-grpcurl}
TIMEOUT=${NEWBEE_INIT_TIMEOUT:-300}

usage() {
    cat <<'EOF'
用法: bash init-databases.sh [--service NAME ...] [--dry-run]
新蜂资产管理平台空库初始化（先配置并启动 RPC，再运行此工具）。

  -s, --service NAME  选择服务，可重复；默认初始化所有服务
  -l, --list          列出服务和实际地址
      --dry-run       显示调用命令，不连接或修改数据库
  -h, --help          显示帮助

服务: core、cmdb、ops-center、unified-io、job
地址变量: NEWBEE_CORE_RPC、NEWBEE_CMDB_RPC、NEWBEE_OPS_RPC、
          NEWBEE_IO_RPC、NEWBEE_JOB_RPC
其他变量: GRPCURL（命令或路径）、NEWBEE_INIT_TIMEOUT（秒，默认300）
示例: bash init-databases.sh -s core -s cmdb
EOF
}

service_config() {
    case "$1" in
        core)       TARGET=${NEWBEE_CORE_RPC:-127.0.0.1:9100}; PROTO=core/rpc/core.proto; METHOD=core.Core/initDatabase ;;
        cmdb)       TARGET=${NEWBEE_CMDB_RPC:-127.0.0.1:9200}; PROTO=cmdb/rpc/cmdb.proto; METHOD=cmdb.Cmdb/initDatabase ;;
        ops-center) TARGET=${NEWBEE_OPS_RPC:-127.0.0.1:9600}; PROTO=ops-center/rpc/desc/ops.proto; METHOD=ops.Ops/initDatabase ;;
        unified-io) TARGET=${NEWBEE_IO_RPC:-127.0.0.1:9500}; PROTO=unified-io/rpc/io.proto; METHOD=io.Io/initDatabase ;;
        job)        TARGET=${NEWBEE_JOB_RPC:-127.0.0.1:9105}; PROTO=job/job.proto; METHOD=job.Job/initDatabase ;;
        *) echo "未知服务: $1" >&2; return 2 ;;
    esac
}

while (($#)); do
    case "$1" in
        -s|--service)
            if (($# < 2)); then echo '缺少服务名称' >&2; exit 2; fi
            service_config "$2"
            SELECTED+=("$2")
            shift 2 ;;
        --dry-run) DRY_RUN=true; shift ;;
        -l|--list)
            for name in "${SERVICES[@]}"; do service_config "$name"; printf '%-12s %s %s\n' "$name" "$TARGET" "$METHOD"; done
            exit 0 ;;
        -h|--help) usage; exit 0 ;;
        *) echo "未知选项: $1" >&2; usage >&2; exit 2 ;;
    esac
done

if [[ ! "$TIMEOUT" =~ ^[1-9][0-9]*$ ]]; then
    echo 'NEWBEE_INIT_TIMEOUT 必须是正整数秒' >&2
    exit 2
fi
if [[ "$DRY_RUN" == false ]] && ! command -v "$GRPCURL" >/dev/null 2>&1; then
    echo '未找到 grpcurl；请安装并加入 PATH，或设置 GRPCURL 为其路径' >&2
    exit 2
fi

success=0
failed=0
total=0
# Always preserve dependency order, even if selection flags are in another order.
for name in "${SERVICES[@]}"; do
    if ((${#SELECTED[@]})); then
        found=false
        for selected in "${SELECTED[@]}"; do [[ "$selected" != "$name" ]] || found=true; done
        [[ "$found" == true ]] || continue
    fi
    service_config "$name"
    if [[ ! -f "$ROOT/$PROTO" ]]; then
        echo "缺少 $PROTO；请执行 git submodule update --init --recursive" >&2
        exit 2
    fi
    args=(-plaintext -connect-timeout 5 -max-time "$TIMEOUT"
        -import-path "$ROOT/$(dirname "$PROTO")" -proto "$(basename "$PROTO")"
        -H tenant_id:1 -d '{}' "$TARGET" "$METHOD")
    total=$((total + 1))
    if [[ "$DRY_RUN" == true ]]; then
        printf '%q ' "$GRPCURL" "${args[@]}"; printf '\n'
        continue
    fi
    printf '[%s] 初始化 %s\n' "$name" "$TARGET"
    # Test the RPC's exit status directly; an unreachable service is a failure.
    if "$GRPCURL" "${args[@]}"; then
        success=$((success + 1))
    else
        failed=$((failed + 1))
        printf '[%s] 初始化失败\n' "$name" >&2
        # Later services depend on Core tables and must not run after its failure.
        [[ "$name" != core ]] || break
    fi
done

if [[ "$DRY_RUN" == false ]]; then
    printf '初始化结果: 总计=%d 成功=%d 失败=%d\n' "$total" "$success" "$failed"
    ((failed == 0))
fi
