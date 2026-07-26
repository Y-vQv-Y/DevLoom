package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var safeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

type relay struct {
	docker string
	prefix string
	logger *slog.Logger
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	listen := env("DEVLOOM_PREVIEW_LISTEN", env("RELAY_LISTEN_HTTP", ":9080"))
	handler := &relay{docker: env("DEVLOOM_DOCKER_BINARY", "docker"), prefix: env("DEVLOOM_VM_CONTAINER_PREFIX", "devloom-vm"), logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/p/", handler)
	server := &http.Server{Addr: listen, Handler: mux, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second}
	logger.Info("preview relay listening", "address", listen)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("preview stopped", "error", err)
		os.Exit(1)
	}
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func (p *relay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	remainder := strings.TrimPrefix(r.URL.Path, "/p/")
	parts := strings.SplitN(remainder, "/", 3)
	if len(parts) < 2 || !safeID.MatchString(parts[0]) {
		http.Error(w, "invalid preview path", http.StatusBadRequest)
		return
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil || port < 1 || port > 65535 {
		http.Error(w, "invalid preview port", http.StatusBadRequest)
		return
	}
	ip, err := p.containerIP(r.Context(), parts[0])
	if err != nil {
		p.logger.Warn("resolve preview target", "vm_id", parts[0], "error", err)
		http.Error(w, "preview target is unavailable", http.StatusBadGateway)
		return
	}
	target, _ := url.Parse(fmt.Sprintf("http://%s:%d", ip, port))
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		p.logger.Warn("preview proxy", "error", err)
		http.Error(w, "preview application is unavailable", http.StatusBadGateway)
	}
	original := proxy.Director
	proxy.Director = func(request *http.Request) {
		original(request)
		if len(parts) == 3 {
			request.URL.Path = "/" + parts[2]
		} else {
			request.URL.Path = "/"
		}
		request.URL.RawPath = ""
		request.Host = target.Host
		request.Header.Set("X-Forwarded-Prefix", fmt.Sprintf("/p/%s/%d", parts[0], port))
	}
	proxy.ServeHTTP(w, r)
}

func (p *relay) containerIP(parent context.Context, vmID string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	name := p.prefix + "-" + strings.ToLower(vmID)
	command := exec.CommandContext(ctx, p.docker, "inspect", "--format", `{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}`, name)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker inspect: %s", strings.TrimSpace(string(output)))
	}
	ip := strings.TrimSpace(string(output))
	if ip == "" {
		return "", fmt.Errorf("container has no network address")
	}
	return ip, nil
}
