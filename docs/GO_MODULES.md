# Go 依赖安装指南

## 已验证的配置

在项目根目录执行下列命令可以成功完成依赖整理：

```bash
GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn go mod tidy
```

该命令只对本次执行生效，不会修改机器上的全局 Go 配置。2026-08-01 已在本仓库用 Go `1.26.5` 验证通过。

## 推荐的一次性配置

若本机需要长期使用该代理，执行：

```bash
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=sum.golang.google.cn
```

之后可直接运行：

```bash
go mod tidy
go mod download
go test ./...
```

用下面的命令检查实际生效的配置：

```bash
go env GOPROXY GOSUMDB GOPRIVATE HTTP_PROXY HTTPS_PROXY NO_PROXY
```

## 常见网络问题与处理

本仓库此前的 `GOPROXY=https://mirrors.aliyun.com/goproxy/,direct` 在当前网络中失败，报错为 `lookup mirrors.aliyun.com: no such host`。这表示 DNS 或网络出口无法连接该镜像，不是 `go.mod` 的依赖声明问题。

如果公司/校园网络要求 HTTP(S) 代理，应由网络管理员提供代理地址，并在当前 shell 中设置后再运行安装命令：

```bash
export HTTPS_PROXY=http://proxy.example.com:port
export HTTP_PROXY=http://proxy.example.com:port
GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn go mod tidy
```

不要把真实代理账号、密码或令牌提交到仓库，也不要写入 `.env`。终端关闭后，如不希望继续使用，可执行：

```bash
unset HTTPS_PROXY HTTP_PROXY
```

若需要恢复 Go 的默认持久化设置：

```bash
go env -u GOPROXY
go env -u GOSUMDB
```

## 排查顺序

1. 确认在 `TongjiStudentAgent` 目录运行命令，该目录含有 `go.mod`。
2. 运行 `go env GOPROXY GOSUMDB`，确认没有指向不可访问的代理。
3. 优先使用本文“已验证的配置”重新运行 `go mod tidy`。
4. 若仍出现 `lookup ... no such host`，检查 DNS、VPN、公司/校园网络策略，或配置经批准的 HTTP(S) 代理。
5. 若下载成功但校验失败，不要关闭校验；先确认 `GOSUMDB` 配置及网络出口是否被拦截。

`GOPRIVATE` 仅应包含需要绕过公共代理和校验服务的私有模块域名；不要为了规避公共模块下载错误而把 `github.com`、`golang.org` 等公共域名加入其中。
