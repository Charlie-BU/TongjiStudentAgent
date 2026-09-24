package main

import (
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/network"
	"github.com/cloudwego/hertz/pkg/network/standard"
)

func TestDefaultA2ADiscovery(t *testing.T) {
	var transport network.Transporter
	h := server.New(server.WithHostPorts("127.0.0.1:0"), server.WithTransport(func(o *config.Options) network.Transporter { transport = standard.NewTransporter(o); return transport }))
	customizeRegister(h)
	done := make(chan error, 1)
	go func() { done <- h.Run() }()
	defer func() { h.Close(); <-done }()
	deadline := time.Now().Add(5 * time.Second)
	for !h.IsRunning() {
		if time.Now().After(deadline) {
			t.Fatal("server did not start")
		}
		time.Sleep(time.Millisecond)
	}
	address := transport.(interface{ Listener() net.Listener }).Listener().Addr().String()
	req, err := http.NewRequest("GET", "http://"+address+"/a2a/.well-known/agent-card.json", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "124.223.93.75:8080"
	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("A2A discovery not registered: %d", res.StatusCode)
	}
	var card struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(res.Body).Decode(&card); err != nil {
		t.Fatal(err)
	}
	if card.URL != "http://124.223.93.75:8080/a2a/v0.3" {
		t.Fatalf("wrong request address: %s", card.URL)
	}
}
