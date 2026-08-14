# REFLECTION: Gopher 项目反思


---

## 1. Superpowers 技能的效用评估

### 发挥了最大作用的技能

**TDD（`test-driven-development`）** 在本项目中非常重要：TDD 在 AI 协作下不是阻碍，而是一种**超线性放大器**。原因是AI agent 善于"写代码"，但不善于"判断自己写的代码对不对"。红-绿-重构的循环则给了agent 一个客观、确定、可自动验证的评判标准。所有测试全部用 mock LLM 写完后，每次重构或新增功能，`go test ./...` 跑一遍，三秒内就可以知道有没有回归。

**`writing-plans`** 同样也是关键的。PLAN 中每个 task 标注的文件路径和验证步骤，在后续 subagent 派发时成为了精确的"任务说明书"，减少了 agent 自行发挥的空间。

### 形式大于实质的技能

**Git worktrees** 在当前项目规模下价值有限。worktree 的设计意图是在大型团队项目中隔离并行 feature branch——但对于一个单人项目，每个 task 之间依赖紧密，worktree 并行能发挥的空间很小。PLAN 中标了 3 组可并行 task，但实际执行时我仍然选择串行——因为上下文切换成本高于等待成本。

---

## 2. TDD 在 AI 协作下的角色
该项目实践中证明：**TDD 是最有效的 prompt 压缩手段**。

一个例子：在实现反馈闭环的 `Classify` 函数时，我没有在 SPEC 或 PLAN 中完整描述"编译错误必须在所有其他错误之前检查"。Agent 写出的第一版代码把 `panic` 检查放在了 `build failed` 之前，导致同时有 panic 和编译错误时错误分类为 `panic`。如果没有测试，这个 bug 会在真实 LLM 调用时产生不可预测的行为。但因为有 `TestClassify_CompileOverridesEverything` 这个测试，agent 在后续修复中明确看到了失败信号，修正了优先级。


---

## 3. Subagent 工作流与 task 颗粒度

冷启动验证是检验 subagent 自主性的最佳窗口。一个全新 agent 仅凭 SPEC + PLAN 实现了 T06（治理护栏），产出了 11/11 全绿的代码。它能自主运行约30分钟不偏离主题，这正是 PLAN 中一个 task 的预估时间。

**最优 task 颗粒度：一个 task = 一个包 = 一组测试 = 一个 subagent 一次完成。** 更小的颗粒度（如"写 struct 定义 → 写构造函数 → 写测试"三连 task）会产生过多 agent 启动开销；更大的颗粒度（如"实现整个 harness"一个 task）会让 agent 不清楚任务方向。

---

## 4. SPEC 质量如何影响实现质量

**举例：`ToolCall.Args` 的参数键名未定义。**

原始 SPEC §6.2 写的是 `Args map[string]any`，没有约定参数键名。冷启动 agent 为此兼容了 5 种可能的键名（`command`/`cmd`/`script`/`shell`/`input`）——这是一个典型的"规格不清 → agent 过度补偿"案例。如果这个缺陷留到真正使用真实 LLM 的环节，LLM 可能用 `cmd` 传参而我们的工具系统可能只认 `command`，导致静默失败，工具被调用但参数为空。

修订后的 SPEC 明确了：`read_file`/`write_file` 用 `"path"`，`run_shell` 用 `"command"`。三个字符的约定，可能省下了数小时的调试。

**一般规律：SPEC 中所有 `map[string]any` 都是潜在缺陷——必须说明 key 是什么。**

---

## 5. 最有效的 Prompt/Context 策略

**策略一：在工具定义中写清楚参数 JSON Schema。** 5 个工具的 `Parameters()` 方法返回的是标准 JSON Schema，这是 LLM 原生理解的格式。相比之下，在 SPEC 中用自然语言描述"run_shell 接受 command 参数"的效果差得多，因为LLM 会遗忘。

**策略二：反馈闭环的自然语言摘要。** `BuildSummary` 生成的字符串不是 JSON 格式的测试结果，而是我们可读的 "Test failures detected:\n- TestFoo: expected 1, got 2"。这让 LLM 能像人类工程师一样"读测试报告"，而不需要额外解析。

**策略三：最小上下文原则。** 每轮的 context 只包含 system prompt + memory rules + 最近对话。记忆系统用摘要替代全量历史——10 轮触发一次摘要压缩。这保持了上下文窗口的可用性。

---

## 6. 凭据与分发的工程价值



**凭据安全：** API Key 不是配置项，而应作为机密信息处理。其录入、存储、传输、日志输出、进程内存等所有环节都需要进行威胁建模。我采用 AES-256-GCM 加密文件存储，同时在 README 中明确注明“未接入操作系统原生密钥链”这一限制。注明该限制是工程中应有的诚实做法。

**分发：** "能跑"和"别人也能跑"之间有一道鸿沟。`go build` 产出的二进制在我的机器上能跑，但要让它在新机器上从零跑起来，需要：README 写清命令、首次运行引导录 Key、交叉编译产出多平台二进制、CI 产物自动上传。这些都是"最后一个用户"视角下的工程问题。

---

## 7. 如果重做会改变什么

1. **先建 GitHub 仓库，再写 SPEC。** Module path 的三次修改（`<user>` → `ztjd` → `chenstar1025`）可避免。
2. **先写数据结构约定，再写实现。
3. **给冷启动验证留更多时间。
4. **Mock LLM 的脚本应支持"基于前一轮结果动态选择"**——当前 mock 完全按预设顺序返回，无法模拟"LLM 看到失败后改变策略"的动态行为。这是本项目 mock 机制的局限。

---

## 8. 对 Superpowers 方法论的批判

Superpowers 的核心假设是：**软件开发中的工程纪律（TDD、plan、review、分支管理）可以用一套标准化技能固定下来，且这套技能在不同项目中可迁移。** 这个假设在我的项目中**部分成立**：

成立的部分：TDD 和 writing-plans 确实普适——不论做什么项目，先写测试再写代码，以及先做计划再写代码，都能提升 AI 协作质量。

不成立的部分：Git worktrees 和 finishing-a-development-branch 假设了多分支、多 PR 的团队协作场景，但单人项目天然是单分支 workflow。强行走 worktree 流程更复杂。**Superpowers 没区分"单人项目"和"团队项目"——这导致部分技能的强制要求沦为形式。**

