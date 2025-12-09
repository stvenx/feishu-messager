# 构建阶段
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 复制依赖文件
COPY . .
RUN go mod download

# 编译 - 静态编译，减小体积
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -a -installsuffix cgo \
    -o action .

# 运行阶段 - 使用最小镜像
FROM gcr.io/distroless/static:nonroot

LABEL "com.github.actions.name"="Feishu Messenger"
LABEL "com.github.actions.description"="Post Feishu messages from your own bot"
LABEL "com.github.actions.icon"="bell"
LABEL "com.github.actions.color"="blue"

LABEL version="1.0.0"
LABEL repository="https://github.com/stvenx/feishu-messager.git"
LABEL homepage="https://github.com/stvenx/feishu-messager.git"
LABEL maintainer="stvenx"

WORKDIR /root/

COPY --from=builder /app/action /action

USER nonroot:nonroot

ENTRYPOINT ["/action"]

