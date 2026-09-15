package openroutermodel

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	. "github.com/smartystreets/goconvey/convey"
)

// streamTestTransport 返回离线响应，同时保留 SDK 实际使用的请求上下文。
type streamTestTransport func(*http.Request) (*http.Response, error)

func (f streamTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// stalledStreamBody 在已有数据耗尽后阻塞，只有 Close 能唤醒读取。
type stalledStreamBody struct {
	io.Reader
	blocked, closed, exited chan struct{}
	readOnce, closeOnce     sync.Once
	closeCalls              atomic.Int32
}

func (b *stalledStreamBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if n > 0 || err != io.EOF {
		return n, err
	}
	b.readOnce.Do(func() { close(b.blocked) })
	<-b.closed
	close(b.exited)
	return 0, io.EOF
}

func (b *stalledStreamBody) Close() error {
	b.closeCalls.Add(1)
	b.closeOnce.Do(func() { close(b.closed) })
	return nil
}

// awaitStreamSignal 使用通道同步，超时只用于防止回归导致测试挂起。
func awaitStreamSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal("stream lifecycle signal was not received")
	}
}

func TestStreamCloseWhileUpstreamStalled(t *testing.T) {
	Convey("关闭消息流会中断暂停的上游读取", t, func() {
		for _, receiveFirst := range []bool{false, true} {
			name := "首个事件之前关闭"
			if receiveFirst {
				name = "读取首段后关闭"
			}
			Convey(name, func() {
				prefix := ""
				if receiveFirst {
					prefix = sse(map[string]any{"type": "response.output_text.delta", "delta": "hi"})
				}
				body := &stalledStreamBody{Reader: strings.NewReader(prefix), blocked: make(chan struct{}), closed: make(chan struct{}), exited: make(chan struct{})}
				t.Cleanup(func() { body.closeOnce.Do(func() { close(body.closed) }) })
				var requestContext context.Context
				client := &http.Client{Transport: streamTestTransport(func(r *http.Request) (*http.Response, error) {
					requestContext = r.Context()
					return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: body}, nil
				})}
				m, err := newModel(config{APIKey: "test", Model: "test", BaseURL: "https://example.test", HTTPClient: client})
				So(err, ShouldBeNil)
				reader, err := m.Stream(context.Background(), []*schema.Message{schema.UserMessage("hi")})
				So(err, ShouldBeNil)
				if receiveFirst {
					msg, err := reader.Recv()
					So(err, ShouldBeNil)
					So(msg.Content, ShouldEqual, "hi")
				}
				awaitStreamSignal(t, body.blocked)
				reader.Close()
				awaitStreamSignal(t, body.closed)
				awaitStreamSignal(t, requestContext.Done())
				awaitStreamSignal(t, body.exited)
				So(body.closeCalls.Load(), ShouldEqual, 1)
			})
		}
	})
}

func TestStreamClosesUpstreamOnCompletionAndFailure(t *testing.T) {
	Convey("正常完成和协议失败均释放响应体", t, func() {
		for _, failed := range []bool{false, true} {
			payload := sse(map[string]any{"type": "response.completed", "response": textResponse("done")})
			if failed {
				payload = "data: invalid-json\n\n"
			}
			body := &stalledStreamBody{Reader: strings.NewReader(payload), blocked: make(chan struct{}), closed: make(chan struct{}), exited: make(chan struct{})}
			var requestContext context.Context
			client := &http.Client{Transport: streamTestTransport(func(r *http.Request) (*http.Response, error) {
				requestContext = r.Context()
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: body}, nil
			})}
			m, err := newModel(config{APIKey: "test", Model: "test", BaseURL: "https://example.test", HTTPClient: client})
			So(err, ShouldBeNil)
			msg, err := m.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
			if failed {
				So(err, ShouldNotBeNil)
				So(msg, ShouldBeNil)
			} else {
				So(err, ShouldBeNil)
				So(msg.Content, ShouldEqual, "done")
			}
			awaitStreamSignal(t, body.closed)
			awaitStreamSignal(t, requestContext.Done())
			So(body.closeCalls.Load(), ShouldEqual, 1)
		}
	})
}
