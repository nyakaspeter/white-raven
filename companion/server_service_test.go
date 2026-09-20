package main

import (
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/nyakaspeter/white-raven/server/runtime"
)

func TestHarbrrLifecycle(t *testing.T) {
	port := freeTCPPort(t)
	service := &ServerService{
		harbrrLogs: runtime.NewLogBuffer(1000),
		harbrrData: t.TempDir(),
		harbrrPort: port,
	}
	if err := service.StartHarbrr(); err != nil {
		t.Fatalf("StartHarbrr() error = %v", err)
	}
	t.Cleanup(service.StopHarbrr)

	if status := service.HarbrrStatus(); !status.Running {
		t.Fatal("HarbrrStatus().Running = false after start")
	}
	wantAddress := "http://127.0.0.1:" + strconv.Itoa(port)
	waitForHealthyHarbrr(t, wantAddress+"/healthz")

	service.StopHarbrr()
	if status := service.HarbrrStatus(); status.Running {
		t.Fatal("HarbrrStatus().Running = true after stop")
	}
}

func waitForHealthyHarbrr(t *testing.T, address string) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	for range 100 {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address, nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("Harbrr did not become healthy at %s", address)
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}
