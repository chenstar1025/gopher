# PLAN: Gopher 实现计划

> 每个 task 可由一个 subagent 在一次会话内完成。TDD 强制：先写失败测试，再实现，再重构。

---

## 任务依赖图

```
T01 (项目脚手架)
 │
 ├──► T02 (LLM 抽象层)
 │     │
 │     ├──► T04 (配置系统)
 │     │     │
 │     │     └──► T07 (主循环)
 │     │
 │     └──► T05 (凭据管理) ──► T11 (CLI 入口)
 │
 ├──► T03 (工具系统)
 │     │
 │     ├──► T06 (治理护栏)
 │     │     │
 │     │     └──► T07 (主循环)
 │     │
 │     └──► T08 (反馈闭环 · 解析器+分类器)
 │           │
 │           ├──► T09 (反馈闭环 · 注入器+追踪器)
 │           │     │
 │           │     └──► T07 (主循环)
 │           │
 │           └──► T10 (记忆系统)
 │
 └──► T07 ──► T11 ──► T12 (集成测试+CI+分发)
```

## 可并行任务组合

- **并行组 1**：T02 + T03（LLM 抽象和工具系统互不依赖）
- **并行组 2**：T04 + T05 + T10（配置、凭据、记忆互不依赖，在 T02 完成后）
- **并行组 3**：T06 + T08（护栏和反馈解析器互不依赖，在 T03 完成后）

---

## T01 · 项目脚手架

| 项 | 内容 |
|----|------|
| **目标** | 初始化 Go 模块、目录结构、CI 配置 |
| **涉及文件** | `go.mod`, `main.go` (空入口), `.github/workflows/test.yml`, `.gitignore`, `Makefile` |
| **实现要点** | `module github.com/<user>/gopher`；目录按 SPEC §3.1 创建；CI 只含 `unit-test` job（先写一个占位 `go test ./...`） |
| **验证步骤** | 1. `go build ./...` 无错误 2. `go test ./...` 通过（即使无测试文件） 3. `make test` 可用 |
| **依赖** | 无 |
| **TDD 要求** | N/A（纯脚手架，无业务代码） |
| **可并行** | 仅此一个 |

---

## T02 · LLM 抽象层

| 项 | 内容 |
|----|------|
| **目标** | 实现 `LLM` 接口 + `openai` 实现 + `mock` 实现 |
| **涉及文件** | `internal/llm/llm.go`, `internal/llm/openai.go`, `internal/llm/mock.go`, `internal/llm/openai_test.go`, `internal/llm/mock_test.go` |
| **实现要点** | 接口：`Chat(ctx, messages, tools) (*Response, error)`；OpenAI 实现发 HTTP POST 到 `/v1/chat/completions`；Mock 按预设 `Script` 返回确定性响应 |
| **验证步骤** | 1. `mock_test.go`：构造 script → 断言返回内容精确匹配 2. `openai_test.go`：用 `httptest.NewServer` 伪造 API → 断言请求体和响应解析正确 3. 测试命令：`go test ./internal/llm/... -v` |
| **依赖** | 无 |
| **TDD 要求** | 先写 `mock_test.go`（红）→ 实现 `mock.go`（绿）→ 同理 `openai.go` |
| **可并行** | 可与 T03 并行 |

---

## T03 · 工具系统

| 项 | 内容 |
|----|------|
| **目标** | 实现 5 个工具 + `ToolRegistry` + 工具执行器 |
| **涉及文件** | `internal/tools/tool.go` (接口+注册表), `internal/tools/file.go` (read/write/list), `internal/tools/shell.go` (run_shell, run_test), `internal/tools/tool_test.go` |
| **实现要点** | `Tool` 接口：`Name() string`, `Execute(args map[string]any) ToolResult`；`ToolRegistry` 按名查找；`run_test` 执行 `go test -json ./...` 并捕获输出 |
| **验证步骤** | 1. 测试 `read_file` 读真实临文件 → 断言内容一致 2. 测试 `write_file` 写入 → 断言文件内容 3. 测试 `run_shell` 执行 `echo hello` → 断言 output="hello" 4. `go test ./internal/tools/... -v` |
| **依赖** | 无 |
| **TDD 要求** | 每个工具先写测试 → 再实现 |
| **可并行** | 可与 T02 并行 |

---

## T04 · 配置系统

| 项 | 内容 |
|----|------|
| **目标** | 实现 `Config` 结构体 + 文件读取 + 命令行覆盖 |
| **涉及文件** | `internal/config/config.go`, `internal/config/config_test.go` |
| **实现要点** | 默认值在代码中；从 `.gopher/config.json` 读取覆盖；支持环境变量 `GOPHER_LLM_ENDPOINT` 等；白名单命令列表 |
| **验证步骤** | 1. 测试默认配置 2. 测试从 JSON 文件加载 3. 测试环境变量覆盖 4. `go test ./internal/config/... -v` |
| **依赖** | T02（需要知道 LLM 相关字段） |
| **TDD 要求** | 测试 → 实现 |
| **可并行** | 可与 T05、T10 并行 |

---

## T05 · 凭据管理

| 项 | 内容 |
|----|------|
| **目标** | 实现 API Key 的安全存储、读取、更新、清除 |
| **涉及文件** | `internal/credential/credential.go`, `internal/credential/wincred.go`, `internal/credential/file.go` (Linux/macOS 降级), `internal/credential/credential_test.go` |
| **实现要点** | 接口：`Store(key string) error`, `Retrieve() (string, error)`, `Delete() error`, `Status() string`；Windows 用 `wincred`；降级方案用权限 0600 文件 |
| **验证步骤** | 1. mock 凭据后端 → 测试 Store/Retrieve/Delete/Status 2. Status 返回不包含明文 3. `go test ./internal/credential/... -v` |
| **依赖** | T02（无直接依赖，可并行） |
| **TDD 要求** | 先写接口测试（用 mock 后端）→ 再实现真实后端 |
| **可并行** | 可与 T04、T10 并行 |

---

## T06 · 治理护栏

| 项 | 内容 |
|----|------|
| **目标** | 实现危险命令模式匹配 + 拦截 + 人工确认 |
| **涉及文件** | `internal/guard/guard.go`, `internal/guard/patterns.go`, `internal/guard/guard_test.go` |
| **实现要点** | `func Check(action ToolCall) (blocked bool, reason string)`；正则匹配 §3.6 的危险模式；`Confirm(action ToolCall) bool` 等待 stdin yes/no |
| **验证步骤** | 1. 构造 `rm -rf /` → `Check` 返回 blocked=true 2. 构造 `echo hello` → `Check` 返回 blocked=false 3. mock stdin 输入 yes → Confirm 返回 true 4. 白名单命令即使命中模式也放行 5. `go test ./internal/guard/... -v` |
| **依赖** | T03（需要 `ToolCall` 类型） |
| **TDD 要求** | 测试 → 实现 |
| **可并行** | 可与 T08 并行 |

---

## T07 · Agent 主循环

| 项 | 内容 |
|----|------|
| **目标** | 实现完整的主循环：上下文组装 → LLM 调用 → 解析 → 护栏 → 执行 → 反馈回灌 → 停机 |
| **涉及文件** | `internal/loop/loop.go`, `internal/loop/context.go` (上下文组装), `internal/loop/loop_test.go` |
| **实现要点** | 将 T02–T09 全部组装；循环在以下条件停机：无工具调用 / 达到 maxRounds / 用户中断 / 死循环检测；死循环检测=同一动作连续 3 轮失败 |
| **验证步骤** | 1. mock LLM 返回无工具调用的回复 → 1 轮后停机 2. mock LLM 返回 read_file → write_file → 无工具调用 → 3 轮后停机 3. mock LLM 返回连续 3 次相同失败动作 → 死循环检测触发停机 4. `go test ./internal/loop/... -v`（全部用 mock LLM） |
| **依赖** | T02, T03, T06, T09 |
| **TDD 要求** | 主循环测试全部用 mock LLM —— 核心验证 |
| **可并行** | 不可（最后组装） |

---

## T08 · 反馈闭环 · 解析器 + 分类器

| 项 | 内容 |
|----|------|
| **目标** | 实现 `go test -json` 输出解析 + 失败分类 |
| **涉及文件** | `internal/feedback/parser.go`, `internal/feedback/classifier.go`, `internal/feedback/parser_test.go`, `internal/feedback/classifier_test.go` |
| **实现要点** | `ParseTestOutput(jsonl string) ([]TestEvent, error)` 逐行解析；`Classify(events []TestEvent) FeedbackType` 分类为 compile_error/test_failure/timeout/panic/success |
| **验证步骤** | 1. 构造 compile_error JSON → Parse 正确提取 + Classify=compile_error 2. 同理构造 test_failure / timeout / panic JSON 3. 构造全部 pass 的 JSON → Classify=success 4. `go test ./internal/feedback/... -v` |
| **依赖** | T03（需要 know `run_test` 的输出格式） |
| **TDD 要求** | 测试 → 实现 |
| **可并行** | 可与 T06、T03 并行 |

---

## T09 · 反馈闭环 · 注入器 + 追踪器

| 项 | 内容 |
|----|------|
| **目标** | 实现反馈回灌 + 修正轮次追踪 |
| **涉及文件** | `internal/feedback/injector.go`, `internal/feedback/tracker.go`, `internal/feedback/injector_test.go`, `internal/feedback/tracker_test.go` |
| **实现要点** | `Injector` 将 `Feedback` 转为 `Message{Role: "tool", Content: summary}`；`Tracker` 记录 task_id → 修正轮数，超 maxRetries(3) 返回 `ShouldStop=true` |
| **验证步骤** | 1. 构造 Feedback → Inject → 断言输出 Message 格式正确 2. Tracker：连续 `Record` 同 task 3 次 → ShouldStop=true 3. Tracker：不同 task → 各独立计数 4. `go test ./internal/feedback/... -v` |
| **依赖** | T08 |
| **TDD 要求** | 测试 → 实现 |
| **可并行** | 不可（依赖 T08） |

---

## T10 · 记忆系统

| 项 | 内容 |
|----|------|
| **目标** | 实现项目约定注入 + 对话摘要 + 本地 JSON 存储 |
| **涉及文件** | `internal/memory/memory.go`, `internal/memory/summary.go`, `internal/memory/memory_test.go` |
| **实现要点** | 读取 `.gopher/rules.md` 注入 system prompt；每 10 轮触发摘要（摘要是纯文本截断 + token 估算，不调 LLM）；存储至 `.gopher/memory.json` |
| **验证步骤** | 1. 创建临时 rules.md → 断言 LoadRules 读入正确 2. 模拟 10 轮对话 → 断言摘要触发 3. 存储+读取会话 → 断言消息完整性 4. `go test ./internal/memory/... -v` |
| **依赖** | 无 |
| **TDD 要求** | 测试 → 实现 |
| **可并行** | 可与 T04、T05 并行 |

---

## T11 · CLI 入口

| 项 | 内容 |
|----|------|
| **目标** | 实现 `cmd/gopher/main.go` 的命令行接口 |
| **涉及文件** | `cmd/gopher/main.go`, `internal/cli/run.go`, `internal/cli/config.go`, `internal/cli/cli_test.go` |
| **实现要点** | 子命令：`run <task>`（启动 agent）、`status`（查看凭据状态）、`config set-key`（配置 Key）、`config clear-key`（清除 Key）、`--verbose` 全局 flag |
| **验证步骤** | 1. `gopher --help` 正常输出 2. `gopher run "echo hello"` 在 mock 模式下跑通 3. `gopher status` 准确显示 4. `gopher config set-key` 引导录入（测试用 mock stdin） 5. `go test ./internal/cli/... -v` + `go test ./cmd/... -v` |
| **依赖** | T07, T05 |
| **TDD 要求** | CLI 测试用 `exec.Command` 或直接调函数 → 断言输出 |
| **可并行** | 不可（最后组装） |

---

## T12 · 集成测试 + CI + 分发

| 项 | 内容 |
|----|------|
| **目标** | 端到端集成测试、完善 CI、构建二进制、写 README |
| **涉及文件** | `test/integration_test.go`, `.github/workflows/test.yml`（完善）, `Makefile`, `README.md`, `Dockerfile`（可选） |
| **实现要点** | 集成测试用 mock LLM 跑完整"读文件→改代码→跑测试→修正→通过"场景；CI 包含 `unit-test` job + `build` job；Makefile 添加 `build`、`build-all`（交叉编译）target |
| **验证步骤** | 1. `make test` 全绿 2. `make build` 产出单文件二进制 3. CI pass 4. README 包含安装/运行/Key 配置/已知限制 |
| **依赖** | 所有前置 task |
| **TDD 要求** | 集成测试先写期望的端到端行为 → 组装实现 |

---

## 任务汇总

| Task | 模块 | 预估时间 | 可并行 |
|------|------|----------|--------|
| T01 | 项目脚手架 | 10 min | - |
| T02 | LLM 抽象层 | 30 min | T03 |
| T03 | 工具系统 | 30 min | T02 |
| T04 | 配置系统 | 15 min | T05, T10 |
| T05 | 凭据管理 | 25 min | T04, T10 |
| T06 | 治理护栏 | 20 min | T08 |
| T07 | 主循环 | 40 min | - |
| T08 | 反馈闭环 · 解析+分类 | 25 min | T06 |
| T09 | 反馈闭环 · 注入+追踪 | 20 min | - |
| T10 | 记忆系统 | 20 min | T04, T05 |
| T11 | CLI 入口 | 25 min | - |
| T12 | 集成+CI+分发 | 30 min | - |

**总预估**：约 5 小时（单个 subagent 连续工作），实际需要留出人工审查与修正时间。

---

*PLAN 持续更新：每完成一个 task 即标记完成并附 commit hash。*
