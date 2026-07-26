package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

type request struct {
	Model    string `json:"model"`
	Messages []struct {
		Role string `json:"role"`
	} `json:"messages"`
	Tools []json.RawMessage `json:"tools"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, map[string]any{"status": "ok"}) })
	mux.HandleFunc("GET /v1/models", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"object": "list", "data": []map[string]any{{"id": "devloom-e2e", "object": "model"}}})
	})
	mux.HandleFunc("POST /v1/chat/completions", completions)
	listen := os.Getenv("DEVLOOM_E2E_LLM_LISTEN")
	if listen == "" {
		listen = ":9999"
	}
	server := &http.Server{Addr: listen, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("DevLoom E2E model listening on %s", listen)
	log.Fatal(server.ListenAndServe())
}

func completions(w http.ResponseWriter, r *http.Request) {
	var req request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Tools) == 0 {
		writeJSON(w, completion(message("E2E model is healthy")))
		return
	}
	toolResults := 0
	for _, item := range req.Messages {
		if item.Role == "tool" {
			toolResults++
		}
	}
	var response map[string]any
	switch toolResults {
	case 0:
		response = toolCall("e2e-1", "write_file", map[string]any{"path": "backend/server.py", "content": backendSource})
	case 1:
		response = toolCall("e2e-2", "write_file", map[string]any{"path": "frontend/index.html", "content": frontendSource})
	case 2:
		response = toolCall("e2e-3", "run_command", map[string]any{"command": "python3 -m py_compile backend/server.py", "timeout_seconds": 60})
	case 3:
		response = toolCall("e2e-4", "run_command", map[string]any{"command": "nohup python3 backend/server.py >/tmp/devloom-demo.log 2>&1 & echo $!", "timeout_seconds": 30})
	case 4:
		response = toolCall("e2e-5", "run_command", map[string]any{"command": "for i in 1 2 3 4 5; do curl -fsS http://127.0.0.1:8000/api/todos && exit 0; sleep 1; done; cat /tmp/devloom-demo.log; exit 1", "timeout_seconds": 30})
	case 5:
		response = toolCall("e2e-6", "publish_port", map[string]any{"port": 8000})
	default:
		response = message("前后端待办应用已经生成并通过 API 验证。后端提供 GET/POST /api/todos，前端支持新增待办和刷新列表，服务已发布到预览端口 8000。")
	}
	writeJSON(w, completion(response))
}

func message(content string) map[string]any {
	return map[string]any{"role": "assistant", "content": content}
}
func toolCall(id, name string, arguments map[string]any) map[string]any {
	payload, _ := json.Marshal(arguments)
	return map[string]any{"role": "assistant", "content": nil, "tool_calls": []map[string]any{{"id": id, "type": "function", "function": map[string]any{"name": name, "arguments": string(payload)}}}}
}
func completion(message map[string]any) map[string]any {
	return map[string]any{"id": "chatcmpl-devloom-e2e", "object": "chat.completion", "choices": []map[string]any{{"index": 0, "finish_reason": map[bool]string{true: "tool_calls", false: "stop"}[message["tool_calls"] != nil], "message": message}}, "usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}}
}
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

const backendSource = `from http.server import ThreadingHTTPServer, SimpleHTTPRequestHandler
import json
import os

TODOS = [{"id": 1, "title": "验证 DevLoom 全链路", "done": True}]

class Handler(SimpleHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/api/todos":
            return self.send_json(TODOS)
        if self.path == "/":
            self.path = "/index.html"
        return super().do_GET()

    def do_POST(self):
        if self.path != "/api/todos":
            return self.send_error(404)
        length = int(self.headers.get("Content-Length", "0"))
        body = json.loads(self.rfile.read(length) or b"{}")
        item = {"id": len(TODOS) + 1, "title": body.get("title", "新待办"), "done": False}
        TODOS.append(item)
        self.send_json(item, 201)

    def send_json(self, value, status=200):
        payload = json.dumps(value, ensure_ascii=False).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

if __name__ == "__main__":
    os.chdir("frontend")
    ThreadingHTTPServer(("0.0.0.0", 8000), Handler).serve_forever()
`

const frontendSource = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>DevLoom 待办工作台</title>
  <style>
    *{box-sizing:border-box}body{margin:0;font-family:system-ui,sans-serif;background:#f4f6f8;color:#18212b}.shell{max-width:760px;margin:48px auto;padding:0 20px}header{display:flex;align-items:end;justify-content:space-between;border-bottom:2px solid #1f6f54;padding-bottom:18px}h1{margin:0;font-size:30px;letter-spacing:0}.badge{color:#1f6f54;font-weight:700}.entry{display:grid;grid-template-columns:1fr auto;gap:10px;margin:24px 0}input,button{height:42px;border:1px solid #bcc6cc;font:inherit}input{padding:0 12px;background:white}button{padding:0 18px;background:#1f6f54;color:white;border-color:#1f6f54;cursor:pointer}.list{background:white;border:1px solid #d9e0e4}.item{display:flex;gap:12px;align-items:center;padding:16px;border-bottom:1px solid #e7ebee}.item:last-child{border:0}.done{text-decoration:line-through;color:#6c7881}.status{margin-top:14px;color:#52616b;font-size:14px}
  </style>
</head>
<body><main class="shell"><header><div><div class="badge">DEVLOOM DEMO</div><h1>待办工作台</h1></div><span>API + Web</span></header><form class="entry" id="form"><input id="title" required placeholder="输入新的待办事项"><button>新增</button></form><section class="list" id="list"></section><div class="status" id="status">正在连接后端...</div></main><script>
const list=document.querySelector('#list'),status=document.querySelector('#status');async function load(){const r=await fetch('api/todos'),data=await r.json();list.innerHTML=data.map(function(x){return '<div class="item"><input type="checkbox" '+(x.done?'checked':'')+' disabled><span class="'+(x.done?'done':'')+'">'+x.title+'</span></div>'}).join('');status.textContent='已连接后端，共 '+data.length+' 条待办'}document.querySelector('#form').onsubmit=async e=>{e.preventDefault();const input=document.querySelector('#title');await fetch('api/todos',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({title:input.value})});input.value='';load()};load();
</script></body></html>`
