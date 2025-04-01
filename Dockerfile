# Dockerfile for Markdown-translator-go

# --- 构建阶段 (Build Stage) ---
# 使用官方 Go Alpine 镜像作为构建环境，体积较小
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 优化 Docker 缓存: 先复制依赖管理文件
COPY go.mod go.sum ./
# 下载并验证依赖。如果 go.mod/go.sum 未变，这一层会被缓存
RUN go mod download && go mod verify

# 复制所有源代码到工作目录
COPY . .

# 编译 Go 应用
# CGO_ENABLED=0: 禁用 CGO，生成静态链接的可执行文件，不依赖宿主机的 C 库
# -ldflags="-w -s": 优化标记，-w 去除调试信息，-s 去除符号表，减小最终二进制文件大小
# -o /app/Markdown-translator-go-app: 指定输出文件路径和名称
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/Markdown-translator-go-app ./main.go

# --- 运行阶段 (Runtime Stage) ---
# 使用非常小的 Alpine Linux 作为最终运行环境的基础镜像
FROM alpine:latest

# 设置工作目录
WORKDIR /app

# 从构建阶段复制编译好的二进制可执行文件
COPY --from=builder /app/Markdown-translator-go-app /app/Markdown-translator-go-app
# 复制默认的 Prompt 模板文件，以便程序能找到它
COPY prompt.template /app/prompt.template

# (可选) 创建一个非 root 用户和组来运行程序，提高安全性
# RUN addgroup -S appgroup && adduser -S appuser -G appgroup
# USER appuser

# 设置容器启动时执行的命令为我们的应用程序
ENTRYPOINT ["/app/Markdown-translator-go-app"]

# (可选) 可以设置默认的命令行参数，例如显示帮助信息
# CMD ["--help"]
