# Agent Residue Evidence 架构、产品与验收设计

- 日期：2026-08-27
- 产品名：Agent Residue Evidence
- 简称：ARE
- 状态：对话设计已批准，等待书面规格复核

## 1. 决策摘要

Agent Residue Evidence 是一个完全本地、任务粒度、对用户资源无修改能力的证据工具。它回答：

> 一个明确的 Agent 测试任务结束后，在授权范围内新增或遗留了哪些文件、目录、进程和进程绑定端口；这些候选与任务之间有什么证据关系；它们现在是否仍然存在或仍被使用。

ARE 只生成证据列表，不删除文件、不终止进程、不执行清理命令，也不替 Agent 或用户作出“可以安全删除”的决定。责任链固定为：

```text
ARE 观察并列出证据
        ↓
Agent 分析候选与风险
        ↓
Agent 与用户沟通并取得明确授权
        ↓
Agent 使用自身工具执行精确清理
        ↓
ARE 复核候选当前状态
```

首版只支持明确任务范围，不提供全盘扫描。没有任务范围、范围过宽或路径边界无法可靠固定时，ARE 必须拒绝开始或降低结论强度。

## 2. 产品定位

ARE 与 Agent Runtime Proof（ARP）是两个独立产品：

```text
ARP：证明当前实际运行的是什么
ARE：证明一个任务结束后留下了什么
```

二者都使用确定性本地证据、保守结论、CLI 与标准 `stdio` MCP，但不共享产品状态、私有协议或宿主依赖。ARE 不依赖 Across Agents Assistant、Across Autopilot、Across Orchestrator 或 Across Context；AAA、Codex、Claude 等仅属于兼容性验收矩阵。

## 3. 目标与非目标

### 3.1 首版目标

1. 在任务第一次构建或测试命令执行前记录受限基线。
2. 在任务结束时比较基线与当前状态，生成文件、目录、进程和进程绑定端口候选。
3. 接收通用、可选、默认脱敏的 Agent Task Event，以增强任务归因。
4. 支持没有基线的已完成任务受限回溯，但明确降低证据等级。
5. 支持 Agent、宿主和 CI 显式调用任务开始、结束和清理后复核。
6. 支持 Agent 会话中断、MCP 重启和遗忘结束调用的恢复流程。
7. 在 macOS 14+ arm64、Linux amd64 和 Windows 11 amd64 上完成原生验收。
8. 以一次安装的 Agent Plugin + MCP Server 作为主要分发体验，同时提供独立 CLI/MCP 和 MCPB。

### 3.2 明确非目标

首版不做以下内容：

- 不扫描整块磁盘、用户主目录、系统根目录或项目集合根目录；
- 不删除、移动、隔离或修改用户文件；
- 不结束、暂停或重启进程；
- 不关闭端口、容器、虚拟机或系统服务；
- 不创建或删除系统用户、测试身份、浏览器 Profile、Keychain 项或凭据；
- 不读取文件正文；
- 不保存完整命令行、完整对话、原始 Agent Event 或环境变量值；
- 不访问网络，不提供遥测、账号、云同步或远端控制面；
- 不为 AAA、Codex、Claude 或其他宿主分叉核心实现；
- 不输出“安全删除”或“整台机器已干净”的结论；
- 不把交叉编译、模拟数据或源码测试冒充原生平台验收。

## 4. 核心原则

### 4.1 任务粒度

所有观察必须绑定版本化 `task_id`、明确的工作区根和零个或多个任务临时根。ARE 只在这些根内建立文件系统证据。任务范围之外的路径只可作为安全投影的越界引用出现，ARE 不继续展开。

### 4.2 证据与建议分离

ARE 陈述对象、来源、当前状态、活动引用、空间占用、证据等级与限制。是否清理、如何清理以及何时清理由 Agent 与用户决定。

### 4.3 当前状态优先

Agent Event、测试日志或清理返回值不能代替当前复核。如果 Event 声称对象已删除但对象仍存在，报告必须保留冲突并以当前观察为准。

### 4.4 不确定性保守

证据不足、权限不足、扫描竞态、文件身份变化、进程身份不稳定或路径逃逸时返回 `UNKNOWN`、`PARTIAL_EVIDENCE` 或限制信息，不推断为无残留。

### 4.5 完全本地运行

运行时不得创建网络客户端、监听器或远端依赖。安装阶段可以由宿主从正式发布渠道取得并验证安装包；安装后的观察、报告、保留和复核必须在断网环境中完整工作。

## 5. 总体架构

采用“独立核心 + 通用 MCP + 薄 Agent Plugin”架构：

```text
Agent Plugin / Skill          CI / 人类
          │                       │
          ▼                       ▼
    stdio MCP API               CLI
          │                       │
          └──────────┬────────────┘
                     ▼
             Application Service
                     │
       ┌─────────────┼─────────────┐
       ▼             ▼             ▼
 Scope Guard   Observation Engine  Event Normalizer
       │             │             │
       └─────────────┼─────────────┘
                     ▼
            Evidence Correlator
                     │
                     ▼
             Compact Report Store
                     │
         macOS / Linux / Windows adapters
```

依赖规则：

- CLI、MCP 和 Plugin 不拥有第二套判断逻辑；
- 宿主名称不进入核心结论分支；
- Event 只增强归因，不决定当前状态；
- OS 适配层只提供本地观察，不执行修复；
- 所有用户资源访问均为只读；
- 只允许 ARE 自有状态目录产生内部写入。

## 6. 用户旅程与自动化

### 6.1 标准任务

```text
用户要求 Agent 修改代码并运行测试
        ↓
Agent 在第一次测试/构建命令前调用 begin_task_observation
        ↓
Agent 正常执行命令，可选分批提交通用 Event
        ↓
Agent 在最终答复前调用 end_task_observation
        ↓
ARE 返回候选证据报告
        ↓
Agent 向用户解释并请求是否处理
        ↓
用户授权后 Agent 自行精确清理
        ↓
Agent 调用 verify_task_residue
        ↓
ARE 报告候选的最新状态
```

显式调用是协议要求，不等于用户手动操作。支持生命周期 Hook 的宿主可以自动发起标准 MCP 调用；没有 Hook 的宿主由 Plugin Skill 指导 Agent 调用；CI 使用 CLI 调用同一合同。

### 6.2 已完成任务回溯

如果从未建立基线，Agent 可以调用 `inspect_completed_task`，提供任务 ID、明确根、时间窗口和可选通用 Event。ARE 只在这些根内复核当前状态，结果只能是 `EVENT_BOUND`、`RECEIPT_BOUND`、`INFERRED` 或 `UNATTRIBUTED`，不得升级为 `BASELINE_OBSERVED`。

## 7. MCP 与 CLI 合同

### 7.1 MCP 工具

首版提供六个工具：

1. `begin_task_observation`
   - 验证任务范围；
   - 记录轻量文件系统和进程基线；
   - 创建 ARE 自有任务状态；
   - 返回 `observation_id`。

2. `append_task_events`
   - 可选分批接收通用安全 Event；
   - 执行 schema、任务归属、路径范围和敏感字段验证；
   - 只保存归因所需摘要。
   - 允许提交空事件批次作为显式心跳；每次成功追加或空批心跳都会更新任务的最后活动时间。

3. `end_task_observation`
   - 执行限定范围结束观察；
   - 关联基线、Event、回执与当前状态；
   - 生成最终报告并压缩中间状态。

4. `inspect_completed_task`
   - 对没有基线的已完成任务执行受限回溯；
   - 永远披露缺失基线的限制。

5. `get_residue_report`
   - 读取已保存报告；
   - 不重新观察或改变任务状态。

6. `verify_task_residue`
   - 只复核报告内候选的当前状态；
   - 不执行清理；
   - 记录复核时间和状态变化。

这些工具对用户资源均无修改能力，但 `begin`、`append`、`end`、`inspect` 和 `verify` 会修改 ARE 自有状态，因此 MCP 元数据不得错误声明为完全无副作用。

### 7.2 CLI

CLI 与 MCP 提供相同语义，供 CI、脚本和无 Plugin 宿主使用。CLI 至少包含：

```text
agent-residue-evidence begin
agent-residue-evidence event append
agent-residue-evidence end
agent-residue-evidence inspect-completed
agent-residue-evidence report get
agent-residue-evidence report retain
agent-residue-evidence report forget
agent-residue-evidence verify
agent-residue-evidence doctor
agent-residue-evidence mcp
```

CLI 的 JSON 输出与 MCP schema 共用同一合同。

## 8. 任务与报告状态

### 8.1 任务生命周期

- `ACTIVE`：基线存在，任务正在观察；
- `COMPLETED`：正常结束并生成报告；
- `INTERRUPTED`：24 小时无心跳或宿主异常结束；
- `EXPIRED`：报告已按保留策略过期。

同一任务重新连接后可以恢复或显式结束。长任务通过 `append_task_events` 的成功追加或空事件批次刷新心跳。超过 24 小时无心跳的任务自动变为 `INTERRUPTED`，执行一次限定范围复核并生成中断报告，但不执行任何用户资源修改。

### 8.2 报告状态

- `NO_CANDIDATES_OBSERVED`
- `REVIEW_REQUIRED`
- `PARTIAL_EVIDENCE`
- `INTERRUPTED_TASK`
- `OBSERVATION_FAILED`

`NO_CANDIDATES_OBSERVED` 只代表声明观察范围内未观察到候选，不代表整台机器或所有副作用已清理。

### 8.3 证据等级

- `BASELINE_OBSERVED`：任务前后基线直接证明；
- `EVENT_BOUND`：通用结构化 Event 明确关联；
- `RECEIPT_BOUND`：进程或工具回执绑定；
- `INFERRED`：由时间、路径和执行记录推断；
- `UNATTRIBUTED`：存在于任务范围内但无法可靠归因。

### 8.4 候选当前状态

- `PRESENT`
- `ACTIVE_REFERENCE`
- `NO_LONGER_PRESENT`
- `CHANGED_SINCE_REPORT`
- `UNKNOWN`

证据等级与当前状态独立。例如，一个 `EVENT_BOUND` 目录可以在复核时变为 `NO_LONGER_PRESENT`。

## 9. 首版证据对象

首版仅支持：

- 文件；
- 目录；
- 任务相关进程；
- 已归因进程绑定的端口。

端口不能通过主机扫描发现，只能从已归因 PID 查询。Docker/虚拟机内部残留、系统用户、浏览器 Profile、系统服务、共享缓存、Keychain 和凭据不在首版范围内。

每个候选至少包含：

- 类型和稳定候选 ID；
- 任务范围内的精确路径或稳定进程身份；
- 大小、创建/修改时间或进程创建时间；
- 任务、Event、回执和父子关系来源；
- 当前活动引用；
- 证据等级；
- 当前状态；
- 观察限制；
- 面向 Agent 的 `review` 提示，不包含删除命令或安全删除判断。

## 10. 文件系统观察

### 10.1 基线

`begin` 记录轻量元数据：

- 任务根内相对路径；
- 文件类型；
- 平台稳定文件身份；
- 大小；
- 修改时间；
- 权限；
- 符号链接或重解析点状态。

ARE 不对整个工作区预先计算内容哈希。`end` 只对新增、变化或需确认对象执行进一步身份和必要摘要检查。

### 10.2 路径安全

- 拒绝 `/`、用户主目录、驱动器根、项目集合根和其他明显过宽目标；
- 使用规范化后的绝对根和稳定文件身份约束遍历；
- 不跟随越出根的符号链接、junction 或 reparse point；
- 发现路径组件替换、扫描期间变化或身份竞态时返回限制或 `UNKNOWN`；
- Git 状态只能增强归因，`.gitignore` 不能证明对象可删除；
- 权限不足时不申请提权。

## 11. 进程与端口观察

任务开始和结束时可以读取当前用户的轻量进程身份快照，但只保留与任务相关的进程：

- Event 或回执中出现的 PID；
- 与任务命令有稳定父子关系的进程；
- 工作目录位于任务范围内的进程；
- 打开任务候选文件的进程。

稳定进程身份至少绑定 PID 和进程创建时间。无法取得稳定创建时间、路径或父子证据时不得只凭 PID 作确定归因。

ARE 不保存完整命令行、环境变量或无关用户进程元数据。端口只从已归因 PID 的本地 socket 状态取得。

## 12. 通用 Agent Task Event

Event 为可选增强，缺失 Event 不阻止标准基线模式。首版事件类型：

- `command_started`
- `command_completed`
- `process_started`
- `process_exited`
- `artifact_declared`
- `test_phase_started`
- `test_phase_completed`
- `cleanup_attempted`

事件使用版本化 `agent-task-event/1.0` 合同，包含任务 ID、事件 ID、类型、时间、工作目录、安全命令摘要、退出状态、可选 PID 身份和声明输出。禁止提交命令原文、环境变量值、完整对话或文件正文。

核心不解析 AAA、Codex、Claude 或其他宿主的私有历史格式。任何宿主只要产生通用 Event 即可增强归因。

## 13. 本地状态、保留和容量

### 13.1 默认状态路径

- macOS：`~/Library/Application Support/AgentResidueEvidence`
- Linux：`$XDG_STATE_HOME/agent-residue-evidence`，未设置时使用 `~/.local/state/agent-residue-evidence`；
- Windows：`%LOCALAPPDATA%\AgentResidueEvidence`

`ARE_HOME` 可以为 CI 和隔离验收指定任务自有状态目录。

### 13.2 数据内容

只保存：

- 活跃任务轻量基线；
- 安全 Event 摘要；
- 最终证据报告；
- 保留、容量和生命周期索引。

不保存文件副本、文件正文、完整日志、完整对话、原始 Event 或命令原文。

### 13.3 保留规则

- 任务结束后删除可重建中间索引，只保留紧凑报告；
- 最终报告默认保留 7 天；
- 全部报告默认总容量上限 100 MB；
- 超限时优先过期最旧且未标记保留的报告；
- 活跃任务基线不能因容量限制中途清除；
- 24 小时无心跳的活跃任务先生成中断报告，再进入普通保留规则；
- 用户或 Agent 可以通过 CLI 的 `report retain` 标记报告保留，或通过 `report forget` 提前删除 ARE 自有报告；这两个命令只能操作 ARE 报告，不能接受任意文件路径；
- ARE 必须报告自身状态目录的大小和保留策略执行结果。

## 14. 权限与隐私

首版权限模型：

- 不申请管理员权限；
- 不访问网络；
- 只读取明确任务根和任务相关进程状态；
- 只写 ARE 自有状态目录；
- 不执行命令、不修改用户文件、不终止进程；
- 不读取或保存密钥、Token、Cookie、环境变量值、文件正文或完整命令行；
- 本地报告可以保留任务范围内精确路径，以支持用户审核；
- 对外导出默认隐藏用户名和私人目录前缀；
- 安装后在断网状态下保持全部能力。

运行时依赖、源码和最终包必须通过静态扫描与行为测试证明没有网络访问路径。没有遥测开关，因为首版不存在遥测功能。

## 15. 失败与恢复

- 范围过宽、根不存在或无法固定：拒绝 `begin`；
- 部分目录无权限：报告 `PARTIAL_EVIDENCE`；
- Event 与当前状态冲突：保留冲突并以当前状态为准；
- 进程身份不稳定：候选状态为 `UNKNOWN`；
- 扫描中路径变化：不重试到虚假稳定，报告竞态限制；
- `end` 失败：保留已取得证据与失败原因，不返回 `NO_CANDIDATES_OBSERVED`；
- MCP 重启：从 ARE 自有状态恢复活跃任务；
- 24 小时无心跳：标记 `INTERRUPTED`，限定复核并生成报告；
- 容量不足：优先压缩或过期旧报告，不破坏活跃基线；
- 复核对象身份已变化：返回 `CHANGED_SINCE_REPORT`，不把新对象当成原候选。

## 16. 安装、打包与生命周期

用户主要体验为一次安装。内部原子完成：

1. 识别平台和架构；
2. 验证正式包摘要、来源证明和 SBOM；
3. 安装平台原生 CLI/MCP 二进制；
4. 安装薄 Agent Plugin、Skill 和 MCP 启动配置；
5. 注册本地 `stdio` MCP；
6. 执行 `initialize`、`tools/list` 和 `doctor`；
7. 一项失败则回滚整个安装事务。

正式发布包含：

- macOS arm64、Linux amd64、Windows amd64 原生包；
- 一体化 Agent Plugin；
- 跨平台 MCPB；
- 独立 CLI/MCP 安装入口；
- SHA-256 校验清单；
- CycloneDX SBOM；
- 构建来源证明；
- Event、报告和生命周期 schema；
- 安装、升级、回滚和卸载文档。

同版本修复、版本升级和回滚都必须停止旧 MCP 进程、原子替换、重连并验证工具合同。卸载删除程序和 ARE 自有状态时必须分别取得明确意图；默认卸载程序不静默删除仍在保留期内的报告。

## 17. 平台支持

首版正式支持矩阵与 ARP 一致：

| 平台 | 首版状态 | 必需证据 |
| --- | --- | --- |
| macOS 14+ arm64 | 正式支持 | 原生构建、文件身份、进程/端口、MCP、安装态完整旅程 |
| Linux amd64 | 正式支持 | 原生主机或容器构建、文件身份、进程/端口、MCP、安装态完整旅程 |
| Windows 11 amd64 | 正式支持 | 原生机器构建、reparse 安全、进程/端口、MCP、安装态完整旅程 |

交叉编译只能证明构建兼容性，不能替代任何平台的原生验收。

## 18. 验收场景

首版必须覆盖：

1. 标准 `begin → test → end`；
2. 无 Event 的基线报告；
3. Event 增强归因；
4. Event 与当前状态冲突；
5. 没有基线的受限历史回溯；
6. Agent 崩溃、MCP 重启和 24 小时中断恢复；
7. Agent 在测试夹具中模拟用户授权清理后调用 `verify`；
8. 拒绝根目录、主目录、路径逃逸、符号链接和 reparse 攻击；
9. 文件替换、PID 复用和扫描竞态；
10. 报告秘密、绝对开发者路径和完整命令泄漏扫描；
11. 7 天保留、100 MB 上限、标记保留和活跃基线保护；
12. 完全断网运行；
13. 一次安装、同版本修复、升级、回滚和卸载；
14. 测试结束后没有进程、端口、临时目录、数据库、安装包、构建目录或工作树残留。

清理验收只能由测试夹具或 Agent 自有工具删除任务自有对象，不能通过 ARE 删除能力实现。

## 19. 完成与发布门槛

正式发布必须同时满足：

```text
三平台原生验收通过
+ 标准 MCP 在真实 Agent 中完成完整任务旅程
+ 一次安装及失败回滚通过
+ 用户资源无修改能力
+ 运行时零网络访问
+ 无秘密或完整会话泄漏
+ 无测试残留
+ 报告不把不确定性包装成可安全删除结论
```

测试、构建、打包、安装、运行服务、用户可见结果和公开发布必须作为独立证据层报告。源码测试通过不能替代安装态 MCP 或真实 Agent 旅程。

## 20. 实施边界

首个实施计划应按以下顺序推进，但不得在本设计书面复核前开始：

1. 合同、范围守卫和失败测试；
2. 文件系统基线与差异核心；
3. 通用 Event 和证据关联；
4. 进程与端口平台适配；
5. 生命周期、中断恢复和报告保留；
6. CLI 与 `stdio` MCP；
7. Agent Plugin、MCPB 和原子安装；
8. macOS、Linux、Windows 原生验收；
9. 开源发布与真实 Agent 兼容矩阵。

本设计不授权创建远端仓库、发布版本或接入第三方账号。此类外部状态变更应在后续实施计划和发布门中单独确认。
