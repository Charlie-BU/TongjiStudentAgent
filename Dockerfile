FROM golang:1.25.10-bookworm AS build

WORKDIR /src

ARG GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tongji-student-agent .

FROM debian:bookworm-slim AS runtime

# Bootstrap without CA certificates; Debian signature and hash checks remain enabled.
RUN sed -i 's|http://deb.debian.org|http://mirrors.tuna.tsinghua.edu.cn|g; s|http://security.debian.org|http://mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list.d/debian.sources \
 && printf '%s\n' \
      'Acquire::Retries "2";' \
      'Acquire::http::Timeout "30";' \
      'Acquire::https::Timeout "30";' \
      'Acquire::http::Pipeline-Depth "0";' \
      'APT::Update::Error-Mode "any";' \
      > /etc/apt/apt.conf.d/99ci-network \
 && timeout 900 sh -ec 'apt-get update && apt-get install -y --no-install-recommends ca-certificates curl chromium fonts-noto-cjk' \
 && chromium --version \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/tongji-student-agent /app/tongji-student-agent

# The application passes this path to chromedp explicitly instead of relying
# on its host-dependent executable discovery.
ENV CHROME_BIN=/usr/bin/chromium

# Chromium and Crashpad need writable config/cache directories for the runtime UID.
RUN mkdir -p /home/agent/.config /home/agent/.cache \
 && chown -R 65532:65532 /home/agent

ENV HOME=/home/agent \
    XDG_CONFIG_HOME=/home/agent/.config \
    XDG_CACHE_HOME=/home/agent/.cache

HEALTHCHECK --interval=5s --timeout=4s --start-period=60s --retries=12 \
  CMD curl --fail --silent --show-error --max-time 3 "http://127.0.0.1:${APP_PORT:-8080}/v1/ping" > /dev/null || exit 1

USER 65532:65532
CMD ["/app/tongji-student-agent"]
