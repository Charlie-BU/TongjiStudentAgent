package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
)

func TestTongjiUserBasicInfo(t *testing.T) {
	for _, tc := range []struct {
		name, authorization string
		upstream, want      int
		configured          bool
	}{
		{"success", "Bearer test-access-token", 200, 200, true},
		{"missing token", "", 200, 401, true},
		{"invalid scheme", "Basic token", 200, 401, true},
		{"empty bearer", "Bearer", 200, 401, true},
		{"upstream failure", "Bearer test-access-token", 503, 502, true},
		{"missing configuration", "Bearer test-access-token", 200, 500, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("Authorization") != "Bearer test-access-token" {
					t.Error("unexpected authorization")
				}
				w.WriteHeader(tc.upstream)
				fmt.Fprint(w, `{"code":"A00000","data":{"list":[{"name":"测试同学","userId":"2350939","userTypeName":"本科生"}]}}`)
			}))
			defer server.Close()
			for key, value := range map[string]string{"TONGJI_LOGIN_CLIENT_ID": "login", "TONGJI_LOGIN_CLIENT_SECRET": "secret", "TONGJI_OPEN_PLATFORM_REDIRECT_URI": "https://example.test", "TONGJI_OPEN_PLATFORM_STATE_SECRET": "state", "TONGJI_OPEN_PLATFORM_API_BASE_URL": server.URL} {
				t.Setenv(key, value)
			}
			if !tc.configured {
				t.Setenv("TONGJI_LOGIN_CLIENT_SECRET", "")
			}
			request := app.NewContext(0)
			request.Request.Header.Set("Authorization", tc.authorization)
			TongjiUserBasicInfo(context.Background(), request)
			if request.Response.StatusCode() != tc.want {
				t.Fatalf("status=%d want=%d", request.Response.StatusCode(), tc.want)
			}
			if tc.want == 200 && string(request.Response.Body()) != `{"name":"测试同学","userId":"2350939","userTypeName":"本科生"}` {
				t.Fatalf("unexpected response: %s", request.Response.Body())
			}
			if (tc.want == 401 || tc.want == 500) && calls != 0 {
				t.Fatal("unexpected upstream call")
			}
		})
	}
}
