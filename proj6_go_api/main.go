// Go HTTP API - sb_lxc 多阶段构建测试 (与 taskhub 区别: 仅标准库、零外部依赖)
//
// 提供以下端点:
//   GET /          - 欢迎 HTML 页面
//   GET /api/info  - 服务信息 (JSON)
//   GET /api/echo?msg=xxx - 回显消息 (JSON)
//   GET /health    - 健康检查 (JSON)
package main

import (
	"encoding/json"
	"html"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"
)

// 启动时间，用于 uptime 计算
var startTime = time.Now()

// 通过 -ldflags 注入版本 (本 Incusfile 注入 main.version=v1.6.0)
var version = "dev"

// infoResponse 是 /api/info 的 JSON 响应
type infoResponse struct {
	App       string `json:"app"`
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Uptime    string `json:"uptime"`
	Hostname  string `json:"hostname"`
}

// echoResponse 是 /api/echo 的 JSON 响应
type echoResponse struct {
	Message string `json:"message"`
	Time    string `json:"time"`
}

// healthResponse 是 /health 的 JSON 响应
type healthResponse struct {
	Status string `json:"status"`
	Now    string `json:"now"`
}

const welcomeHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Go API · sb_lxc</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI",
                         "PingFang SC", "Microsoft YaHei", sans-serif;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            background: linear-gradient(135deg, #fc466b 0%, #3f5efb 100%);
            color: #fff;
        }
        .card {
            background: rgba(255, 255, 255, 0.08);
            backdrop-filter: blur(10px);
            -webkit-backdrop-filter: blur(10px);
            border: 1px solid rgba(255, 255, 255, 0.15);
            border-radius: 16px;
            padding: 48px 64px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.4);
            text-align: center;
            max-width: 520px;
            width: 90%;
        }
        h1 { font-size: 30px; margin-bottom: 8px; }
        .subtitle { font-size: 14px; color: rgba(255, 255, 255, 0.7); margin-bottom: 32px; }
        .endpoints { text-align: left; margin: 24px 0; display: grid; gap: 8px; }
        .endpoint {
            display: flex; gap: 12px; align-items: center;
            font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
            font-size: 13px; color: rgba(255, 255, 255, 0.9);
            padding: 8px 12px;
            background: rgba(0, 0, 0, 0.15);
            border-radius: 6px;
        }
        .method {
            font-weight: 700; padding: 2px 8px;
            background: rgba(255, 255, 255, 0.2); border-radius: 4px;
        }
        .footer { margin-top: 24px; font-size: 12px; color: rgba(255, 255, 255, 0.5); }
    </style>
</head>
<body>
    <div class="card">
        <h1>Go HTTP API</h1>
        <p class="subtitle">多阶段构建 · 编译型语言 · 零外部依赖</p>
        <div class="endpoints">
            <div class="endpoint"><span class="method">GET</span> /</div>
            <div class="endpoint"><span class="method">GET</span> /api/info</div>
            <div class="endpoint"><span class="method">GET</span> /api/echo?msg=hello</div>
            <div class="endpoint"><span class="method">GET</span> /health</div>
        </div>
        <p class="footer">Powered by Go 1.25 · Debian 13 · Incus</p>
    </div>
</body>
</html>`

// rootHandler 返回欢迎页面
func rootHandler(w http.ResponseWriter, r *http.Request) {
	// 只接受精确 / 路径，避免 404 被劫持
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(welcomeHTML))
}

// infoHandler 返回服务信息 JSON
func infoHandler(w http.ResponseWriter, r *http.Request) {
	host, _ := os.Hostname()
	resp := infoResponse{
		App:       "sb-lxc-go-api",
		Version:   version,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Uptime:    time.Since(startTime).Round(time.Second).String(),
		Hostname:  host,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

// echoHandler 回显请求参数 msg
func echoHandler(w http.ResponseWriter, r *http.Request) {
	msg := r.URL.Query().Get("msg")
	if msg == "" {
		msg = "no message provided"
	}
	resp := echoResponse{
		Message: html.EscapeString(msg),
		Time:    time.Now().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

// healthHandler 健康检查
func healthHandler(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status: "ok",
		Now:    time.Now().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func main() {
	// 路由注册
	mux := http.NewServeMux()
	mux.HandleFunc("/", rootHandler)
	mux.HandleFunc("/api/info", infoHandler)
	mux.HandleFunc("/api/echo", echoHandler)
	mux.HandleFunc("/health", healthHandler)

	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	log.Printf("[sb-lxc-go-api] v=%s listening on %s", version, addr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
