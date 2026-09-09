FROM golang:1.23.8-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tongji-student-agent .

FROM debian:bookworm-slim AS runtime

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates \
      chromium \
      fonts-noto-cjk \
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

USER 65532:65532
CMD ["/app/tongji-student-agent"]
