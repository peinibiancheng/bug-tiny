# Bug 管理系统 (Minimalist Bug Tracker)

一个极简、单文件、零外部依赖的 Bug 管理系统，由 Go 语言标准库构建。

## 🌟 核心特性

- **零依赖**：完全基于 Go 标准库（推荐 Go 1.22+），无需安装任何第三方包。
- **单执行文件**：编译后生成一个独立的 `exe` 文件，随处可运行。
- **文件即数据库**：不依赖 SQLite 或 MySQL。每个 Bug 是一个独立的 `.md` Markdown 文件。支持使用 Git 进行版本控制，或用 VS Code 批量检索替换。
- **双端支持**：内置完整的 Web UI 和强大的命令行 (CLI) 交互能力。
- **高性能**：程序启动时构建内存索引，百万次查询毫无压力。

---

## 🛠️ 如何编译

项目中提供了 `Makefile`，交叉编译变得极为简单：

```bash
# 同时编译生成 Linux (bug) 和 Windows (bug.exe) 的可执行文件
make 

# 或者分别编译
make linux
make windows
```

或者你也可以直接使用标准 Go 命令：`go build -o bug main.go`。

---

## 🚀 如何运行

首次运行时，程序会自动在当前目录下创建一个 `bugs/` 文件夹及其子文件夹 `bugs/images/`，用于存储所有的数据。

### 1. 启动 Web 模式

不带参数或带 `web` 参数运行，即可启动 Web 管理后台：

```bash
./bug.exe

# 或者
./bug.exe web
```
启动后，在浏览器中访问 [http://localhost:8601](http://localhost:8601) 即可使用完整的网页版应用。

### 2. 使用 CLI 命令行模式

如果你是终端爱好者，可以直接通过命令行管理 Bug：

```bash
# 交互式添加一个新的 Bug
./bug.exe add

# 查看 Bug 列表
./bug.exe list

# 在终端打印某个 Bug 的完整内容
./bug.exe show <bug-id>

# 调用系统默认的编辑器（如 Vim/VS Code）直接编辑 Markdown 文件
./bug.exe edit <bug-id>

# 快速修改 Bug 的状态 (状态支持：open, fixed, closed)
./bug.exe status <bug-id> fixed

# 彻底删除该 Bug 及其引用的所有本地图片
./bug.exe delete <bug-id>
```

---

## 📂 数据结构说明

你的所有数据都在 `bugs/` 目录下，完全透明：

```text
./bugs/
├── 20260427-143052.md        # Bug 详情 (源文件)
└── images/                   # 图片附件目录
    ├── 20260427-143052-01.png
    └── 20260427-143052-02.png
```

你可以随时将 `bugs/` 目录纳入 Git 仓库，享受完美的代码级 Bug 追踪体验！

---

## 💡 最佳实践建议 (Best Practices)

**强烈建议：采用“每个项目独立运行 (Per-Project)”模式**

我们推荐将本工具作为类似 `git` 的基础工具，**在每个独立的代码项目根目录中分别使用**，而不是全局建立一个仓库集中管理。

**这么做的优势：**
1. **强上下文绑定**：Bug 作为 Markdown 文件保存在项目的 `./bugs/` 目录下，与代码同生同灭。切换 Git 分支、合并代码时，Bug 的状态变更与代码逻辑完全同步。
2. **完美适配 AI Agent**：AI 在读取或重构你的代码仓库时，无需跨越目录或请求外部接口，顺手就能抓取到当前项目的所有待办 Bug 及其图片附件上下文。
3. **极简轻量**：没有多租户/多项目的复杂逻辑。建议将编译好的二进制文件放入系统 `PATH` 环境变量中，随后在任何项目根目录执行 `bug web`，即可立刻拉起该项目的私有 Bug 面板。
