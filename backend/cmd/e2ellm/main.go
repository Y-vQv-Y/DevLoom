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
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"status": "ok", "service": "devloom-enterprise-e2e-model"})
	})
	mux.HandleFunc("GET /v1/models", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"object": "list", "data": []map[string]any{{"id": "devloom-enterprise-e2e", "object": "model"}}})
	})
	mux.HandleFunc("POST /v1/chat/completions", completions)
	listen := os.Getenv("DEVLOOM_E2E_LLM_LISTEN")
	if listen == "" {
		listen = ":9999"
	}
	server := &http.Server{Addr: listen, Handler: mux, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second}
	log.Printf("DevLoom enterprise E2E model listening on %s", listen)
	log.Fatal(server.ListenAndServe())
}

func completions(w http.ResponseWriter, r *http.Request) {
	var req request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Tools) == 0 {
		writeJSON(w, completion(message("DevLoom enterprise E2E model is healthy")))
		return
	}
	toolResults := 0
	for _, item := range req.Messages {
		if item.Role == "tool" {
			toolResults++
		}
	}
	writeJSON(w, completion(nextMessage(toolResults)))
}

func nextMessage(toolResults int) map[string]any {
	switch toolResults {
	case 0:
		return toolCall("enterprise-1", "write_file", map[string]any{"path": "backend/server.py", "content": backendSource})
	case 1:
		return toolCall("enterprise-2", "write_file", map[string]any{"path": "frontend/index.html", "content": frontendSource})
	case 2:
		return toolCall("enterprise-3", "write_file", map[string]any{"path": "tests/test_api.py", "content": testSource})
	case 3:
		return toolCall("enterprise-4", "write_file", map[string]any{"path": "tests/acceptance.sh", "content": acceptanceSource})
	case 4:
		return toolCall("enterprise-5", "write_file", map[string]any{"path": "README.md", "content": readmeSource})
	case 5:
		return toolCall("enterprise-6", "run_command", map[string]any{"command": "python3 -m py_compile backend/server.py tests/test_api.py", "timeout_seconds": 60})
	case 6:
		return toolCall("enterprise-7", "run_command", map[string]any{"command": "python3 -m unittest discover -s tests -p 'test_*.py' -v", "timeout_seconds": 120})
	case 7:
		return toolCall("enterprise-8", "run_command", map[string]any{"command": "nohup python3 backend/server.py >/tmp/devloom-enterprise-demo.log 2>&1 & echo $!", "timeout_seconds": 30})
	case 8:
		return toolCall("enterprise-9", "run_command", map[string]any{"command": "bash tests/acceptance.sh", "timeout_seconds": 60})
	case 9:
		return toolCall("enterprise-10", "publish_port", map[string]any{"port": 8000})
	default:
		return message("Enterprise delivery control center completed: persistent API, work-item workflow, metrics, filters, audit history, automated tests, live API acceptance, and preview publication all passed.")
	}
}

func message(content string) map[string]any {
	return map[string]any{"role": "assistant", "content": content}
}

func toolCall(id, name string, arguments map[string]any) map[string]any {
	payload, _ := json.Marshal(arguments)
	return map[string]any{"role": "assistant", "content": nil, "tool_calls": []map[string]any{{"id": id, "type": "function", "function": map[string]any{"name": name, "arguments": string(payload)}}}}
}

func completion(message map[string]any) map[string]any {
	return map[string]any{
		"id":      "chatcmpl-devloom-enterprise-e2e",
		"object":  "chat.completion",
		"choices": []map[string]any{{"index": 0, "finish_reason": map[bool]string{true: "tool_calls", false: "stop"}[message["tool_calls"] != nil], "message": message}},
		"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

const backendSource = `from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlparse
import datetime as dt
import json
import os
import re
import sqlite3

ROOT = Path(__file__).resolve().parents[1]
WEB_ROOT = ROOT / "frontend"
DB_PATH = ROOT / "data" / "delivery.db"
STATUSES = {"planned", "active", "blocked", "review", "done"}
PRIORITIES = {"critical", "high", "medium", "low"}

SEED_ITEMS = [
    ("WI-1001", "Identity federation rollout", "Identity Hub", "active", "critical", "Lin Chen", "2026-08-02", 8),
    ("WI-1002", "Quarterly access review", "Identity Hub", "review", "high", "Maya Patel", "2026-07-30", 5),
    ("WI-1003", "Regional failover rehearsal", "Cloud Control", "planned", "high", "Owen Reed", "2026-08-08", 13),
    ("WI-1004", "Invoice reconciliation rules", "Finance Ops", "blocked", "medium", "Sofia Kim", "2026-07-28", 8),
    ("WI-1005", "Customer export retention", "Data Trust", "done", "medium", "Nora Singh", "2026-07-24", 3),
    ("WI-1006", "SLO alert tuning", "Cloud Control", "active", "low", "Ethan Zhou", "2026-08-05", 5),
]

def utc_now():
    return dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat()

def connect():
    DB_PATH.parent.mkdir(parents=True, exist_ok=True)
    db = sqlite3.connect(DB_PATH, timeout=10)
    db.row_factory = sqlite3.Row
    db.execute("PRAGMA journal_mode=WAL")
    db.execute("PRAGMA foreign_keys=ON")
    return db

def init_db():
    with connect() as db:
        db.executescript("""
        CREATE TABLE IF NOT EXISTS work_items (
          id TEXT PRIMARY KEY, title TEXT NOT NULL, project TEXT NOT NULL,
          status TEXT NOT NULL, priority TEXT NOT NULL, assignee TEXT NOT NULL,
          due_date TEXT NOT NULL, points INTEGER NOT NULL CHECK(points > 0),
          created_at TEXT NOT NULL, updated_at TEXT NOT NULL
        );
        CREATE TABLE IF NOT EXISTS audit_events (
          id INTEGER PRIMARY KEY AUTOINCREMENT, action TEXT NOT NULL,
          entity_id TEXT NOT NULL, actor TEXT NOT NULL, detail TEXT NOT NULL,
          created_at TEXT NOT NULL
        );
        """)
        if db.execute("SELECT COUNT(*) FROM work_items").fetchone()[0] == 0:
            now = utc_now()
            db.executemany("INSERT INTO work_items VALUES (?,?,?,?,?,?,?,?,?,?)", [item + (now, now) for item in SEED_ITEMS])
            db.execute("INSERT INTO audit_events(action,entity_id,actor,detail,created_at) VALUES(?,?,?,?,?)", ("workspace.seeded", "ADTEC-OPS", "system", "Initial enterprise portfolio loaded", now))

def item_dict(row):
    return dict(row) if row else None

def list_items(filters):
    clauses, args = [], []
    for key in ("status", "priority", "project"):
        value = filters.get(key, [""])[0].strip()
        if value:
            clauses.append(key + " = ?")
            args.append(value)
    search = filters.get("q", [""])[0].strip()
    if search:
        clauses.append("(title LIKE ? OR assignee LIKE ? OR id LIKE ?)")
        args.extend(["%" + search + "%"] * 3)
    query = "SELECT * FROM work_items"
    if clauses:
        query += " WHERE " + " AND ".join(clauses)
    query += " ORDER BY CASE priority WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 ELSE 4 END, due_date, id"
    with connect() as db:
        return [dict(row) for row in db.execute(query, args)]

def summary():
    with connect() as db:
        total = db.execute("SELECT COUNT(*) FROM work_items").fetchone()[0]
        done = db.execute("SELECT COUNT(*) FROM work_items WHERE status='done'").fetchone()[0]
        active = db.execute("SELECT COUNT(*) FROM work_items WHERE status IN ('active','review')").fetchone()[0]
        blocked = db.execute("SELECT COUNT(*) FROM work_items WHERE status='blocked'").fetchone()[0]
        points = db.execute("SELECT COALESCE(SUM(points),0) FROM work_items WHERE status='done'").fetchone()[0]
        projects = [dict(row) for row in db.execute("SELECT project, COUNT(*) AS total, SUM(status='done') AS done FROM work_items GROUP BY project ORDER BY project")]
    return {"organization": "ADTEC Digital Operations", "total": total, "active": active, "blocked": blocked, "completed": done, "completion_rate": round(done * 100 / total) if total else 0, "delivered_points": points, "projects": projects}

def validate_item(payload, partial=False):
    allowed = {"title", "project", "status", "priority", "assignee", "due_date", "points"}
    data = {key: payload[key] for key in allowed if key in payload}
    required = allowed if not partial else set()
    missing = [key for key in required if key not in data or str(data[key]).strip() == ""]
    if missing:
        raise ValueError("missing fields: " + ", ".join(sorted(missing)))
    if "status" in data and data["status"] not in STATUSES:
        raise ValueError("invalid status")
    if "priority" in data and data["priority"] not in PRIORITIES:
        raise ValueError("invalid priority")
    if "points" in data:
        data["points"] = int(data["points"])
        if data["points"] < 1 or data["points"] > 100:
            raise ValueError("points must be between 1 and 100")
    if "due_date" in data:
        dt.date.fromisoformat(data["due_date"])
    for key in ("title", "project", "assignee"):
        if key in data:
            data[key] = str(data[key]).strip()[:160]
            if not data[key]:
                raise ValueError(key + " must not be empty")
    return data

def create_item(payload, actor):
    data = validate_item(payload)
    with connect() as db:
        last = db.execute("SELECT COALESCE(MAX(CAST(SUBSTR(id,4) AS INTEGER)),1000) FROM work_items").fetchone()[0]
        item_id, now = "WI-" + str(last + 1), utc_now()
        db.execute("INSERT INTO work_items VALUES (?,?,?,?,?,?,?,?,?,?)", (item_id, data["title"], data["project"], data["status"], data["priority"], data["assignee"], data["due_date"], data["points"], now, now))
        db.execute("INSERT INTO audit_events(action,entity_id,actor,detail,created_at) VALUES(?,?,?,?,?)", ("work_item.created", item_id, actor, data["title"], now))
        return item_dict(db.execute("SELECT * FROM work_items WHERE id=?", (item_id,)).fetchone())

def update_item(item_id, payload, actor):
    data = validate_item(payload, partial=True)
    if not data:
        raise ValueError("no editable fields supplied")
    fields = [key + "=?" for key in data]
    args = list(data.values())
    now = utc_now()
    fields.append("updated_at=?")
    args.extend([now, item_id])
    with connect() as db:
        if db.execute("UPDATE work_items SET " + ",".join(fields) + " WHERE id=?", args).rowcount == 0:
            return None
        db.execute("INSERT INTO audit_events(action,entity_id,actor,detail,created_at) VALUES(?,?,?,?,?)", ("work_item.updated", item_id, actor, json.dumps(data, sort_keys=True), now))
        return item_dict(db.execute("SELECT * FROM work_items WHERE id=?", (item_id,)).fetchone())

class Handler(BaseHTTPRequestHandler):
    server_version = "ADTEC-Delivery-Control/1.0"

    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path == "/healthz":
            return self.send_json({"status": "ok", "database": "sqlite", "service": "delivery-control"})
        if parsed.path == "/api/summary":
            return self.send_json(summary())
        if parsed.path == "/api/work-items":
            return self.send_json({"items": list_items(parse_qs(parsed.query)), "filters": parse_qs(parsed.query)})
        if parsed.path == "/api/audit":
            with connect() as db:
                events = [dict(row) for row in db.execute("SELECT * FROM audit_events ORDER BY id DESC LIMIT 50")]
            return self.send_json({"events": events})
        if parsed.path in ("/", "/index.html"):
            return self.send_file(WEB_ROOT / "index.html", "text/html; charset=utf-8")
        self.send_error(404)

    def do_POST(self):
        if urlparse(self.path).path != "/api/work-items":
            return self.send_error(404)
        try:
            item = create_item(self.read_json(), self.headers.get("X-Actor", "web-user")[:80])
            self.send_json(item, 201)
        except (ValueError, TypeError, json.JSONDecodeError) as error:
            self.send_json({"error": str(error)}, 400)

    def do_PATCH(self):
        match = re.fullmatch(r"/api/work-items/(WI-[0-9]+)", urlparse(self.path).path)
        if not match:
            return self.send_error(404)
        try:
            item = update_item(match.group(1), self.read_json(), self.headers.get("X-Actor", "web-user")[:80])
            if item is None:
                return self.send_json({"error": "work item not found"}, 404)
            self.send_json(item)
        except (ValueError, TypeError, json.JSONDecodeError) as error:
            self.send_json({"error": str(error)}, 400)

    def read_json(self):
        length = int(self.headers.get("Content-Length", "0"))
        if length < 1 or length > 1048576:
            raise ValueError("invalid request size")
        return json.loads(self.rfile.read(length))

    def send_json(self, value, status=200):
        payload = json.dumps(value, ensure_ascii=False).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(payload)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(payload)

    def send_file(self, path, content_type):
        if not path.is_file():
            return self.send_error(404)
        payload = path.read_bytes()
        self.send_response(200)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, format, *args):
        print("delivery-control", self.address_string(), format % args, flush=True)

def create_server(host="0.0.0.0", port=8000):
    init_db()
    return ThreadingHTTPServer((host, port), Handler)

if __name__ == "__main__":
    create_server(port=int(os.environ.get("PORT", "8000"))).serve_forever()
`

const frontendSource = `<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>ADTEC Delivery Control</title>
  <style>
    :root{color-scheme:light;--ink:#20252b;--muted:#667078;--line:#dce1e4;--soft:#f4f6f5;--green:#176b51;--green-soft:#e5f2ed;--amber:#9a6200;--red:#b53939;--white:#fff}*{box-sizing:border-box}body{margin:0;background:var(--soft);color:var(--ink);font-family:Inter,ui-sans-serif,system-ui,sans-serif;letter-spacing:0}.app{min-height:100vh;display:grid;grid-template-columns:224px minmax(0,1fr)}aside{background:#20252b;color:#fff;padding:24px 18px;display:flex;flex-direction:column}.brand{display:flex;align-items:center;gap:10px;font-weight:750;font-size:18px}.brand-mark{display:grid;place-items:center;width:34px;height:34px;background:#27a77a;color:#102a22;font-weight:900}.org{margin:28px 0 18px;color:#aeb7bc;font-size:12px;text-transform:uppercase}.nav{display:grid;gap:4px}.nav span{padding:10px 12px;color:#cbd2d5;font-size:14px}.nav .active{background:#343b40;color:#fff;border-left:3px solid #37b98b}.account{margin-top:auto;border-top:1px solid #3d454b;padding-top:18px;font-size:13px;color:#cbd2d5}.content{min-width:0}.topbar{height:72px;padding:0 32px;background:#fff;border-bottom:1px solid var(--line);display:flex;align-items:center;justify-content:space-between}.topbar strong{font-size:15px}.role{font-size:12px;color:var(--green);background:var(--green-soft);padding:5px 8px}.main{padding:28px 32px 48px;max-width:1440px;margin:auto}.heading{display:flex;align-items:end;justify-content:space-between;gap:20px;margin-bottom:24px}h1{font-size:28px;margin:0 0 4px}.sub{margin:0;color:var(--muted);font-size:14px}.primary{border:0;background:var(--green);color:#fff;height:40px;padding:0 16px;font-weight:650;cursor:pointer}.metrics{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));background:#fff;border:1px solid var(--line);margin-bottom:22px}.metric{padding:18px 20px;border-right:1px solid var(--line)}.metric:last-child{border:0}.metric label{display:block;color:var(--muted);font-size:12px;margin-bottom:8px}.metric strong{font-size:26px}.workspace{background:#fff;border:1px solid var(--line)}.toolbar{padding:14px;border-bottom:1px solid var(--line);display:flex;gap:10px;flex-wrap:wrap}.toolbar input,.toolbar select,.form input,.form select{height:38px;border:1px solid #bbc4c8;background:#fff;padding:0 10px;font:inherit;font-size:13px}.toolbar input{min-width:240px;flex:1}.table-wrap{overflow:auto}table{border-collapse:collapse;width:100%;min-width:900px}th,td{text-align:left;padding:13px 14px;border-bottom:1px solid #e7eaec;font-size:13px}th{color:var(--muted);font-size:11px;text-transform:uppercase;background:#fafbfa}tr:last-child td{border-bottom:0}.id{font-family:ui-monospace,monospace;color:var(--muted)}.priority{font-weight:700}.priority.critical{color:var(--red)}.priority.high{color:var(--amber)}.status{display:inline-block;padding:4px 7px;background:#eef1f2;text-transform:capitalize}.status.active,.status.review{background:var(--green-soft);color:var(--green)}.status.blocked{background:#f8e9e7;color:var(--red)}.status.done{background:#e8eff5;color:#315e7d}.action{border:1px solid #aeb8bd;background:#fff;height:30px;padding:0 9px;cursor:pointer}.lower{display:grid;grid-template-columns:1.2fr .8fr;gap:22px;margin-top:22px}.band{background:#fff;border:1px solid var(--line);padding:18px}.band h2{font-size:15px;margin:0 0 14px}.project-row,.audit-row{display:grid;gap:10px;padding:10px 0;border-bottom:1px solid #edf0f1;font-size:13px}.project-row{grid-template-columns:1fr auto}.project-row:last-child,.audit-row:last-child{border:0}.audit-row small{color:var(--muted)}dialog{width:min(620px,calc(100% - 28px));border:1px solid #aab4b9;padding:0}dialog::backdrop{background:rgba(20,24,26,.55)}.dialog-head{padding:18px 20px;border-bottom:1px solid var(--line);display:flex;justify-content:space-between}.dialog-head h2{font-size:18px;margin:0}.close{border:0;background:none;font-size:22px;cursor:pointer}.form{padding:20px;display:grid;grid-template-columns:1fr 1fr;gap:14px}.field{display:grid;gap:6px}.field label{font-size:12px;color:var(--muted)}.field.wide{grid-column:1/-1}.dialog-actions{grid-column:1/-1;display:flex;justify-content:flex-end;gap:10px;margin-top:8px}.secondary{height:40px;padding:0 14px;background:#fff;border:1px solid #aeb8bd;cursor:pointer}.empty{padding:32px;text-align:center;color:var(--muted)}@media(max-width:900px){.app{grid-template-columns:1fr}aside{display:none}.main,.topbar{padding-left:18px;padding-right:18px}.metrics{grid-template-columns:1fr 1fr}.metric:nth-child(2){border-right:0}.metric:nth-child(-n+2){border-bottom:1px solid var(--line)}.lower{grid-template-columns:1fr}}@media(max-width:560px){.heading{align-items:flex-start;flex-direction:column}.metrics{grid-template-columns:1fr}.metric{border-right:0;border-bottom:1px solid var(--line)!important}.form{grid-template-columns:1fr}.field.wide,.dialog-actions{grid-column:1}.topbar span{display:none}}
  </style>
</head>
<body><div class="app"><aside><div class="brand"><span class="brand-mark">A</span><span>ADTEC DevLoom</span></div><div class="org">Digital Operations</div><div class="nav"><span class="active">Portfolio</span><span>Projects</span><span>Delivery teams</span><span>Risk register</span><span>Audit center</span></div><div class="account">Jordan Lee<br><small>Platform administrator</small></div></aside><section class="content"><header class="topbar"><strong>Delivery Control Center</strong><span><span class="role">ENTERPRISE</span> Production workspace</span></header><main class="main"><div class="heading"><div><h1>Portfolio operations</h1><p class="sub">Delivery health, ownership and execution across the organization.</p></div><button class="primary" id="new-item">Create work item</button></div><section class="metrics" id="metrics"></section><section class="workspace"><div class="toolbar"><input id="search" aria-label="Search" placeholder="Search ID, title or assignee"><select id="status-filter" aria-label="Status"><option value="">All statuses</option><option>planned</option><option>active</option><option>blocked</option><option>review</option><option>done</option></select><select id="priority-filter" aria-label="Priority"><option value="">All priorities</option><option>critical</option><option>high</option><option>medium</option><option>low</option></select><button class="secondary" id="refresh">Refresh</button></div><div class="table-wrap"><table><thead><tr><th>ID</th><th>Work item</th><th>Project</th><th>Owner</th><th>Priority</th><th>Status</th><th>Due</th><th>Action</th></tr></thead><tbody id="items"></tbody></table></div></section><section class="lower"><div class="band"><h2>Project delivery</h2><div id="projects"></div></div><div class="band"><h2>Recent audit events</h2><div id="audit"></div></div></section></main></section></div><dialog id="create-dialog"><div class="dialog-head"><h2>Create work item</h2><button class="close" id="close-dialog" aria-label="Close">x</button></div><form class="form" id="create-form"><div class="field wide"><label>Title</label><input name="title" required value="Enterprise release readiness review"></div><div class="field"><label>Project</label><input name="project" required value="Cloud Control"></div><div class="field"><label>Assignee</label><input name="assignee" required value="Jordan Lee"></div><div class="field"><label>Status</label><select name="status"><option>planned</option><option>active</option><option>blocked</option><option>review</option><option>done</option></select></div><div class="field"><label>Priority</label><select name="priority"><option>critical</option><option selected>high</option><option>medium</option><option>low</option></select></div><div class="field"><label>Due date</label><input type="date" name="due_date" required value="2026-08-15"></div><div class="field"><label>Estimate (points)</label><input type="number" name="points" min="1" max="100" value="8" required></div><div class="dialog-actions"><button type="button" class="secondary" id="cancel-dialog">Cancel</button><button class="primary">Create</button></div></form></dialog><script>
const api=path=>fetch(path).then(async r=>{const body=await r.json();if(!r.ok)throw new Error(body.error||('HTTP '+r.status));return body});
const esc=value=>String(value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
async function loadSummary(){const x=await api('api/summary');document.querySelector('#metrics').innerHTML=[['Total work',x.total],['In delivery',x.active],['Blocked',x.blocked],['Completion',x.completion_rate+'%']].map(v=>'<div class="metric"><label>'+v[0]+'</label><strong>'+v[1]+'</strong></div>').join('');document.querySelector('#projects').innerHTML=x.projects.map(p=>'<div class="project-row"><span>'+esc(p.project)+'</span><strong>'+p.done+' / '+p.total+' done</strong></div>').join('')}
async function loadItems(){const q=new URLSearchParams();const search=document.querySelector('#search').value.trim(),status=document.querySelector('#status-filter').value,priority=document.querySelector('#priority-filter').value;if(search)q.set('q',search);if(status)q.set('status',status);if(priority)q.set('priority',priority);const data=await api('api/work-items?'+q);document.querySelector('#items').innerHTML=data.items.length?data.items.map(x=>'<tr><td class="id">'+esc(x.id)+'</td><td><strong>'+esc(x.title)+'</strong><br><small>'+x.points+' points</small></td><td>'+esc(x.project)+'</td><td>'+esc(x.assignee)+'</td><td class="priority '+esc(x.priority)+'">'+esc(x.priority)+'</td><td><span class="status '+esc(x.status)+'">'+esc(x.status)+'</span></td><td>'+esc(x.due_date)+'</td><td><button class="action" data-id="'+esc(x.id)+'" data-status="'+esc(x.status)+'">Advance</button></td></tr>').join(''):'<tr><td colspan="8" class="empty">No work items match these filters.</td></tr>'}
async function loadAudit(){const data=await api('api/audit');document.querySelector('#audit').innerHTML=data.events.slice(0,5).map(x=>'<div class="audit-row"><span><strong>'+esc(x.action)+'</strong> '+esc(x.entity_id)+'</span><small>'+esc(x.actor)+' / '+esc(x.created_at)+'</small></div>').join('')}
async function refresh(){await Promise.all([loadSummary(),loadItems(),loadAudit()])}
document.querySelector('#refresh').onclick=refresh;document.querySelector('#search').oninput=loadItems;document.querySelector('#status-filter').onchange=loadItems;document.querySelector('#priority-filter').onchange=loadItems;const dialog=document.querySelector('#create-dialog');document.querySelector('#new-item').onclick=()=>dialog.showModal();document.querySelector('#close-dialog').onclick=()=>dialog.close();document.querySelector('#cancel-dialog').onclick=()=>dialog.close();document.querySelector('#create-form').onsubmit=async e=>{e.preventDefault();const data=Object.fromEntries(new FormData(e.target));data.points=Number(data.points);await fetch('api/work-items',{method:'POST',headers:{'Content-Type':'application/json','X-Actor':'Jordan Lee'},body:JSON.stringify(data)}).then(async r=>{if(!r.ok)throw new Error((await r.json()).error)});dialog.close();await refresh()};document.querySelector('#items').onclick=async e=>{const button=e.target.closest('button[data-id]');if(!button)return;const next={planned:'active',active:'review',review:'done',blocked:'active',done:'done'}[button.dataset.status];await fetch('api/work-items/'+button.dataset.id,{method:'PATCH',headers:{'Content-Type':'application/json','X-Actor':'Jordan Lee'},body:JSON.stringify({status:next})}).then(r=>{if(!r.ok)throw new Error('Update failed')});await refresh()};refresh().catch(e=>alert(e.message));
</script></body></html>`

const testSource = `import importlib.util
import json
from pathlib import Path
import tempfile
import threading
import unittest
from urllib import error, request

SPEC = importlib.util.spec_from_file_location("delivery_server", Path(__file__).parents[1] / "backend" / "server.py")
server = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(server)

class DeliveryAPITest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        server.DB_PATH = Path(self.temp.name) / "test.db"
        self.httpd = server.create_server("127.0.0.1", 0)
        self.thread = threading.Thread(target=self.httpd.serve_forever, daemon=True)
        self.thread.start()
        self.base = "http://127.0.0.1:" + str(self.httpd.server_port)

    def tearDown(self):
        self.httpd.shutdown()
        self.httpd.server_close()
        self.thread.join(timeout=2)
        self.temp.cleanup()

    def call(self, method, path, body=None):
        data = json.dumps(body).encode() if body is not None else None
        req = request.Request(self.base + path, data=data, method=method, headers={"Content-Type": "application/json", "X-Actor": "test-suite"})
        with request.urlopen(req, timeout=3) as response:
            return response.status, json.load(response)

    def test_health_summary_and_filter(self):
        status, health = self.call("GET", "/healthz")
        self.assertEqual((status, health["status"]), (200, "ok"))
        _, summary = self.call("GET", "/api/summary")
        self.assertEqual(summary["organization"], "ADTEC Digital Operations")
        self.assertEqual(summary["total"], 6)
        _, result = self.call("GET", "/api/work-items?status=blocked")
        self.assertTrue(result["items"])
        self.assertTrue(all(item["status"] == "blocked" for item in result["items"]))

    def test_create_update_and_audit(self):
        item = {"title": "Enterprise release readiness", "project": "Cloud Control", "status": "planned", "priority": "critical", "assignee": "Jordan Lee", "due_date": "2026-08-15", "points": 8}
        status, created = self.call("POST", "/api/work-items", item)
        self.assertEqual(status, 201)
        _, updated = self.call("PATCH", "/api/work-items/" + created["id"], {"status": "active"})
        self.assertEqual(updated["status"], "active")
        _, audit = self.call("GET", "/api/audit")
        actions = [event["action"] for event in audit["events"]]
        self.assertIn("work_item.created", actions)
        self.assertIn("work_item.updated", actions)

if __name__ == "__main__":
    unittest.main()
`

const acceptanceSource = `#!/usr/bin/env bash
set -Eeuo pipefail
base=http://127.0.0.1:8000
for _ in 1 2 3 4 5 6 7 8 9 10; do
  curl -fsS "$base/healthz" >/dev/null && break
  sleep 1
done
curl -fsS "$base/healthz" | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="ok"'
curl -fsS "$base/api/summary" | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["organization"]=="ADTEC Digital Operations" and x["total"]>=6 and len(x["projects"])>=4'
created="$(curl -fsS -X POST "$base/api/work-items" -H 'Content-Type: application/json' -H 'X-Actor: E2E Acceptance' --data '{"title":"Enterprise release gate","project":"Cloud Control","status":"planned","priority":"critical","assignee":"Jordan Lee","due_date":"2026-08-18","points":13}')"
item_id="$(printf '%s' "$created" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
curl -fsS -X PATCH "$base/api/work-items/$item_id" -H 'Content-Type: application/json' -H 'X-Actor: E2E Acceptance' --data '{"status":"active"}' | python3 -c 'import json,sys; x=json.load(sys.stdin); assert x["status"]=="active" and x["priority"]=="critical"'
curl -fsS "$base/api/work-items?priority=critical" | python3 -c 'import json,sys; x=json.load(sys.stdin); assert len(x["items"])>=2 and all(i["priority"]=="critical" for i in x["items"])'
curl -fsS "$base/api/audit" | python3 -c 'import json,sys; x=json.load(sys.stdin); actions=[e["action"] for e in x["events"]]; assert "work_item.created" in actions and "work_item.updated" in actions'
curl -fsS "$base/" | grep -q 'Delivery Control Center'
printf 'enterprise acceptance passed: %s\n' "$item_id"
`

const readmeSource = `# ADTEC Delivery Control Center

This project is generated by the DevLoom enterprise end-to-end acceptance flow.

## Requirement

Build a zero-dependency, offline-capable delivery operations system for ADTEC. It must provide an executive portfolio summary, project work-item tracking, priority and lifecycle management, search and filtering, persistent SQLite storage, immutable audit events, health checks, a responsive web console, and automated API tests.

## Acceptance

- GET /healthz reports the application and database health.
- GET /api/summary returns cross-project delivery metrics.
- Work items support list, search, filtering, creation and lifecycle updates.
- Every write produces an audit event with actor and timestamp.
- Unit and live HTTP acceptance tests pass before port 8000 is published.

Run with: python3 backend/server.py
`
