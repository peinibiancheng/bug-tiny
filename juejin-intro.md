# 🐛 我写了个单文件零依赖的 Bug 管理系统，把 SQLite 都扔了

## 起源：一个 Bug 管理工具的"反卷"实验

事情是这样的。前段时间在折腾一个小项目，需要一个轻量级的 Bug 跟踪工具。

开一个 Jira？太重型了。装一个 GitHub Issues？项目还没公开。用 SQLite 存？得写迁移脚本，还得操心数据库版本兼容。

于是我做了一个"反卷"的决定：**一个 Bug 就是一个 Markdown 文件，文件系统就是数据库。** 编译产物是一个单文件二进制，零外部依赖。

项目地址：[github/peinibiancheng/bug](https://github.com/peinibiancheng/bug)

## 到底有多轻？

```bash
go build -o bug main.go
```

没别的了。没有 `go mod download`，没有 `npm install`，没有 Docker 镜像。编译出来就是一个独立的可执行文件，复制到任何目录直接跑。

```bash
./bug web
# 打开 http://localhost:8601
```

就这么简单。首次运行自动创建数据目录，不需要配置数据库连接字符串，不需要建表。

## 核心设计理念：KISS 至上

### 文件即数据库

每个 Bug 是一个独立的 Markdown 文件：

```markdown
---
id: 20260427-143052
title: 登录按钮点击无响应
status: open
created_at: 2026-04-27T14:30:00+08:00
updated_at: 2026-04-27T14:30:00+08:00
---

## 描述
点击登录按钮页面无反应，控制台报错 Type Error。
```

YAML 风格的头信息 + Markdown 正文，就这么简单。不用 SQL，不用 ORM，不用迁移脚本。

这样做有几个好处：

- **数据完全透明**：任何时候打开 `bugs/` 目录，一目了然
- **Git 友好**：Bug 记录天然可以纳入版本控制，分支切换时 Bug 列表跟着走
- **编辑器即管理工具**：你可以用 VS Code 全局搜索替换 Bug 内容
- **零运维**：不会出现数据库连不上、表锁死、连接池耗尽这些问题

### 单文件架构

整个后端 785 行代码，全部写在 `main.go` 里，分为五个清晰的层次：

```
1. 数据模型层 — BugMeta 和 Bug 结构体
2. 核心服务层 — BugService（CRUD + 文件读写）
3. Web 层 — HTTP Handler + 路由注册
4. 视图层 — HTML/CSS 模板（常量字符串嵌入）
5. CLI 层 — 命令行子命令分发
```

没用任何第三方库。路由是 Go 1.22+ 原生的 `http.ServeMux`，ORM 不存在（直接读写文件），模板引擎是 `html/template`。

### 内存索引 + 文件直读

启动时扫描 `bugs/` 目录下的所有 `.md` 文件，构建内存索引。列表查询走内存，毫秒响应。

但有个巧妙的点：**写操作直接写文件，读操作也直接读文件**。索引在写操作时同步更新，但 `List()` 方法是每次都重新从文件系统读取的——这是一个有意的设计决策，确保手动编辑 `.md` 文件后，索引不会"过期"。

## 双端交互：Web UI + CLI

### Web 端

启动后打开浏览器就是一个简洁的 Bug 管理面板，支持：

- 列表查看和状态筛选（All / Open / Fixed / Closed）
- Bug 详情展示
- 新建和编辑（支持多图片上传）
- 一键切换状态（Open → Fixed → Closed）
- 删除确认

UI 没有任何框架，纯手写 CSS，全部以内联模板字符串嵌入 Go 代码。

### CLI 端

终端爱好者可以直接在命令行操作：

```bash
bug add                 # 交互式创建 bug
bug list                # 列出所有 bug
bug show <id>           # 查看详情
bug edit <id>           # 调用 $EDITOR 编辑（默认 vim）
bug status <id> fixed   # 快捷改状态
bug delete <id>         # 删除 bug 及关联图片
bug web                 # 启动 Web 服务
```

`bug edit` 的实现很有意思——它直接调用系统编辑器打开对应的 `.md` 文件，因为你编辑的就是原始数据，不需要走 API。

## 并发安全设计

虽然操作的是文件，但 Web 服务会面对并发请求。解决方案是 Go 标准库的 `sync.RWMutex`：

- **读操作**（List、Get）：加读锁，多个 goroutine 可并发读
- **写操作**（Create、Update、Delete）：加写锁，排他访问

写操作先写文件再更新内存索引，两个操作在同一个锁保护下原子执行。

## 附件的处理

图片上传到 `bugs/images/` 目录，命名规则是 `{BugID}-{序号}.{ext}`。

删除 Bug 时，不解析 Markdown 找图片引用，而是直接按文件名前缀匹配清理：

```go
imgs, _ := filepath.Glob(filepath.Join(s.imgDir, id+"-*"))
for _, img := range imgs {
    os.Remove(img)
}
```

简单粗暴，但足够可靠。

## 最佳实践：Per-Project 模式

这个工具的设计哲学是 **每个项目一个实例**，而不是全局部署一个中心化服务。

推荐做法：

1. 把 `bug` 二进制放到 `PATH` 里
2. 在每个代码项目根目录下运行 `bug web`
3. 把 `bugs/` 目录提交到 Git 仓库

这样 Bug 与代码同生同灭。切换 Git 分支时，Bug 状态自然也同步。AI 助手读取项目时，顺手就能拿到所有待办 Bug 的上下文。

## 一些反思

这个项目其实是一个"反主流"的实验。在大家都在往微服务、容器化、重型框架的方向走的时候，我选择了另一个方向——**把事情变简单**。

当然它不是万能的：

- 不支持多用户
- 没有权限控制
- 不适合大规模团队协作
- 搜索依赖文件系统的 glob 匹配

但它解决了一个真实的问题：**当你只是想记录一个 Bug，不想打开 Jira、不想配数据库、不想写 SQL 的时候，有一个工具可以让你十秒内开始记录。**

有时候，最简单的方案就是最好的方案。

---

## 快速开始

```bash
git clone <repo>
cd bug
make linux        # Linux
# 或 make windows  # Windows
./bug web         # 访问 http://localhost:8601
```

去试试吧，也许你会发现——扔掉数据库的感觉，还挺爽的。
