#!/usr/bin/env bash
# Penpot 一键部署：clone 官方仓库 + 复制本目录配置 + 生成密钥 + 启动
# 用法：
#   ./install.sh --domain penpot.example.com
#   ./install.sh --domain penpot.example.com --dir /opt/penpot --tag 2.18
# 说明：
#   - 只部署用官方镜像；需要源码构建请看同目录 README.md §6
#   - 真实密钥写在 <dir>/penpot/penpot.env，不进仓库
set -euo pipefail

DIR="/opt/penpot"
TAG="main"
DOMAIN=""
PORT="9001"
GH_PROXY=""   # 可选：GitHub 加速前缀，如 https://gh-proxy.com/ 或 https://ghfast.top/

usage() {
  echo "用法: $0 --domain <域名> [--dir <部署目录>] [--tag <penpot版本>] [--port <本机端口>] [--gh-proxy <加速前缀>]"
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --domain) DOMAIN="${2:-}"; shift 2 ;;
    --dir)    DIR="${2:-}";    shift 2 ;;
    --tag)    TAG="${2:-}";    shift 2 ;;
    --port)   PORT="${2:-}";   shift 2 ;;
    --gh-proxy) GH_PROXY="${2:-}"; shift 2 ;;
    *) usage ;;
  esac
done

[[ -n "$DOMAIN" ]] || usage

command -v docker >/dev/null || { echo "未安装 docker"; exit 1; }
docker compose version >/dev/null 2>&1 || { echo "未安装 docker compose v2"; exit 1; }
command -v git >/dev/null || { echo "未安装 git"; exit 1; }

BASE="$DIR/penpot"
mkdir -p "$DIR"

if [[ ! -d "$BASE/.git" ]]; then
  echo "==> clone 官方仓库（tag: $TAG）到 $BASE"
  REPO_URL="https://github.com/penpot/penpot.git"
  if [[ -n "$GH_PROXY" ]]; then
    REPO_URL="${GH_PROXY%/}/https://github.com/penpot/penpot.git"
    echo "==> 使用 GitHub 加速前缀: $GH_PROXY"
  fi
  git clone --depth 1 --single-branch --filter=blob:none --branch "$TAG" "$REPO_URL" "$BASE"
else
  echo "==> 已存在 $BASE，跳过 clone"
fi

echo "==> 复制部署配置"
cp docker-compose.yml "$BASE/docker-compose.penpot.yml"

if [[ ! -f "$BASE/penpot.env" ]]; then
  echo "==> 生成 penpot.env（随机密钥）"
  SECRET="$(openssl rand -hex 32)"
  DBPASS="$(openssl rand -base64 24 | tr -d '/+=' | cut -c1-24)"
  sed -e "s#^PENPOT_PUBLIC_URI=.*#PENPOT_PUBLIC_URI=https://$DOMAIN#" \
      -e "s#^PENPOT_HTTP_PORT=.*#PENPOT_HTTP_PORT=$PORT#" \
      -e "s#^PENPOT_SECRET_KEY=.*#PENPOT_SECRET_KEY=$SECRET#" \
      -e "s#^PENPOT_DB_PASSWORD=.*#PENPOT_DB_PASSWORD=$DBPASS#" \
      penpot.env.example > "$BASE/penpot.env"
  chmod 600 "$BASE/penpot.env"
else
  echo "==> 已存在 penpot.env，保留原值（如需改域名请手动编辑）"
fi

echo "==> 启动容器"
cd "$BASE"
docker compose -p penpot -f docker-compose.penpot.yml up -d

echo "==> 等待 backend 就绪"
for i in $(seq 1 60); do
  if docker compose -p penpot -f docker-compose.penpot.yml ps penpot-backend | grep -q "Up"; then break; fi
  sleep 2
done
docker compose -p penpot -f docker-compose.penpot.yml ps

cat <<EOF

完成。接下来：
1) 创建首个管理员（自助注册默认关闭）：
   docker compose -p penpot -f docker-compose.penpot.yml exec penpot-backend \\
     python3 manage.py create-profile --email admin@$DOMAIN --fullname "Admin" --password '换成强密码'
2) 反代：把 127.0.0.1:$PORT 挂到 https://$DOMAIN，务必放行 /ws/notifications 与 /mcp/ws
   （Nginx 示例见 deploy/penpot/README.md 第 3 节）
3) MCP：登录后到账号概览复制 MCP 地址 https://$DOMAIN/mcp/stream?userToken=...
EOF
