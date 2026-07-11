# What
FaaS 运行原生 Golang HTTP 框架 Hertz 的 Demo
Hertz 用户文档：https://bytedance.feishu.cn/wiki/wikcn5G6W2cX7vtj5uSQHrtDGHf

# 项目结构
```
.
├── biz                 # 示例用户代码逻辑 sample user code logic
│   ├── config
│   │   └── config.go
│   ├── dal
│   │   └── error.go
│   ├── debug
│   │   ├── idl.go
│   │   └── server.go
│   ├── handler
│   │   └── ping.go
│   ├── idl.go
│   └── service
├── build.sh             # 用户构建脚本 build script for bytefaas-cli/scm build artifacts
├── aipaas.yml
├── conf                 # Hertz 配置文件 Hertz related configurations
│   ├── config.yaml
│   └── toutiao_bytefaas_bytefaas_native_hertz.yaml
├── go.mod
├── go.sum
├── main.go
├── output               # Hertz 构建产物 build artifacts
│   ├── bin
│   │   └── toutiao.bytefaas.bytefaas_native_hertz
│   ├── bootstrap.sh
│   ├── bootstrap_staging.sh
│   ├── conf
│       ├── config.yaml
│       └── toutiao_bytefaas_bytefaas_native_hertz.yaml
│   
│   
├── router.go            # 自定义router 路由 service self defined routes
├── router_gen.go        # Hertz IDL 自动生成 router 路由 Hertz IDL generated routes
├── run.sh               # ByteFaaS 启动脚本 bytefaas launching script
└── script               # Hertz 启动脚本 hertz launching script
    ├── bootstrap.sh

```
