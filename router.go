package main

import (
	"code.byted.org/middleware/hertz/pkg/app/server"

	"code.byted.org/bytefaas/bytefaas_native_hertz_a2a_deep_agent_demo/biz/handler"
)

// customizeRegister register customize routers.
func customizeRegister(r *server.Hertz) {
	// your code ...
	r.GET("/", handler.HelloWorld)

	// NOTE: GET v1/ping should be provided for liveness check by FaaS Platform.
	// Status Code OK denotes that the service is healthy.
	r.GET("/v1/ping", handler.Ping)
}
