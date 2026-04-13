# Workspace 架构重构文档

## 概述

本次重构将 Cobot 从一个单一全局 workspace 的架构改为支持多 workspace 的架构，实现了 workspace 级别的资源隔离。

## 架构变化

### 旧架构

```
~/.config/cobot/              # 全局配置
├── config.yaml               # 全局配置
├── SOUL.md                   # 全局 bot 人格
├── USER.md                   # 全局用户配置
└── MEMORY.md                 # 全局记忆

~/.local/share/cobot/         # 全局数据
├── memory/                   # 全局 memory
├── sessions/                 # 全局 sessions
└── skills/                   # 全局 skills

项目目录/
└── .cobot/                   # 项目级配置
    ├── config.yaml
    └── AGENTS.md
```

### 新架构

```
~/.config/cobot/                      # 全局配置
├── config.yaml                       # 全局配置（API keys 等）
└── workspaces/                       # 所有 workspaces
    ├── manager.yaml                  # workspace 管理器配置（当前 workspace）
    ├── default/                      # 默认 workspace
    │   ├── workspace.yaml            # workspace 配置
    │   ├── SOUL.md                   # workspace 级别 SOUL
    │   ├── USER.md                   # workspace 级别 USER
    │   └── MEMORY.md                 # workspace 级别 MEMORY
    └── <other-workspaces>/           # 其他 workspaces
        └── ...

~/.local/share/cobot/                 # 数据目录
└── workspaces/                       # workspace 级别数据
    ├── default/                      # default workspace 数据
    │   ├── memory/                   # 隔离的 memory 存储
    │   ├── sessions/                 # 隔离的 sessions
    │   └── skills/                   # 隔离的 skills
    └── <other-workspaces>/
        └── ...
```

## 核心改动

### 1. Workspace 结构体重构 (`internal/workspace/workspace.go`)

新增了 `Workspace` 结构体的字段：

```go
type Workspace struct {
    ID          string        // 唯一标识符
    Name        string        // 显示名称
    Type        WorkspaceType // default/project/custom
    Root        string        // 项目目录（project 类型）
    ConfigPath  string        // workspace.yaml 路径
    ConfigDir   string        // 配置目录
    DataDir     string        // 数据目录
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

新增方法：
- `GetSoulPath()`, `GetUserPath()`, `GetMemoryMdPath()` - 获取 persona 文件路径
- `MemoryDir()`, `SessionsDir()`, `SkillsDir()` - 获取数据目录
- `ValidatePathWithinWorkspace()` - 路径权限验证

### 2. Workspace Manager (`internal/workspace/manager.go`)

新增的 `Manager` 类型管理所有 workspaces：

```go
type Manager struct {
    workspacesDir      string
    workspacesDataDir  string
    current            *Workspace
}
```

提供的方法：
- `Create()` - 创建自定义 workspace
- `CreateProject()` - 从项目目录创建 workspace
- `List()` - 列出所有 workspaces
- `GetByID()`, `GetByName()` - 获取 workspace
- `Switch()` - 切换当前 workspace
- `Delete()` - 删除 workspace
- `Rename()` - 重命名 workspace

### 3. Discovery 逻辑更新 (`internal/workspace/discovery.go`)

- `Discover()` - 从项目目录发现 workspace，自动创建 project workspace
- `DiscoverOrDefault()` - 发现失败返回 default workspace
- `DiscoverOrCurrent()` - 发现失败返回当前 workspace

### 4. Persona 服务重构 (`internal/persona/persona.go`)

新增 `Service` 类型，支持 workspace 级别的 persona：

```go
type Service struct {
    ws *workspace.Workspace
}
```

原有 `Persona` 类型保留用于向后兼容。

### 5. CLI 命令更新 (`cmd/cobot/workspace.go`)

新增 workspace 管理命令：

```bash
cobot workspace list              # 列出所有 workspaces
cobot workspace create <name>     # 创建自定义 workspace
cobot workspace project [path]    # 从项目目录创建 workspace
cobot workspace switch <name>     # 切换 workspace
cobot workspace current           # 显示当前 workspace
cobot workspace rename <old> <new># 重命名 workspace
cobot workspace delete <name> --force  # 删除 workspace
```

### 6. Memory 命令更新 (`cmd/cobot/memory_cmd.go`)

Memory 命令现在使用当前 workspace 的 memory 目录：

```bash
cobot memory search "query"       # 在当前 workspace memory 中搜索
cobot memory store ...            # 存储到当前 workspace memory
cobot memory status               # 显示当前 workspace memory 状态
```

### 7. Persona 命令更新 (`cmd/cobot/persona_cmd.go`)

Persona 命令现在操作当前 workspace 的 persona 文件：

```bash
cobot persona init                # 初始化当前 workspace persona
cobot persona edit [soul|user|memory]  # 编辑当前 workspace persona
cobot persona show [soul|user|memory]  # 显示当前 workspace persona
```

### 8. 配置命令更新 (`cmd/cobot/config_cmd.go`)

所有配置命令现在操作当前 workspace 的配置：

```bash
cobot config init                 # 初始化当前 workspace 配置
cobot config edit                 # 编辑当前 workspace 配置
cobot config set ...              # 设置当前 workspace 配置
```

## 权限控制

`Workspace.ValidatePathWithinWorkspace()` 方法确保 agent 只能访问当前 workspace 的文件：

```go
func (w *Workspace) ValidatePathWithinWorkspace(path string) error {
    // 验证路径是否在以下范围内：
    // 1. workspace 数据目录内
    // 2. workspace 配置目录内
    // 3. project workspace 的项目根目录内
}
```

## 向后兼容

保留了以下函数用于向后兼容：

```go
// 这些函数现在都指向 default workspace
func GlobalWorkspace() *Workspace
func GlobalMemoryDir() string
func GlobalSessionsDir() string
func GlobalSoulPath() string
func GlobalUserPath() string
func GlobalMemoryMdPath() string
func EnsureGlobalWorkspace() error
```

## 迁移指南

1. 首次运行新版本的 cobot 时，会自动创建 `default` workspace
2. 原有的全局 SOUL.md/USER.md/MEMORY.md 会被移动到 `~/.config/cobot/workspaces/default/`
3. 原有的全局 memory 数据会被移动到 `~/.local/share/cobot/workspaces/default/memory/`

## 使用示例

```bash
# 查看当前 workspace
cobot workspace current

# 创建新的 workspace
cobot workspace create my-project

# 切换到 workspace
cobot workspace switch my-project

# 在项目目录创建 workspace
cobot workspace project ~/projects/my-app

# 不同 workspace 有独立的 memory
cobot workspace switch default
cobot memory search "query"  # 搜索 default workspace memory

cobot workspace switch my-project
cobot memory search "query"  # 搜索 my-project workspace memory
```

## 测试

所有 workspace 测试通过：

```bash
go test ./internal/workspace/... -v
```

测试覆盖：
- Workspace 发现逻辑
- Workspace 路径验证
- Default workspace 回退
