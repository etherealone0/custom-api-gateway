package tests

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func runCmd(t *testing.T, args ...string) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = ".." // Run from root of project
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %v failed: %v\nOutput: %s", args, err, out)
	}
}

func generateToken(secret string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "integration-test",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})
	str, _ := token.SignedString([]byte(secret))
	return str
}

func waitForGateway(t *testing.T) {
	client := &http.Client{Timeout: 1 * time.Second}
	for i := 0; i < 30; i++ { // Wait up to 30 seconds
		resp, err := client.Get("http://localhost:8080/metrics")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return
		}
		if err == nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("Gateway did not become healthy in time")
}

func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 1. Save original config
	configPath := "../config.yaml"
	origConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config.yaml: %v", err)
	}
	defer func() {
		os.WriteFile(configPath, origConfig, 0644)
	}()

	// 2. Start docker-compose
	t.Log("Starting docker-compose...")
	runCmd(t, "docker-compose", "down", "-v")
	runCmd(t, "docker-compose", "up", "--build", "-d")
	defer func() {
		t.Log("Tearing down docker-compose...")
		runCmd(t, "docker-compose", "down", "-v")
	}()

	// 3. Wait for gateway
	t.Log("Waiting for gateway to be ready...")
	waitForGateway(t)

	token := generateToken("supersecret")
	client := &http.Client{Timeout: 2 * time.Second}

	doRequest := func(path string) (int, string, string) {
		req, _ := http.NewRequest("GET", "http://localhost:8080"+path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			return 0, "", err.Error()
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header.Get("X-Server-Id"), string(body)
	}

	// Wait an extra second for health checkers to mark backends alive
	time.Sleep(2 * time.Second)

	// 4. Test Routing
	t.Log("Testing Routing...")
	status, serverID, _ := doRequest("/api/orders/123")
	if status != 200 {
		t.Fatalf("expected 200 for /api/orders, got %d", status)
	}
	if serverID != "backend1" && serverID != "backend2" {
		t.Fatalf("expected backend1 or backend2 for /api/orders, got %s", serverID)
	}

	status, serverID, _ = doRequest("/api/users/456")
	if status != 200 {
		t.Fatalf("expected 200 for /api/users, got %d", status)
	}
	if serverID != "backend3" {
		t.Fatalf("expected backend3 for /api/users, got %s", serverID)
	}

	// 5. Test Failover
	t.Log("Testing Failover (Stopping backend1)...")
	runCmd(t, "docker", "stop", "backend1")

	// Wait for health checker to notice it's down (health check interval is 10s in config)
	// Actually, wait 12s
	t.Log("Waiting 12s for health checker to mark backend1 dead...")
	time.Sleep(12 * time.Second)

	// Send 5 requests to /api/orders, they should all go to backend2 now
	for i := 0; i < 5; i++ {
		status, serverID, _ := doRequest("/api/orders/789")
		if status != 200 {
			t.Fatalf("expected 200 after failover, got %d", status)
		}
		if serverID != "backend2" {
			t.Fatalf("failover failed: expected all traffic to go to backend2, got %s", serverID)
		}
	}

	// 6. Test Hot-Reload
	t.Log("Testing Hot-Reload...")
	// Insert the new route right after the /api/users route
	newRoute := `
  - path: /api/new-route
    strip_prefix: true
    strategy: round_robin
    backends:
      - url: http://backend3:9000
        weight: 1`
	newConfig := strings.Replace(string(origConfig), "health_check:", newRoute+"\n\nhealth_check:", 1)

	f, err := os.OpenFile(configPath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatalf("failed to open config.yaml for hot-reload: %v", err)
	}
	if _, err := f.Write([]byte(newConfig)); err != nil {
		t.Fatalf("failed to write new config: %v", err)
	}
	f.Close()

	// Wait for fsnotify to pick it up (debounce is 100ms, but we wait 2s to be safe)
	time.Sleep(2 * time.Second)

	status, serverID, body := doRequest("/api/new-route/hello")
	if status != 200 {
		t.Fatalf("expected 200 for hot-reloaded route, got %d (body: %s)", status, body)
	}
	if serverID != "backend3" {
		t.Fatalf("expected backend3 for new route, got %s", serverID)
	}
	if !strings.Contains(body, "Path: /hello") {
		t.Fatalf("expected StripPrefix to work on new route, body: %s", body)
	}

	t.Log("All Integration Tests Passed!")
}
