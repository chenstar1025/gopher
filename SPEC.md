# SPEC: Gopher — A Coding Agent Harness

> 项目方向：A · Coding Agent Harness

---

## 1. 问题陈述

### 1.1 要解决的问题

LLM 可以输出代码建议，但它不能自主完成完整的编码循环——读取文件、写入修改、运行测试、根据失败信号自我修正。Gopher 是一层工程基础设施（harness），将 LLM 从"只能说话的大模型"封装成"能进入真实编码循环、根据客观反馈自我修正的自动化工具"。

### 1.2 目标用户

- **开发者**：希望在本地终端中获得一个自主编码助手，能读代码、写代码、跑测试、自我修复。
- **课程评审者**：本项目同时也是 AI4SE 课程的期末交付物，须满足课程对 harness 的所有要求。

### 1.3 为什么值得做

当前市面上的 coding agent 大多是在 IDE 插件或 SaaS 中运行，其内部循环、治理、反馈机制对用户不透明。Gopher 从零编码实现这些机制，让使用者——以及评审者——能看清 harness 每一层的工作原理。它不依赖 LangChain、AutoGen 等框架的 agent runner，而是用 Go 代码直接组装：主循环 + LLM 抽象 + 工具分发 + 护栏 + 反馈闭环。

---

## 2. 用户故事

| # | 作为… | 我想要… | 以便… | 验收标准 |
|---|-------|---------|-------|----------|
| US1 | 开发者 | 在终端输入一个自然语言任务描述 | Gopher 能理解任务、规划步骤并开始执行 | 输入任务后，agent 开始按步骤操作文件/执行命令 |
| US2 | 开发者 | Gopher 每次修改代码后自动运行测试 | 我不用手动验证每次改动是否正确 | 每轮修改后测试结果自动回显，红色/绿色状态清晰 |
| US3 | 开发者 | 测试失败时 Gopher 自动分析失败原因并尝试修复 | 我不需要手把手告诉它怎么改 | 单次失败 ≤3 轮自动修复；若仍失败则暂停并报告 |
| US4 | 开发者 | Gopher 在执行危险命令前暂停并请求我确认 | 我的项目不会被意外破坏 | `rm -rf`、`git push --force` 等被拦截，需输入 yes |
| US5 | 开发者 | 首次使用时安全地录入我的 LLM API Key | Key 不会以明文出现在任何文件中 | Key 存入系统凭据管理器，查看状态时不回显明文 |
| US6 | 开发者 | 将 Gopher 的二进制文件下载到一台新机器后能立刻跑起来 | 分发体验流畅 | 单文件二进制 + 首次运行配置 Key 向导，无需额外安装依赖 |
| US7 | 评审者 | 用 mock LLM 运行 Gopher 的所有核心机制 | 验证护栏、反馈闭环等机制的确定性行为 | `go test ./...` 全绿，不依赖网络与真实 LLM |

---

## 3. 功能规约

### 3.1 模块总览

```
┌──────────────────────────────────────────┐
│               CLI 入口 (cmd/gopher)        │
├──────────────────────────────────────────┤
│  主循环 (internal/loop)                    │
├──────────┬──────────┬──────────┬─────────┤
│ LLM 抽象 │ 工具系统 │ 反馈闭环 │ 治理护栏 │
│ (llm/)   │ (tools/) │(feedback/)│(guard/) │
├──────────┴──────────┴──────────┴─────────┤
│         凭据管理 (internal/credential)      │
│         记忆系统 (internal/memory)          │
└──────────────────────────────────────────┘
```

### 3.2 Agent 主循环 (`internal/loop`)

**输入**：用户任务（自然语言字符串）、工作目录路径。

**行为**：
1. 构建上下文（系统提示 + 记忆 + 当前工具列表 + 对话历史）
2. 调用 LLM 获取下一步动作
3. 解析 LLM 返回的动作（工具调用或纯文本回复）
4. 若为工具调用 → 护栏检查 → 分发执行 → 收集结果 → 回灌给 LLM
5. 若有测试类工具调用 → 反馈闭环解析结果 → 回灌
6. 判断停机条件（LLM 回复不含工具调用、达到最大轮次、或用户中断）
7. 若未停机 → 回到步骤 1

**边界条件**：
- 最大轮次默认 50，可配置
- 若连续 3 轮 LLM 返回相同动作且均失败，停机并报告死循环

**错误处理**：
- LLM 调用超时（默认 120s）→ 重试 2 次 → 仍失败则停机
- LLM 返回无法解析的动作 → 将原始响应回灌 + 要求重新输出

### 3.3 LLM 抽象层 (`internal/llm`)

**接口**：
```go
type LLM interface {
    Chat(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error)
}
```

**实现**：
- `openai`：对接 OpenAI 兼容 API（含 Anthropic、DeepSeek 等通过兼容网关）
- `mock`：确定性 mock，按预设脚本返回

**输入**：消息列表 + 可用工具定义。
**输出**：`Response{Messages []Message, ToolCalls []ToolCall, FinishReason string}`。

### 3.4 工具系统 (`internal/tools`)

**支持的工具体积（基础实现）**：
| 工具 | 功能 | 权限级别 |
|------|------|----------|
| `read_file` | 读取指定文件内容 | safe |
| `write_file` | 写入/覆盖文件 | safe |
| `list_dir` | 列出目录内容 | safe |
| `run_shell` | 执行 shell 命令 | dangerous |
| `run_test` | 执行 `go test ./...` 并返回结构化结果 | safe |

**工具注册**：通过 `ToolRegistry` 接口注册，支持动态添加。

**输入**：工具名 + JSON 参数。
**输出**：`ToolResult{Success bool, Output string, Error string}`。

### 3.5 反馈闭环 (`internal/feedback`) — 重点维度

**核心机制**（全部为确定性代码，不依赖 LLM 判断）：

1. **测试结果解析器**：解析 `go test -json` 输出，提取失败的测试名、文件、行号、错误信息。
2. **失败分类器**：
   - `compile_error` → 编译不通过，无法运行
   - `test_failure` → 测试断言失败
   - `timeout` → 测试超时
   - `panic` → 运行时 panic
3. **反馈构造器**：将解析结果构造为结构化的 `Feedback` 对象，注入下一轮对话。
4. **修正轮次追踪**：记录同一 task 的修正轮数，超过 3 轮仍失败 → 停机并报告。

**反馈对象结构**：
```go
type Feedback struct {
    Type       FeedbackType // compile_error | test_failure | timeout | panic | success
    Failures   []TestFailure
    Summary    string       // 供 LLM 阅读的自然语言摘要
}

type TestFailure struct {
    TestName string
    File     string
    Line     int
    Message  string
}
```

### 3.6 治理护栏 (`internal/guard`)

**危险动作定义**（匹配以下模式的 shell 命令为危险）：
- `rm -rf` / `rm -r` / `rmdir` 等删除目录
- `git push --force` / `git reset --hard` / `git clean`
- `DROP` / `DELETE` 等数据库破坏操作
- `> /dev/` 写入设备文件
- `chmod 777` 等权限过度放开
- `curl | bash` / `wget -O - | sh` 等管道执行

**护栏逻辑**（确定性代码）：
```go
func (g *Guardrail) Check(action ToolCall) (Blocked bool, Reason string)
```
- 若命中危险模式 → 返回 `Blocked=true` + 原因 → 暂停 → 等待用户确认（`yes`/`no`）
- 若用户输入 `yes` → 放行
- 若用户输入 `no` 或 60s 超时 → 拒绝执行

**白名单**：用户可通过配置文件将特定命令加入白名单。匹配规则为**前缀匹配**（白名单 `git` 则所有 `git` 命令放行，包括 `git push --force`）。

### 3.7 记忆系统 (`internal/memory`)

**基础实现**：
- **项目约定文件**（`.gopher/rules.md`）：用户可编辑，LLM 每轮上下文均注入。
- **对话摘要**：每 10 轮触发一次摘要，旧消息折叠为摘要注入后续上下文。
- **存储后端**：本地 JSON 文件（`.gopher/memory.json`）。

### 3.8 凭据管理 (`internal/credential`)

- **存储**：Windows Credential Manager（通过 `golang.org/x/sys/windows`），macOS Keychain / Linux Secret Service 作为后续扩展。
- **首次运行向导**：
  1. 检测凭据是否存在
  2. 若不存在 → 提示输入 API Key（隐藏回显）
  3. 写入系统凭据管理器
- **查看状态**：`gopher status` → 显示 Key 是否已配置 + 最后 4 位（不回显完整 Key）
- **更新**：`gopher config set-key` → 覆盖旧凭据
- **清除**：`gopher config clear-key` → 删除凭据

---

## 4. 非功能性需求

### 4.1 性能
- 主循环单轮延迟（不含 LLM 调用）< 100ms
- mock 模式下完整三轮循环 < 1s

### 4.2 安全
- API Key 绝不写入文件、日志、终端
- 凭据威胁模型（见 §7.1）
- 护栏拦截所有危险命令，无例外
- 项目通过 `go vet` 和 `gosec` 扫描，零高危告警

### 4.3 可用性
- 单文件二进制，无需安装运行时
- 首次运行自动引导配置
- `--help` 输出完整命令参考

### 4.4 可观测性
- 每轮打印：`[Round N] LLM → tool: X, args: {...}`（不含凭据）
- `--verbose` 模式打印完整上下文
- `AGENT_LOG.md` 风格的结构化日志写入 `.gopher/logs/`

---

## 5. 系统架构

### 5.1 组件图

```
         ┌──────────────────────┐
         │    CLI (cmd/gopher)   │
         │  run | status | config│
         └──────────┬───────────┘
                    │
         ┌──────────▼───────────┐
         │     主循环 (loop)     │
         │  ┌────────────────┐  │
         │  │ 上下文组装      │  │
         │  │ LLM 调用 → 解析 │  │
         │  │ 护栏 → 分发     │  │
         │  │ 反馈 → 回灌     │  │
         │  │ 停机判断        │  │
         │  └────────────────┘  │
         └──┬──────┬──────┬─────┘
            │      │      │
    ┌───────▼┐ ┌──▼──┐ ┌─▼──────┐
    │ LLM    │ │工具  │ │反馈闭环 │
    │ 抽象层  │ │系统  │ │(重点)   │
    └───┬────┘ └─────┘ └────────┘
        │
  ┌─────┴──────┐
  │ OpenAI API │  │ Mock (测试用)
  │ (真实LLM)  │  │
  └────────────┘  └────────────┘
```

### 5.2 数据流（以"修复编译错误"为例）

```
User Task: "修复 main.go 中的编译错误"
  │
  ▼
[1] 构建上下文 → 调 LLM
  │
  ▼
[2] LLM: tool=read_file, args={"path":"main.go"}
  │ → 护栏检查 (safe) → 执行 → 返回文件内容
  ▼
[3] LLM: tool=write_file, args={"path":"main.go","content":"..."}
  │ → 护栏检查 (safe) → 执行
  ▼
[4] LLM: tool=run_test, args={}
  │ → 执行 go test -json → 反馈闭环解析
  │ → 结果: compile_error @ main.go:10: missing )
  ▼
[5] Feedback 注入 → LLM 收到: "编译失败: main.go:10 缺少 )"
  │
  ▼
[6] LLM: tool=write_file ... (修正) → run_test → 通过 ✓
  │
  ▼
[7] 停机: task 完成
```

### 5.3 外部依赖
- **LLM 供应商**：OpenAI 兼容 API（用户自备 Key，默认指向 `api.openai.com`，可配置 endpoint）
- **凭据存储**：操作系统原生凭据管理器
- **Go 标准库** + 少量社区库（见表 `§8.1`）

---

## 6. 数据模型

### 6.1 消息
```go
type Message struct {
    Role      string     // system | user | assistant | tool
    Content   string
    ToolCalls  []ToolCall
    ToolCallID string
}
```

### 6.2 工具调用
```go
type ToolCall struct {
    ID       string
    Name     string
    Args     map[string]any // key 约定：read_file/write_file 用 "path"；run_shell 用 "command"
}

type ToolResult struct {
    ToolCallID string
    Success    bool
    Output     string
    Error      string
}
```

### 6.3 反馈（重点维度）
```go
type Feedback struct {
    Type     FeedbackType
    Failures []TestFailure
    Summary  string
}

type TestFailure struct {
    TestName string
    File     string
    Line     int
    Message  string
}
```

### 6.4 对话状态
```go
type Session struct {
    ID        string
    Messages  []Message
    Round     int
    Status    SessionStatus // running | waiting_approval | done | failed
    CreatedAt time.Time
}
```

### 6.5 配置
```go
type Config struct {
    LLMEndpoint   string // LLM API 端点，默认 https://api.openai.com/v1
    LLMModel      string // 模型名，默认 gpt-4o
    MaxRounds     int    // 最大轮次，默认 50
    MaxRetries    int    // 反馈修正最大轮数，默认 3
    WorkDir       string // 工作目录，默认当前目录
    WhitelistCmds []string // 信任的命令，跳过护栏
}
```

---

## 7. 凭据与分发设计

### 7.1 凭据威胁模型

| 威胁 | 对策 |
|------|------|
| API Key 被提交到 Git | Key 存在系统凭据管理器，源码中无任何 hardcode 路径；`.gitignore` 已排 `.env`、`.gopher/keys/` |
| 终端日志泄露 Key | 凭据相关命令全程隐藏回显；verbose 日志自动脱敏 `sk-***` 模式 |
| 进程环境变量泄露 | 不使用 `export` 方式传 Key；凭据仅在调用 LLM 时从凭据管理器读取到内存，用完即释放 |
| 内存 dump 泄露 | 凭据在内存中以 `SecureString` 持有，GC 前主动清零 |

### 7.2 首次运行 Key 配置流程

```
$ gopher run "修复 main.go 的 bug"
  ⚠ No API key found.
  Enter your OpenAI API key: [****隐藏输入****]
  Key saved to Windows Credential Manager.
  Starting Gopher...
```

### 7.3 分发

- **形态**：单文件原生二进制
- **平台**：Windows (amd64)、Linux (amd64)、macOS (amd64 + arm64)
- **获取方式**：`go install github.com/chenstar1025/gopher@latest` 或 GitHub Releases 下载预编译二进制
- **Key 配置**：首次运行自动弹出配置向导
- **已知限制**：凭据管理器当前仅支持 Windows（`wincred`），macOS / Linux 使用明文配置文件作为降级方案（标有明确风险警告）。Linux 后续版本接入 `libsecret`。

---

## 8. 技术选型与理由

### 8.1 语言与核心依赖

| 类别 | 选择 | 理由 |
|------|------|------|
| 语言 | Go 1.22+ | 单文件编译分发、内置测试框架、CLI 库成熟、跨平台 |
| LLM SDK | 自写 HTTP 客户端 + `encoding/json` | OpenAI 兼容 API 足够简单，不引入重量级 SDK |
| 凭据存储 | `github.com/danieljoos/wincred` | Windows Credential Manager 的 Go 绑定 |
| CLI 框架 | 标准库 `flag` + 手写命令路由 | 依赖最少，编译快 |
| 测试 | `testing` + `go test -json` | 标准库，无需第三方 |
| CI | GitHub Actions | 免费、仓库自带 |

### 8.2 不使用现成 Agent 框架

本项目**不依赖** LangChain、AutoGen、CrewAI、LlamaIndex 或任何编码智能体 SDK 的 agent runner。主循环、工具分发、护栏、反馈闭环全部由 Go 代码直接实现。LLM 抽象层的唯一职责是发送 HTTP 请求拿到回复，不做任何 agent 层面的编排。

---

## 9. 验收标准

| 功能模块 | 验收标准 |
|----------|----------|
| 主循环 | 在 mock LLM 下完成 ≥3 轮"任务→工具调用→回灌→停机"循环 |
| LLM 抽象 | 同一接口下，真实 OpenAI API 和 mock 均可正常工作 |
| 工具系统 | 5 个工具全部可被 agent 正确调用并返回结果 |
| 反馈闭环 | 注入一次测试失败后，agent 在 3 轮内修改文件并重试；mock 下可确定性地验证 |
| 治理护栏 | `rm -rf /` 被拦截；`yes` 放行，`no` 拒绝；mock 下可确定性地验证 |
| 凭据管理 | Key 录入/查看/清除 三个子命令均可正常工作；查看时明文不出现 |
| 分发 | `go build -o gopher .` 产出单文件二进制；Windows 下可直接运行 |
| Mock 测试 | `go test ./...` 全绿，不访问网络 |
| CI | GitHub Actions `unit-test` job pass |

---

## 10. 风险与未决问题

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| LLM 返回格式不稳定 | 动作解析失败，循环中断 | 解析失败时回灌原始响应 + 要求重新输出；最多重试 2 次 |
| 反馈闭环误判 | 将非错误输出识别为测试失败 | 严格依赖 `go test -json` 的结构化字段，不做启发式判断 |
| 跨平台凭据管理 | Linux/macOS 版本使用明文降级方案 | README 明确标注风险；后续版本接入 OS 原生 API |
| subagent 在复杂修复中偏离主题 | 越修越错 | 反馈闭环的 3 轮上限 + 死循环检测 |
| mock LLM 过于简单无法覆盖真实场景 | 真实 LLM 下行为与 mock 测试不一致 | mock 设"正常"/"错误"/"边界"三种脚本，覆盖主要分支 |

---

## 11. 领域与机制设计（A 类额外要求）

### 11.1 Coding 领域的四类机制

**动作 / 工具**：文件读写、shell 执行、测试运行。Coding 领域天然需要这些操作，不能只靠聊天。

**客观反馈信号**：
- `go test -json` → 编译错误 / 测试失败 → 客观、确定、可解析
- `go vet` → 静态分析告警
- 文件 diff → 改动是否真的发生了

**危险动作**：删除文件/目录、force push、数据库破坏、管道执行远程脚本。

**记忆**：项目 `.gopher/rules.md`（约定）、对话摘要（长对话压缩）。

### 11.2 重点维度：反馈闭环

**为什么选它**：Coding agent 区别于聊天机器人的唯一标志是"能根据测试结果自我修正"。反馈闭环是这条循环的物理实现，且天然由确定性代码组成（解析测试输出 → 分类失败 → 构造结构化反馈），最容易满足"移除 LLM 后仍可单测验证"的硬要求。

**编码实现策略**：
1. `internal/feedback/parser.go` — 解析 `go test -json` 输出，纯数据转换，零 LLM 依赖
2. `internal/feedback/classifier.go` — 根据解析结果分类（`compile_error` / `test_failure` / `timeout` / `panic`），纯 switch 逻辑
3. `internal/feedback/injector.go` — 将 `Feedback` 对象序列化为 ToolResult 注入上下文
4. `internal/feedback/tracker.go` — 追踪同 task 修正轮数，超过上限停机

**mock 测试策略**：
- 构造假 JSON（模拟 `go test -json` 的各种输出）→ 断言 parser 正确提取
- 构造不同失败类型 → 断言 classifier 分类正确
- mock LLM 返回"空修复"动作 → 断言 tracker 正确计数并停机

### 11.3 机制演示计划（§A.6）

| # | 演示行为 | 实现方式 |
|---|----------|----------|
| 1 | 护栏拦截危险命令 | `guard.TestGuardrailIntercept`：mock LLM 输出 `rm -rf /`，断言 `check()` 返回 `blocked=true` |
| 2 | 反馈闭环驱动修正 | `feedback.TestFeedbackLoop`：注入 `compile_error` → mock LLM 收到反馈后返回 `write_file` → 断言 agent 改变行为 |
| 3 | （重点维度）失败分类确定性 | `feedback.TestClassifier`：4 种失败类型各一条输入，断言分类全部准确 |

### 11.4 六个维度的基础实现分工

| 维度 | 实现位置 | 深度 |
|------|----------|------|
| 决策封装 | `internal/loop/` — 主循环 | 基础 |
| 动作/工具 | `internal/tools/` — 5 个工具 | 基础 |
| 上下文/记忆 | `internal/memory/` — 摘要 + 约定 | 基础 |
| 治理护栏 | `internal/guard/` — 模式匹配拦截 | 基础 |
| 反馈闭环 | `internal/feedback/` — 解析 + 分类 + 回灌 + 追踪 | **深入** |
| 配置 | `internal/config/` — 文件 + 命令行 | 基础 |

---

*本文档由 brainstorming 过程沉淀，后续如有修订将在 SPEC_PROCESS.md 中记录。*
