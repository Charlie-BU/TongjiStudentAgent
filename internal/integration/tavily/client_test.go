package tavily

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

// failingBody 模拟读取阶段超时或连接中断并记录关闭行为。
type failingBody struct {
	err    error
	closed bool
}

func (b *failingBody) Read([]byte) (int, error) { return 0, b.err }
func (b *failingBody) Close() error             { b.closed = true; return nil }

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newTestClient(body string, code int) *Client {
	c, _ := NewClient("test-key", time.Second, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	}))
	return c
}

func TestClientSearch(t *testing.T) {
	Convey("搜索协议和认证隔离", t, func() {
		var request *http.Request
		c, err := NewClient("test-key", time.Second, roundTripFunc(func(r *http.Request) (*http.Response, error) {
			request = r
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"results":[{"title":"公告","url":"https://example.org","content":"正文"}],"request_id":"test-id"}`))}, nil
		}))
		So(err, ShouldBeNil)
		response, err := c.Search(context.Background(), SearchInput{Query: "公开公告", MaxResults: 5, StartDate: "2026-01-01", IncludeDomains: []string{"example.org"}})
		So(err, ShouldBeNil)
		So(response.Results, ShouldHaveLength, 1)
		So(request.Method, ShouldEqual, "POST")
		So(request.URL.String(), ShouldEqual, "https://api.tavily.com/search")
		So(request.Header.Get("Authorization"), ShouldEqual, "Bearer test-key")
		So(request.Header.Get("Content-Type"), ShouldEqual, "application/json")
		So(request.Header.Get("Cookie"), ShouldBeEmpty)
		So(request.Header.Get("X-Tongji-Access-Token"), ShouldBeEmpty)
		var data map[string]any
		So(json.NewDecoder(request.Body).Decode(&data), ShouldBeNil)
		So(data["query"], ShouldEqual, "公开公告")
		So(data["start_date"], ShouldEqual, "2026-01-01")
		So(data["search_depth"], ShouldEqual, "basic")
		So(data["max_results"], ShouldEqual, 5)
		for _, key := range []string{"auto_parameters", "include_answer", "include_raw_content", "include_images"} {
			So(data[key], ShouldEqual, false)
		}
		So(data["reason"], ShouldBeNil)
	})
}

func TestClientErrors(t *testing.T) {
	Convey("传输错误和响应边界", t, func() {
		for _, tc := range []struct {
			code   int
			status string
		}{{400, "invalid_arguments"}, {401, "web_unavailable"}, {429, "rate_limited"}, {432, "quota_exceeded"}, {433, "quota_exceeded"}, {500, "web_unavailable"}, {302, "web_unavailable"}} {
			c := newTestClient("secret upstream body", tc.code)
			_, err := c.Search(context.Background(), SearchInput{})
			So(Status(err), ShouldEqual, tc.status)
			So(err.Error(), ShouldNotContainSubstring, "secret")
		}
		for _, body := range []string{"broken", `null`, `{}`, `{"results":[]} trailing`, strings.Repeat("x", maxResponseBytes+1)} {
			_, err := newTestClient(body, 200).Search(context.Background(), SearchInput{})
			So(Status(err), ShouldEqual, "web_unavailable")
		}
		result, err := newTestClient(`{"results":[]}`, 200).Search(context.Background(), SearchInput{})
		So(err, ShouldBeNil)
		So(result.Results, ShouldBeEmpty)
		c := newTestClient(`{}`, 200)
		So(c.http.CheckRedirect(nil, nil), ShouldEqual, http.ErrUseLastResponse)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = c.Search(ctx, SearchInput{})
		So(errors.Is(err, context.Canceled), ShouldBeTrue)
		c.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })
		_, err = c.Search(context.Background(), SearchInput{})
		So(Status(err), ShouldEqual, "timeout")
		c.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("private network details") })
		_, err = c.Search(context.Background(), SearchInput{})
		So(err.Error(), ShouldEqual, "web_unavailable")
		var empty *Client
		_, err = empty.Search(context.Background(), SearchInput{})
		So(Status(err), ShouldEqual, "web_unavailable")
	})
}

func TestClientExtract(t *testing.T) {
	Convey("提取正文和业务失败", t, func() {
		for _, body := range []string{`{"results":[],"failed_results":[{"url":"https://example.org","error":"secret"}]}`, `{"results":[{"raw_content":" "}]}`, `{"results":[]}`, `null`} {
			_, err := newTestClient(body, 200).Extract(context.Background(), ExtractInput{URL: "https://example.org"})
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldNotContainSubstring, "secret")
		}
		for _, query := range []string{"", "时间"} {
			c := newTestClient("", 200)
			var data map[string]any
			c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				err := json.NewDecoder(r.Body).Decode(&data)
				if err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"results":[{"url":"https://example.org","raw_content":"通知正文"}],"failed_results":[]}`))}, nil
			})
			result, err := c.Extract(context.Background(), ExtractInput{URL: "https://example.org", Query: query})
			So(err, ShouldBeNil)
			So(result.Results[0].Content, ShouldEqual, "通知正文")
			So(data["urls"], ShouldResemble, []any{"https://example.org"})
			So(data["format"], ShouldEqual, "markdown")
			if query == "" {
				So(data["query"], ShouldBeNil)
				So(data["chunks_per_source"], ShouldBeNil)
			} else {
				So(data["query"], ShouldEqual, query)
				So(data["chunks_per_source"], ShouldEqual, 5)
			}
		}
	})
}

func TestClientCancellationAndRedirect(t *testing.T) {
	Convey("请求取消、读取失败和重定向边界", t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c, err := NewClient("test-key", time.Second, roundTripFunc(func(r *http.Request) (*http.Response, error) {
			cancel()
			return nil, r.Context().Err()
		}))
		So(err, ShouldBeNil)
		_, err = c.Search(ctx, SearchInput{})
		So(errors.Is(err, context.Canceled), ShouldBeTrue)
		for _, readErr := range []error{context.DeadlineExceeded, errors.New("private connection details")} {
			body := &failingBody{err: readErr}
			c.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: body}, nil
			})
			_, err = c.Search(context.Background(), SearchInput{})
			So(body.closed, ShouldBeTrue)
			if errors.Is(readErr, context.DeadlineExceeded) {
				So(Status(err), ShouldEqual, "timeout")
			} else {
				So(Status(err), ShouldEqual, "web_unavailable")
			}
		}
		calls := 0
		c.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://other.example.org"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		})
		_, err = c.Search(context.Background(), SearchInput{})
		So(Status(err), ShouldEqual, "web_unavailable")
		So(calls, ShouldEqual, 1)
	})
}
