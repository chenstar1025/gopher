# AGENT_LOG: Gopher 开发日志

> 按时间顺序记录每个 task 的关键操作、subagent 派发、人工干预与教训。

---

## 2026-08-09

### 项目初始化
| 项 | 内容 |
|----|------|
| **时间** | 约 20:30 |
| **触发** | 直接操作（非 subagent） |
| **操作** | `git init` + 初始 commit（课程材料 + submission.jsonc） |
| **Commit** | `e05d91f` |

### SPEC 产出
| 项 | 内容 |
|----|------|
| **时间** | 约 21:00 |
| **方式** | Claude Code 主导，用户逐步确认设计决策 |
| **关键决策** | Go 语言、反馈闭环为重点维度、项目名 Gopher |
| **超能力技能** | brainstorming（自然对话形式，未通过 `/brainstorming` 命令触发） |
| **产出** | SPEC.md — 11 节 + A 类额外"领域与机制设计"一节 |
| **人工干预** | 无——用户审查后直接签字确认 |
| **Commit** | `0ab58ef` |

### PLAN 产出
| 项 | 内容 |
|----|------|
| **时间** | 约 21:30 |
| **方式** | Claude Code 主导，将 SPEC 拆为 12 个 task |
| **超能力技能** | writing-plans（自然对话形式） |
| **产出** | PLAN.md — 12 task + 依赖图 + 并行组标记 |
| **人工干预** | 无 |
| **Commit** | `f1cbe12` |

### SPEC_PROCESS 产出
| 项 | 内容 |
|----|------|
| **时间** | 约 22:00 |
| **方式** | Claude Code 主导，文档化 brainstorming 过程 |
| **产出** | SPEC_PROCESS.md — 5 节过程记录 + 反思 |
| **人工干预** | 无 |
| **Commit** | `f584910` |

---

## 2026-08-10

### 冷启动验证（§4.5）

| 项 | 内容 |
|----|------|
| **时间** | 约 00:00–00:30 |
| **验证 agent** | Claude Code subagent（全新 session，无任何对话历史） |
| **关键 prompt** | 仅提供 SPEC.md + PLAN.md 全文，指定实现 T06（治理护栏），要求"遇到不确定之处即暂停询问" |
| **Context 配置** | 全新 subagent，无 memory、无 history、无 prior context |
| **Agent 行为** | 自主下载 Go 1.24.4 → 初始化 module → 写测试（红）→ 实现（绿）→ 全 11 测试通过 |
| **Agent 主动报告的问题** | (1) Module path 占位符 `<user>` 非法，自行改为 `github.com/gopher/gopher` (2) `run_shell` 参数键名未定义，兼容 5 种可能 (3) 白名单语义未说明 |
| **人工干预** | 审查 subagent 代码 → 确认 3 个 SPEC 缺陷 → 修订 SPEC §6.2（参数键名）、§3.6（白名单语义）、§8.1（Go 版本）→ 记录到 SPEC_PROCESS.md §6 |
| **教训** | SPEC 中任何未明确定义的数据结构字段名都会被 agent 自行猜测；占位符不应出现在最终 SPEC 中 |
| **Commit** | `c56aa73` |

### Module path 修正
| 项 | 内容 |
|----|------|
| **时间** | 约 00:40 |
| **人工干预** | 用户提供 GitHub 用户名 `chenstar1025`，全局替换 module path |
| **教训** | 项目初期就应确定 GitHub 仓库名，避免后续全局替换 |
| **Commit** | `a35a6e6` |

### T01 · 项目脚手架
| 项 | 内容 |
|----|------|
| **时间** | 约 01:00 |
| **方式** | Claude Code 直接操作 |
| **操作** | 创建 `.gitignore`、`Makefile`（`make test`/`build`/`build-all`）、`.github/workflows/test.yml`（`unit-test` job） |
| **验证** | `git add -A` → commit → push |
| **人工干预** | 无 |
| **Commit** | `abc27d7` |

### T02+T03 · LLM 抽象 + 工具系统
| 项 | 内容 |
|----|------|
| **时间** | 约 01:15 |
| **方式** | Claude Code 直接编写 |
| **关键 prompt** | 遵循 SPEC §3.3（LLM 接口 + mock + OpenAI）和 §3.4（5 工具 + 注册表） |
| **产出** | `internal/llm/`（接口 + OpenAI HTTP 客户端 + Mock 脚本引擎）+ `internal/tools/`（5 工具 + ToolRegistry） |
| **验证** | `go test ./...` — **24/24 全绿** |
| **教训** | Mock LLM 的 `Script` 模式非常有效——它使主循环和集成测试可以在完全不依赖网络的条件下运行 |
| **Commit** | `6578940` |

### T04+T05+T10 · 配置 + 凭据 + 记忆
| 项 | 内容 |
|----|------|
| **时间** | 约 01:30 |
| **方式** | Claude Code 直接编写 |
| **关键决策** | 凭据存储放弃外部 `wincred` 依赖（网络不可达），改用 AES-256-GCM 加密文件方案 |
| **验证** | `go test ./...` — **全部通过** |
| **人工干预** | 修正 `MaskKey` 测试用例中的期望值错误 + `memory.Store` 的 JSON 序列化 bug + `SummarizeMessages` 的截断逻辑 |
| **教训** | 外部依赖不可达时应降级为自实现方案，不要阻塞进度；这也是 SPEC 中凭据存储"降级方案"的设计意图 |
| **Commit** | `0eb1e61` |

### T08+T09 · 反馈闭环（重点维度）
| 项 | 内容 |
|----|------|
| **时间** | 约 01:45 |
| **方式** | Claude Code 直接编写 |
| **实现要点** | `Parser`（`go test -json` → `[]TestEvent`）、`Classifier`（5 类失败优先级排序）、`Injector`（`Feedback` → `llm.Message`）、`Tracker`（修正轮次计数） |
| **Bug 修复** | `TestFailure` 命名冲突——常量名和结构体同名，将结构体改名为 `Failure`；`Classify` 优先级错误——编译错误应在所有检查之前 |
| **验证** | `go test ./internal/feedback/...` — **23/23 全绿** |
| **教训** | Go 中常量名和类型名不能冲突；反馈分类的优先级必须明确——编译错误是第一优先级，因为它使所有其他检查无效 |
| **Commit** | `3495f4a` |

### T07 · Agent 主循环
| 项 | 内容 |
|----|------|
| **时间** | 约 02:00 |
| **方式** | Claude Code 直接编写 |
| **实现要点** | 上下文组装（system prompt + rules + tools）→ LLM 调用 → 解析 → 护栏 → 执行 → 反馈注入 → 停机 |
| **Bug 修复** | `llm.ToolCall` 与 `tools.ToolCall` 类型冲突——在 loop 中添加手动转换 |
| **验证** | `go test ./internal/loop/...` — **7/7 全绿**，全部用 mock LLM |
| **Commit** | `0a51222` |

### T11 · CLI 入口
| 项 | 内容 |
|----|------|
| **时间** | 约 02:15 |
| **方式** | Claude Code 直接编写 |
| **产出** | `cmd/gopher/main.go` — `run`/`status`/`config set-key`/`config clear-key` 四个子命令 |
| **验证** | `go build ./cmd/gopher/...` 编译成功 |
| **Commit** | `e8d26a4` |

### T12 · 集成测试 + README + 分发
| 项 | 内容 |
|----|------|
| **时间** | 约 02:30 |
| **方式** | Claude Code 直接编写 |
| **产出** | `test/integration_test.go`（3 个集成测试，覆盖治理拦截 + 反馈修正 + 完整修复流程）+ `README.md`（含获取/运行/Key 配置/已知限制/目录结构/许可） |
| **验证** | `go test ./... -count=1` — **81/81 全绿**；`go vet ./...` — 零告警 |
| **§A.6 机制演示对应** | `TestIntegration_GuardrailInterceptsDangerous` → 护栏拦截；`TestIntegration_FeedbackLoopDrivesCorrection` → 反馈驱动修正；`TestClassify_*` 系列 → 确定性分类行为 |
| **Commit** | `2eb4581` |

---

## 人工干预汇总

| 次数 | 问题类型 | 具体操作 |
|------|----------|----------|
| 1 | SPEC 缺陷（冷启动） | 补全 module path、ToolCall 参数键名、白名单语义、Go 版本 |
| 2 | 命名冲突 | `TestFailure` 常量/类型重名 → 结构体改名 `Failure` |
| 3 | 分类逻辑 | 编译错误未优先检查 → 改为两遍扫描 |
| 4 | 测试期望值错误 | `MaskKey` 测试用例的最后 4 字符期望错误 |
| 5 | JSON 序列化 bug | `memory.Store` 序列化全结构而非仅 entries 数组 |
| 6 | 截断逻辑 | `SummarizeMessages` 不包含第一条消息即截断 → 修正为始终包含第一条 |
| 7 | 类型冲突 | `llm.ToolCall` vs `tools.ToolCall` → loop 中添加字段级转换 |
| 8 | 依赖安装失败 | `wincred` 网络不可达 → 改自实现 AES 加密文件存储 |

## 关键教训

1. **Mock LLM 是 harness 开发中最有价值的工具。** 81 个测试无一依赖网络，全部在 mock LLM 下跑通。移除真实 LLM 后，留下的每个测试都在验证"机制是代码，不是提示词"。

2. **SPEC 必须对数据结构字段名做明确定义。** 冷启动验证中 agent 兼容了 5 种 `command` 键名，说明 SPEC §6.2 原本的 `map[string]any` 不够——需要约定 "command" 是 `run_shell` 的参数键。

3. **分类器的优先级顺序很重要。** 编译错误必须在 panic 和 test failure 之前检查——不编译的代码什么都不是。

4. **外部依赖的降级方案不是"备选"而是"主线"。** wincred 装不上时直接切 AES 加密文件方案，后续在 README 标注了已知限制。这正是 SPEC §7 中"降级方案"设计的价值。

5. **Subagent 在 TDD 纪律下表现优异。** 冷启动 agent 没有对话历史指导，但遵循 PLAN 中的"先写测试"指令，严格走红-绿路径，产出了 11/11 通过的代码。

---

*记录完成时间：2026-08-10*
