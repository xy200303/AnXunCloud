# 物业巡检管理系统 唯一 Dockerfile（构建上下文为项目根目录）
# 通过 --target 区分构建目标：
#   backend-dev   后端开发镜像（air 热编译，源码由 compose 挂载 ./backend -> /app）
#   frontend-dev  前端开发镜像（vite dev server，源码由 compose 挂载 ./frontend -> /app）
#   prod          生产镜像（node 构建 dist → go 构建二进制 → alpine 单端口运行）
# 示例：
#   docker build --target backend-dev -t pi-backend-dev .
#   docker build --target prod -t pi-app .

# ========== 后端开发 ==========
FROM golang:1.26.5-alpine AS backend-dev
WORKDIR /app
RUN go install github.com/air-verse/air@latest
COPY backend/go.mod backend/go.sum ./
RUN go mod download
CMD ["air", "-c", ".air.toml"]

# ========== 前端开发 ==========
FROM node:20-alpine AS frontend-dev
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
# 启动时再 install 一次（应对挂载源码后 package.json 变化），随后以 0.0.0.0 启动 dev server
CMD ["sh", "-c", "npm install --no-audit --no-fund && npm run dev -- --host 0.0.0.0"]

# ========== 生产：阶段一 构建前端 SPA ==========
FROM node:20-alpine AS fe
WORKDIR /fe
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ========== 生产：阶段二 构建后端二进制 ==========
FROM golang:1.26.5-alpine AS be
WORKDIR /be
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/server ./cmd/server

# ========== 生产：最终运行镜像（二进制 + dist + 中文字体） ==========
FROM alpine:3.20 AS prod
RUN apk add --no-cache ca-certificates tzdata curl
ENV TZ=Asia/Shanghai
# 水印中文字体（Noto Sans SC，构建期下载；下载失败不阻断构建，水印功能自动降级并记录日志）
RUN mkdir -p /app/fonts \
    && (curl -fsSL -o /app/fonts/NotoSansSC.ttf \
        "https://github.com/google/fonts/raw/main/ofl/notosanssc/NotoSansSC%5Bwght%5D.ttf" || true)
WORKDIR /app
COPY --from=be /out/server /app/server
COPY --from=fe /fe/dist /app/dist
EXPOSE 8080
CMD ["/app/server"]
