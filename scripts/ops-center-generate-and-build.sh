#!/usr/bin/env bash
set -euo pipefail

# One-shot generation + build script for Ops Center (RPC + API)
# Prerequisites (installed in PATH):
# - go (>=1.21)
# - protoc, protoc-gen-go, protoc-gen-go-grpc (if your proto triggers codegen)
# - goctls (or goctl variant used in your team)
# - internet optional (only if your env resolves missing modules)

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
RPC_DIR="$ROOT_DIR/ops-center/rpc"
API_DIR="$ROOT_DIR/ops-center/api"

# Tools
GOCTLS_BIN=${GOCTLS_BIN:-goctls}
ENT_GEN_CMD=${ENT_GEN_CMD:-"go run entgo.io/ent/cmd/ent generate ./ent/schema --feature sql/execquery,intercept,sql/modifier"}

echo "[gen] Working directory: $ROOT_DIR"

cd "$RPC_DIR"
echo "[gen] Generating Ent models..."
echo "> $ENT_GEN_CMD"
sh -c "$ENT_GEN_CMD"

echo "[gen] Generating RPC code (goctls) ..."
if ! command -v "$GOCTLS_BIN" >/dev/null 2>&1; then
  echo "[warn] $GOCTLS_BIN not found in PATH. Skipping RPC generation."
else
  # Adjust arguments to your local goctls usage
  set +e
  "$GOCTLS_BIN" rpc ent \
    --schema=./ent/schema \
    --style=go_zero \
    --service_name=newbee-ops-rpc \
    --output=./ \
    --model=all \
    --proto_out=./desc/ops.proto \
    --i18n \
    --overwrite=true
  rc=$?
  set -e
  if [ $rc -ne 0 ]; then
    echo "[warn] goctls generation returned non-zero exit ($rc). Proceeding with existing code."
  fi
fi

echo "[deps] Download & tidy RPC deps..."
GOWORK=off go mod download || true
GOWORK=off go mod tidy || true

echo "[build] Building RPC..."
GOCACHE="$RPC_DIR/.gocache" GOWORK=off go build ./...

echo "[build] Building API..."
cd "$API_DIR"
echo "[deps] Download & tidy API deps (workspace on)..."
go mod download || true
go mod tidy || true
GOCACHE="$API_DIR/.gocache" go build ./...

echo "[ok] Generation + build succeeded (RPC + API)."
echo "[env] Setting module proxy for generation/build (override via env if needed)"
export GOPROXY=${GOPROXY:-https://goproxy.cn,direct}
export GOSUMDB=${GOSUMDB:-off}
# mark private orgs to skip sumdb
export GOPRIVATE=${GOPRIVATE:-github.com/coder-lulu/*}
export GONOSUMDB=${GONOSUMDB:-github.com/coder-lulu/*}
