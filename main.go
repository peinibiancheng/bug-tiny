package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// =====================================================================
// 1. 数据模型层 (Models)
// =====================================================================

type BugMeta struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"` // open, fixed, closed
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Bug struct {
	BugMeta
	Body string `json:"body"`
}

// =====================================================================
// 2. 核心服务层 (Service / Storage)
// =====================================================================

type BugService struct {
	mu      sync.RWMutex
	baseDir string
	imgDir  string
	index   []*BugMeta
}

func NewBugService(dir string) *BugService {
	return &BugService{
		baseDir: dir,
		imgDir:  filepath.Join(dir, "images"),
	}
}

func (s *BugService) Init() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	os.MkdirAll(s.baseDir, 0755)
	os.MkdirAll(s.imgDir, 0755)

	files, err := filepath.Glob(filepath.Join(s.baseDir, "*.md"))
	if err != nil {
		return err
	}

	s.index = make([]*BugMeta, 0, len(files))
	for _, f := range files {
		bug, err := s.parseMarkdown(f)
		if err == nil {
			s.index = append(s.index, &bug.BugMeta)
		}
	}

	sort.Slice(s.index, func(i, j int) bool {
		return s.index[i].CreatedAt.After(s.index[j].CreatedAt)
	})

	return nil
}

func (s *BugService) List(statusFilter string) []*BugMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files, err := filepath.Glob(filepath.Join(s.baseDir, "*.md"))
	if err != nil {
		return nil
	}

	all := make([]*BugMeta, 0, len(files))
	for _, f := range files {
		bug, err := s.parseMarkdown(f)
		if err == nil {
			all = append(all, &bug.BugMeta)
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	if statusFilter == "" || statusFilter == "all" {
		return all
	}

	var filtered []*BugMeta
	for _, m := range all {
		if m.Status == statusFilter {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func (s *BugService) Get(id string) (*Bug, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.parseMarkdown(filepath.Join(s.baseDir, id+".md"))
}

func (s *BugService) Create(title, body string) (*Bug, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	id := now.Format("20060102-150405")
	bug := &Bug{
		BugMeta: BugMeta{
			ID:        id,
			Title:     title,
			Status:    "open",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body: body,
	}

	if err := s.saveMarkdown(bug); err != nil {
		return nil, err
	}

	s.index = append([]*BugMeta{&bug.BugMeta}, s.index...)
	return bug, nil
}

func (s *BugService) Update(id, title, body, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bug, err := s.parseMarkdown(filepath.Join(s.baseDir, id+".md"))
	if err != nil {
		return err
	}

	bug.Title = title
	bug.Body = body
	if status != "" {
		bug.Status = status
	}
	bug.UpdatedAt = time.Now()

	if err := s.saveMarkdown(bug); err != nil {
		return err
	}

	for _, m := range s.index {
		if m.ID == id {
			m.Title = bug.Title
			m.Status = bug.Status
			m.UpdatedAt = bug.UpdatedAt
			break
		}
	}
	return nil
}

func (s *BugService) UpdateStatus(id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bug, err := s.parseMarkdown(filepath.Join(s.baseDir, id+".md"))
	if err != nil {
		return err
	}

	bug.Status = status
	bug.UpdatedAt = time.Now()

	if err := s.saveMarkdown(bug); err != nil {
		return err
	}

	for _, m := range s.index {
		if m.ID == id {
			m.Status = status
			m.UpdatedAt = bug.UpdatedAt
			break
		}
	}
	return nil
}

func (s *BugService) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	os.Remove(filepath.Join(s.baseDir, id+".md"))

	imgs, _ := filepath.Glob(filepath.Join(s.imgDir, id+"-*"))
	for _, img := range imgs {
		os.Remove(img)
	}

	for i, m := range s.index {
		if m.ID == id {
			s.index = append(s.index[:i], s.index[i+1:]...)
			break
		}
	}
	return nil
}

func (s *BugService) parseMarkdown(path string) (*Bug, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(b)
	bug := &Bug{}
	
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			lines := strings.Split(parts[1], "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				kv := strings.SplitN(line, ":", 2)
				if len(kv) == 2 {
					k := strings.TrimSpace(kv[0])
					v := strings.TrimSpace(kv[1])
					switch k {
					case "id":
						bug.ID = v
					case "title":
						bug.Title = v
					case "status":
						bug.Status = v
					case "created_at":
						bug.CreatedAt, _ = time.Parse(time.RFC3339, v)
					case "updated_at":
						bug.UpdatedAt, _ = time.Parse(time.RFC3339, v)
					}
				}
			}
			bug.Body = strings.TrimSpace(parts[2])
			return bug, nil
		}
	}
	
	return nil, fmt.Errorf("invalid markdown format")
}

func (s *BugService) saveMarkdown(bug *Bug) error {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.WriteString(fmt.Sprintf("id: %s\n", bug.ID))
	buf.WriteString(fmt.Sprintf("title: %s\n", bug.Title))
	buf.WriteString(fmt.Sprintf("status: %s\n", bug.Status))
	buf.WriteString(fmt.Sprintf("created_at: %s\n", bug.CreatedAt.Format(time.RFC3339)))
	buf.WriteString(fmt.Sprintf("updated_at: %s\n", bug.UpdatedAt.Format(time.RFC3339)))
	buf.WriteString("---\n\n")
	buf.WriteString(strings.TrimSpace(bug.Body))
	buf.WriteString("\n")

	path := filepath.Join(s.baseDir, bug.ID+".md")
	return os.WriteFile(path, buf.Bytes(), 0644)
}

// =====================================================================
// 3. Web 层 (HTTP Handlers)
// =====================================================================

type WebHandler struct {
	svc *BugService
}

func (h *WebHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.PageList)
	mux.HandleFunc("GET /bug/{id}", h.PageDetail)
	mux.HandleFunc("GET /new", h.PageNew)
	mux.HandleFunc("GET /edit/{id}", h.PageEdit)

	mux.HandleFunc("POST /api/bugs", h.ApiCreate)
	mux.HandleFunc("POST /api/bugs/{id}", h.ApiUpdate)
	mux.HandleFunc("POST /api/bugs/{id}/status", h.ApiUpdateStatus)
	mux.HandleFunc("POST /api/bugs/{id}/delete", h.ApiDelete)

	mux.Handle("GET /images/", http.StripPrefix("/images/", http.FileServer(http.Dir(h.svc.imgDir))))
}

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	t, err := template.New("web").Parse(tplLayout)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	t, err = t.Parse(tmpl)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	err = t.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func (h *WebHandler) PageList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	bugs := h.svc.List(status)
	renderTemplate(w, tplList, map[string]interface{}{"Bugs": bugs, "Status": status})
}

func (h *WebHandler) PageDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bug, err := h.svc.Get(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	renderTemplate(w, tplDetail, bug)
}

func (h *WebHandler) PageNew(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, tplEdit, nil)
}

func (h *WebHandler) PageEdit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bug, err := h.svc.Get(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	renderTemplate(w, tplEdit, bug)
}

func (h *WebHandler) appendAttachments(r *http.Request, body string) string {
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		files := r.MultipartForm.File["attachments"]
		if len(files) > 0 {
			body += "\n\n### Attachments\n"
		}
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				continue
			}
			
			ext := filepath.Ext(fileHeader.Filename)
			if ext == "" {
				ext = ".png"
			}
			filename := fmt.Sprintf("img_%d%s", time.Now().UnixNano(), ext)
			outPath := filepath.Join(h.svc.imgDir, filename)
			
			out, err := os.Create(outPath)
			if err == nil {
				io.Copy(out, file)
				out.Close()
				body += fmt.Sprintf("![%s](./images/%s)\n", fileHeader.Filename, filename)
			}
			file.Close()
		}
	}
	return body
}

func (h *WebHandler) ApiCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	title := r.FormValue("title")
	body := r.FormValue("body")
	if title == "" {
		http.Error(w, "Title is required", 400)
		return
	}
	
	body = h.appendAttachments(r, body)

	_, err := h.svc.Create(title, body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *WebHandler) ApiUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	title := r.FormValue("title")
	body := r.FormValue("body")
	status := r.FormValue("status")
	
	body = h.appendAttachments(r, body)

	err := h.svc.Update(id, title, body, status)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/bug/"+id, http.StatusSeeOther)
}

func (h *WebHandler) ApiUpdateStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	status := r.FormValue("status")
	h.svc.UpdateStatus(id, status)
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

func (h *WebHandler) ApiDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.svc.Delete(id)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ---------------- HTML/CSS 视图模板 ----------------
const (
	tplLayout = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Bug Tracker</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; color: #333; line-height: 1.6; }
a { color: #0066cc; text-decoration: none; }
a:hover { text-decoration: underline; }
.header { display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid #eee; padding-bottom: 10px; margin-bottom: 20px; }
.btn { display: inline-block; padding: 6px 12px; background: #eee; border-radius: 4px; color: #333; text-decoration: none; border: 1px solid #ccc; cursor: pointer; font-size: 14px;}
.btn-primary { background: #0066cc; color: white; border-color: #0052a3; }
.btn-primary:hover { text-decoration: none; background: #0052a3; }
.badge { padding: 2px 6px; border-radius: 12px; font-size: 12px; color: white; }
.badge-open { background: #dc3545; }
.badge-fixed { background: #28a745; }
.badge-closed { background: #6c757d; }
input[type="text"], select, textarea { width: 100%; padding: 8px; margin: 5px 0 15px; border: 1px solid #ccc; border-radius: 4px; box-sizing: border-box; font-family: inherit;}
</style>
</head>
<body>
<div class="header">
	<h2><a href="/" style="color:inherit;text-decoration:none;">🐛 Bug Tracker</a></h2>
	<div>
		<a href="/new" class="btn btn-primary">New Bug</a>
	</div>
</div>
{{template "content" .}}
</body>
</html>`

	tplList = `{{define "content"}}
<div style="margin-bottom: 20px;">
	Filter: 
	<a href="/">All</a> | 
	<a href="/?status=open">Open</a> | 
	<a href="/?status=fixed">Fixed</a> | 
	<a href="/?status=closed">Closed</a>
</div>
<table style="width:100%; border-collapse: collapse;">
	<thead>
		<tr style="border-bottom: 2px solid #eee; text-align: left;">
			<th style="padding: 10px 0;">ID</th>
			<th>Title</th>
			<th>Status</th>
			<th>Created</th>
		</tr>
	</thead>
	<tbody>
		{{range .Bugs}}
		<tr style="border-bottom: 1px solid #eee;">
			<td style="padding: 10px 0;">{{.ID}}</td>
			<td><a href="/bug/{{.ID}}">{{.Title}}</a></td>
			<td><span class="badge badge-{{.Status}}">{{.Status}}</span></td>
			<td>{{.CreatedAt.Format "2006-01-02 15:04"}}</td>
		</tr>
		{{else}}
		<tr><td colspan="4" style="text-align:center; padding: 20px;">No bugs found.</td></tr>
		{{end}}
	</tbody>
</table>
{{end}}`

	tplDetail = `{{define "content"}}
<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
	<h1 style="margin: 0;">{{.Title}} <span class="badge badge-{{.Status}}" style="font-size: 16px; vertical-align: middle;">{{.Status}}</span></h1>
	<div>
		<a href="/edit/{{.ID}}" class="btn">Edit</a>
		<form method="POST" action="/api/bugs/{{.ID}}/delete" style="display:inline;" onsubmit="return confirm('Delete this bug?');">
			<button type="submit" class="btn" style="color:red;border-color:red;background:white;">Delete</button>
		</form>
	</div>
</div>
<div style="color: #666; font-size: 14px; margin-bottom: 20px;">
	ID: {{.ID}} | Created: {{.CreatedAt.Format "2006-01-02 15:04:05"}} | Updated: {{.UpdatedAt.Format "2006-01-02 15:04:05"}}
</div>

<div style="margin-bottom: 30px;">
	<strong>Change Status:</strong>
	<form method="POST" action="/api/bugs/{{.ID}}/status" style="display:inline;">
		<button type="submit" name="status" value="open" class="btn" {{if eq .Status "open"}}disabled{{end}}>Open</button>
		<button type="submit" name="status" value="fixed" class="btn" {{if eq .Status "fixed"}}disabled{{end}}>Fixed</button>
		<button type="submit" name="status" value="closed" class="btn" {{if eq .Status "closed"}}disabled{{end}}>Closed</button>
	</form>
</div>

<div style="background: #f9f9f9; padding: 20px; border-radius: 8px; border: 1px solid #eee; white-space: pre-wrap; font-family: monospace;">{{.Body}}</div>
{{end}}`

	tplEdit = `{{define "content"}}
<h2>{{if .}}Edit Bug{{else}}New Bug{{end}}</h2>
<form method="POST" action="{{if .}}/api/bugs/{{.ID}}{{else}}/api/bugs{{end}}" enctype="multipart/form-data">
	<label>Title</label>
	<input type="text" name="title" value="{{if .}}{{.Title}}{{end}}" required>
	
	{{if .}}
	<label>Status</label>
	<select name="status">
		<option value="open" {{if eq .Status "open"}}selected{{end}}>Open</option>
		<option value="fixed" {{if eq .Status "fixed"}}selected{{end}}>Fixed</option>
		<option value="closed" {{if eq .Status "closed"}}selected{{end}}>Closed</option>
	</select>
	{{end}}

	<label>Description</label>
	<textarea id="bodyTextarea" name="body" rows="15" required style="font-family: inherit;">{{if .}}{{.Body}}{{end}}</textarea>
	
	<label>Attachments (Optional)</label>
	<input type="file" name="attachments" multiple accept="image/*" style="border: none; padding: 0; margin-bottom: 20px;">
	
	<div>
		<button type="submit" class="btn btn-primary">Save Bug</button>
		<a href="{{if .}}/bug/{{.ID}}{{else}}/{{end}}" class="btn" style="margin-left: 10px;">Cancel</a>
	</div>
</form>
{{end}}`
)

// =====================================================================
// 4. CLI 层 (Command Line Interface)
// =====================================================================

func runCLI(svc *BugService, args []string) {
	if len(args) < 2 {
		printHelp()
		return
	}
	
	switch args[1] {
	case "add":
		cmdAdd(svc)
	case "list":
		cmdList(svc)
	case "show":
		cmdShow(svc, args)
	case "edit":
		cmdEdit(svc, args)
	case "status":
		cmdStatus(svc, args)
	case "delete":
		cmdDelete(svc, args)
	default:
		printHelp()
	}
}

func cmdAdd(svc *BugService) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Bug Title: ")
	title, _ := reader.ReadString('\n')
	title = strings.TrimSpace(title)
	if title == "" {
		fmt.Println("Title cannot be empty")
		return
	}
	
	fmt.Println("Enter description (Press Ctrl+D or type EOF on a new line to finish):")
	var bodyBuilder strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF || strings.TrimSpace(line) == "EOF" {
			break
		}
		bodyBuilder.WriteString(line)
	}
	
	bug, err := svc.Create(title, bodyBuilder.String())
	if err != nil {
		fmt.Printf("Failed to create bug: %v\n", err)
		return
	}
	fmt.Printf("Created Bug %s\n", bug.ID)
}

func cmdList(svc *BugService) {
	bugs := svc.List("")
	if len(bugs) == 0 {
		fmt.Println("No bugs found.")
		return
	}
	fmt.Printf("%-20s | %-10s | %s\n", "ID", "STATUS", "TITLE")
	fmt.Println(strings.Repeat("-", 60))
	for _, b := range bugs {
		fmt.Printf("%-20s | %-10s | %s\n", b.ID, b.Status, b.Title)
	}
}

func cmdShow(svc *BugService, args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: bug show <id>")
		return
	}
	bug, err := svc.Get(args[2])
	if err != nil {
		fmt.Printf("Bug not found: %v\n", err)
		return
	}
	fmt.Printf("ID: %s\n", bug.ID)
	fmt.Printf("Title: %s\n", bug.Title)
	fmt.Printf("Status: %s\n", bug.Status)
	fmt.Printf("Created: %s\n", bug.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated: %s\n", bug.UpdatedAt.Format(time.RFC3339))
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println(bug.Body)
}

func cmdEdit(svc *BugService, args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: bug edit <id>")
		return
	}
	id := args[2]
	path := filepath.Join(svc.baseDir, id+".md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println("Bug not found.")
		return
	}
	
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim" // Default fallback
	}
	
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Editor exited with error: %v\n", err)
		return
	}
	
	svc.Init()
	fmt.Println("Bug updated and index refreshed.")
}

func cmdStatus(svc *BugService, args []string) {
	if len(args) < 4 {
		fmt.Println("Usage: bug status <id> <status>")
		return
	}
	err := svc.UpdateStatus(args[2], args[3])
	if err != nil {
		fmt.Printf("Update failed: %v\n", err)
		return
	}
	fmt.Println("Status updated.")
}

func cmdDelete(svc *BugService, args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: bug delete <id>")
		return
	}
	err := svc.Delete(args[2])
	if err != nil {
		fmt.Printf("Delete failed: %v\n", err)
		return
	}
	fmt.Println("Bug deleted.")
}

func printHelp() {
	fmt.Println(`Bug Tracker — 单文件零依赖 Bug 跟踪器

━━━━ 基本命令 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  bug web                启动 Web 服务器（默认，端口 8601）
  bug list               列出所有 bugs
  bug show <id>          显示 bug 详情
  bug add                交互式添加新 bug
  bug edit <id>          用编辑器修改 bug
  bug status <id> <s>    更新 bug 状态
  bug delete <id>        删除 bug

━━━━ Bug 状态说明 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  状态      含义          使用场景
  ─────     ─────────    ────────────────────
  open      开放的       待处理的缺陷或需求
  fixed     已修复       代码已修复，等待验证
  closed    已关闭       验证通过或放弃

━━━━ Bug ID 格式 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  格式：YYYYMMDD-HHmmss
  示例：20260428-105138
  含义：创建时的日期时间戳

━━━━ 字段说明 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  ID        唯一标识，格式为 YYYYMMDD-HHmmss
  Status    状态：open / fixed / closed
  Title     bug 或需求的简短描述
  Created   创建时间（ISO 8601 格式）
  Updated   最后更新时间

━━━━ 推荐工作流 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  1. 在浏览器或 CLI 创建 bug（状态为 open）
  2. 修复后执行  bug status <id> fixed
  3. 验证通过后执行  bug status <id> closed
  4. 如验证失败，重新打开或创建新 bug

━━━━ 存储结构 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  bugs/          bug 文件目录（.md 格式，YAML 头 + Markdown 正文）
  bugs/images/   附件图片目录

数据存储在本地文件系统中，无需数据库。`)
}

// =====================================================================
// 5. 启动入口
// =====================================================================

func main() {
	svc := NewBugService("./bugs")
	if err := svc.Init(); err != nil {
		fmt.Printf("Init failed: %v\n", err)
		os.Exit(1)
	}

	args := os.Args
	isWebMode := len(args) == 1 || (len(args) == 2 && args[1] == "web")

	if isWebMode {
		port := "8601"
		fmt.Printf("Starting Bug Web Server on http://localhost:%s\n", port)
		
		mux := http.NewServeMux()
		handler := &WebHandler{svc: svc}
		handler.RegisterRoutes(mux)
		
		if err := http.ListenAndServe(":"+port, mux); err != nil {
			fmt.Printf("Server start failed: %v\n", err)
		}
	} else {
		runCLI(svc, args)
	}
}
