// Package main implements a minimal HTTP server that drives the APIAudit
// frontend wizard. It serves the compiled frontend and exposes a single
// streaming endpoint that executes the apiaudit CLI and relays its output
// to the browser via Server-Sent Events.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// RunRequest is the JSON body expected by POST /api/run.
type RunRequest struct {
	// Command is the apiaudit sub-command: audit, scan, detect, generate, annotate, init.
	Command string `json:"command"`
	// Dir is the local directory path to analyse.
	Dir string `json:"dir"`
	// Repo is a git repository URL (alternative to Dir).
	Repo string `json:"repo"`
	// FrontendDir overrides the frontend directory discovery.
	FrontendDir string `json:"frontendDir"`
	// Format controls output format; the server always forces --format json.
	Format string `json:"format"`
	// Output is an optional output file path.
	Output string `json:"output"`
	// Flags holds arbitrary string-valued CLI flags (e.g. {"title": "My API"}).
	Flags map[string]string `json:"flags"`
	// BoolFlags lists boolean flags to enable (e.g. ["skip-frontend", "dry-run"]).
	BoolFlags []string `json:"boolFlags"`
	// BeadsLimit sets --beads-limit; 0 means omit the flag (unlimited).
	BeadsLimit int `json:"beadsLimit"`
}

func main() {
	port := flag.Int("port", 8090, "TCP port to listen on (bound to 127.0.0.1)")
	dev := flag.Bool("dev", false, "serve frontend from disk / proxy to Vite dev server instead of embedded FS")
	binPath := flag.String("bin", "", "override path to the apiaudit binary")
	viteURL := flag.String("vite", "http://localhost:5173", "Vite dev server URL (used with --dev)")
	flag.Parse()

	bin, err := resolveBinary(*binPath)
	if err != nil {
		log.Fatalf("apiaudit binary not found: %v", err)
	}
	log.Printf("using apiaudit binary: %s", bin)

	mux := http.NewServeMux()

	// Health check.
	mux.HandleFunc("/api/health", handleHealth)

	// SSE streaming run endpoint.
	mux.HandleFunc("/api/run", func(w http.ResponseWriter, r *http.Request) {
		handleRun(w, r, bin)
	})

	// Static / SPA fallback.
	if *dev {
		mux.Handle("/", devHandler(*viteURL))
	} else {
		spa := &spaHandler{fs: staticHandler()}
		mux.Handle("/", spa)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // disabled: SSE connections are long-lived
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("APIAudit web server listening on http://%s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-stop
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

// handleHealth responds with a simple JSON health check.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok":true}`)
}

// handleRun streams apiaudit output to the client via Server-Sent Events.
func handleRun(w http.ResponseWriter, r *http.Request, bin string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate and normalise the working directory when provided.
	if req.Dir != "" {
		abs, err := filepath.Abs(req.Dir)
		if err != nil {
			http.Error(w, "invalid dir: "+err.Error(), http.StatusBadRequest)
			return
		}
		req.Dir = abs
	}

	argv, err := buildArgv(req)
	if err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Set SSE headers before writing anything.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Use the request context so the child process is cancelled when the
	// client disconnects or the browser tab closes.
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, argv...)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		writeSSEError(w, flusher, "failed to create stdout pipe: "+err.Error())
		return
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		writeSSEError(w, flusher, "failed to create stderr pipe: "+err.Error())
		return
	}

	if err := cmd.Start(); err != nil {
		writeSSEError(w, flusher, "failed to start apiaudit: "+err.Error())
		return
	}

	// Stream stderr lines as log events in real time.
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			writeSSEEvent(w, flusher, "log", line)
		}
	}()

	// Accumulate stdout so we can emit it as a single result event.
	var stdout strings.Builder
	stdoutDone := make(chan struct{})
	go func() {
		defer close(stdoutDone)
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			stdout.WriteString(scanner.Text())
			stdout.WriteByte('\n')
		}
	}()

	<-stdoutDone
	<-stderrDone

	exitCode := 0
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			writeSSEError(w, flusher, "command timed out or was cancelled")
			return
		} else {
			writeSSEError(w, flusher, "command error: "+err.Error())
			return
		}
	}

	// Emit the buffered stdout as the result event.
	if out := strings.TrimSpace(stdout.String()); out != "" {
		writeSSEEvent(w, flusher, "result", out)
	}

	// Emit the terminal done event with exit code.
	donePayload, _ := json.Marshal(map[string]int{"exitCode": exitCode})
	writeSSEEvent(w, flusher, "done", string(donePayload))
}

// buildArgv constructs the argument slice for the apiaudit binary.
func buildArgv(req RunRequest) ([]string, error) {
	if req.Command == "" {
		return nil, errors.New("command is required")
	}

	var args []string
	args = append(args, req.Command)

	if req.Dir != "" {
		args = append(args, "--dir", req.Dir)
	}
	if req.Repo != "" {
		args = append(args, "--repo", req.Repo)
	}
	if req.FrontendDir != "" {
		args = append(args, "--frontend-dir", req.FrontendDir)
	}

	// Always force JSON output so the frontend can parse structured results.
	args = append(args, "--format", "json")

	if req.Output != "" {
		args = append(args, "--output", req.Output)
	}

	for k, v := range req.Flags {
		args = append(args, "--"+k, v)
	}

	for _, bf := range req.BoolFlags {
		args = append(args, "--"+bf)
	}

	if req.BeadsLimit > 0 {
		args = append(args, "--beads-limit", fmt.Sprintf("%d", req.BeadsLimit))
	}

	return args, nil
}

// resolveBinary returns the absolute path to the apiaudit binary. It checks
// the override flag first, then relative candidate paths, then $PATH.
func resolveBinary(override string) (string, error) {
	if override != "" {
		abs, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("resolve override path: %w", err)
		}
		if _, err := os.Stat(abs); err != nil {
			return "", fmt.Errorf("override binary not found at %s: %w", abs, err)
		}
		return abs, nil
	}

	// Relative candidates from the working directory of the web server process.
	candidates := []string{
		"../bin/apiaudit",
		"./bin/apiaudit",
	}
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			return abs, nil
		}
	}

	// Fall back to $PATH.
	p, err := exec.LookPath("apiaudit")
	if err != nil {
		return "", errors.New("apiaudit not found in ../bin, ./bin, or $PATH")
	}
	return p, nil
}

// spaHandler wraps a file server and falls back to index.html for unknown
// paths so that client-side routing works correctly.
type spaHandler struct {
	fs http.Handler
}

func (s *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Let the inner handler attempt the request. If the file does not exist
	// the standard FileServer returns 404 — we intercept that and serve
	// index.html instead so the SPA router can take over.
	rw := &statusRecorder{ResponseWriter: w}
	s.fs.ServeHTTP(rw, r)
	if rw.status == http.StatusNotFound {
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		s.fs.ServeHTTP(w, r2)
	}
}

// statusRecorder captures the status code without writing it to the wire,
// giving the SPA handler a chance to intercept 404s.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	if code != http.StatusNotFound {
		sr.wroteHeader = true
		sr.ResponseWriter.WriteHeader(code)
	}
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if sr.status == http.StatusNotFound {
		// Discard the 404 body; the SPA handler will write its own response.
		return len(b), nil
	}
	if !sr.wroteHeader {
		sr.wroteHeader = true
		sr.ResponseWriter.WriteHeader(sr.status)
	}
	return sr.ResponseWriter.Write(b)
}

// devHandler returns an http.Handler that proxies non-API requests to the
// Vite dev server, falling back to serving from the local disk.
func devHandler(viteRaw string) http.Handler {
	u, err := url.Parse(viteRaw)
	if err != nil || u.Host == "" {
		log.Printf("dev: invalid vite URL %q, serving from frontend/dist on disk", viteRaw)
		return http.FileServer(http.Dir("frontend/dist"))
	}

	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		// Vite not running — serve from disk instead.
		http.FileServer(http.Dir("frontend/dist")).ServeHTTP(w, r)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that the Vite server is reachable before proxying.
		conn, err := net.DialTimeout("tcp", u.Host, 200*time.Millisecond)
		if err != nil {
			http.FileServer(http.Dir("frontend/dist")).ServeHTTP(w, r)
			return
		}
		conn.Close()
		proxy.ServeHTTP(w, r)
	})
}

// writeSSEEvent writes a single Server-Sent Event and flushes immediately.
func writeSSEEvent(w http.ResponseWriter, f http.Flusher, event, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	f.Flush()
}

// writeSSEError writes an SSE error event with a JSON message payload.
func writeSSEError(w http.ResponseWriter, f http.Flusher, msg string) {
	payload, _ := json.Marshal(map[string]string{"message": msg})
	writeSSEEvent(w, f, "error", string(payload))
}
