# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

- `make` — cross-compile for Linux + Windows
- `make linux` — build Linux binary (`./bug`)
- `make windows` — build Windows binary (`./bug.exe`)
- `make dev` — quick build for current platform
- `make clean` — remove built binaries
- `go build -o bug main.go` — direct build (no Makefile)
- `./bug web` — start web server on port 8601
- `go vet` — static analysis (zero external deps)

## Architecture

Single-file Go 1.22+ application (`main.go`) with zero external dependencies.

**Storage:** Each bug is a Markdown file in `bugs/` with YAML-style frontmatter (`id`, `title`, `status`, `created_at`, `updated_at`). Images stored in `bugs/images/`. No database. On startup, all `.md` files are parsed into an in-memory index.

**Layers (top to bottom in main.go):**
1. **Models** — `BugMeta` (index entry) and `Bug` (full bug with body)
2. **Service** — `BugService` with `sync.RWMutex` for concurrency. CRUD operations read/write `.md` files directly and update in-memory index atomically.
3. **Web Handlers** — `WebHandler` registers routes on `http.ServeMux`. HTML templates embedded as `const` string. Uses Go 1.22+ method-based routing (e.g. `"GET /{$}"`, `"POST /api/bugs/{id}"`).
4. **CLI** — `runCLI()` dispatches subcommands (add/list/show/edit/status/delete).

**Key patterns:**
- `parseMarkdown()` / `saveMarkdown()` — serialize/deserialize bugs as frontmatter-markdown files
- `appendAttachments()` — handles multipart file uploads, saves to `bugs/images/`
- `renderTemplate()` — parses and executes Go templates with layout + content blocks
- Image deletion uses `filepath.Glob(imgDir, id+"-*")` prefix match (not markdown parsing)
