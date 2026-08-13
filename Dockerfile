# 物业巡检管理系统 唯一 Dockerfile（构建上下文为项目根目录）
# 通过 --target 区分构建目标：
#   backend-dev   后端开发镜像（air 热编译，源码由 compose 挂载 ./backend -> /app）
#   frontend-dev  前端开发镜像（vite dev server，源码由 compose 挂载 ./frontend -> /app）
#   prod          生产镜像（node 构建 dist → go 构建二进制 → alpine 单端口运行）
# 示例：
#   docker build --target backend-dev -t pi-backend-dev .
#   docker build --target prod -t pi-app .
#
# 国内加速：Go 模块走 goproxy.cn、npm 走 npmmirror、apk 走阿里云镜像，
# 均可用 build-arg 覆盖（如服务器在海外：--build-arg GOPROXY=direct 等）。
ARG GOPROXY=https://goproxy.cn,direct
ARG NPM_REGISTRY=https://registry.npmmirror.com
ARG ALPINE_MIRROR=mirrors.aliyun.com

# ========== 后端开发 ==========
FROM golang:1.26.5-alpine AS backend-dev
ARG GOPROXY
ENV GOPROXY=$GOPROXY
WORKDIR /app
RUN go install github.com/air-verse/air@latest
COPY backend/go.mod backend/go.sum ./
RUN go mod download
CMD ["air", "-c", ".air.toml"]

# ========== 前端开发 ==========
FROM node:20-alpine AS frontend-dev
ARG NPM_REGISTRY
RUN npm config set registry $NPM_REGISTRY
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
# 启动时再 install 一次（应对挂载源码后 package.json 变化），随后以 0.0.0.0 启动 dev server
CMD ["sh", "-c", "npm install --no-audit --no-fund && npm run dev -- --host 0.0.0.0"]

# ========== 生产：阶段一 构建前端 SPA ==========
FROM node:20-alpine AS fe
ARG NPM_REGISTRY
RUN npm config set registry $NPM_REGISTRY
WORKDIR /fe
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ========== 生产：阶段二 构建后端二进制 ==========
FROM golang:1.26.5-alpine AS be
ARG GOPROXY
ENV GOPROXY=$GOPROXY
WORKDIR /be
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/server ./cmd/server

# ========== 生产：最终运行镜像（二进制 + dist + 中文字体） ==========
FROM alpine:3.20 AS prod
ARG ALPINE_MIRROR
RUN sed -i "s/dl-cdn.alpinelinux.org/$ALPINE_MIRROR/g" /etc/apk/repositories \
    && apk add --no-cache ca-certificates tzdata curl
ENV TZ=Asia/Shanghai
# 水印中文字体（Noto Sans SC，随仓库 backend/fonts 提供，构建阶段一并复制）
COPY --from=be /be/fonts /app/fonts
# 二维码标牌 LOGO（随仓库 backend/assets 提供，可替换为公司 LOGO）
COPY --from=be /be/assets /app/assets
WORKDIR /app
COPY --from=be /out/server /app/server
COPY --from=fe /fe/dist /app/dist
EXPOSE 8080
CMD ["/app/server"]
