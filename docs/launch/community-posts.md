# Community launch copy

Replace `RELEASE_URL` only after the public v0.3.0 release is independently
verified. These drafts are prepared but are not posted without the owner's
authenticated account and channel approval.

## Show HN

**Title:** Show HN: Agent Residue Evidence – see what an Agent test left behind

I built Agent Residue Evidence (ARE) for a small gap in Agent testing: a command
finishing does not prove its files, helper processes, or listening ports are
gone.

ARE is a local stdio MCP server and CLI. An Agent declares the exact workspace
and task-owned temporary roots before a test or build. ARE reports scoped,
path-aliased evidence, then stops for human review. It has no delete, execute,
terminate, or close tool. After authorized cleanup by the Agent's normal host
tools, ARE can verify the same stable candidates.

The reproducible demo creates one harmless task-owned artifact and proves the
review boundary. No daemon, network access, telemetry, full-host scan, raw
environment values, or transcripts.

Demo: https://github.com/fantasyce/agent-residue-evidence/blob/main/docs/demo.md
Release: RELEASE_URL
Source: https://github.com/fantasyce/agent-residue-evidence

## OpenAI Developer Forum

Agent Residue Evidence (ARE) is an evidence-only local MCP server for Agent test
and build residue. It observes exact task roots prospectively, groups files,
directories, attributed processes and ports, and requires an opaque task
capability for report access.

ARE deliberately has no cleanup tool. The Agent presents evidence, the user
decides, the Agent cleans with its normal tools, and ARE verifies. A
`NO_CANDIDATES_OBSERVED` result remains scoped and is never described as a
clean-host certification.

Quickstart: https://github.com/fantasyce/agent-residue-evidence/blob/main/docs/quickstart.md
Demo: https://github.com/fantasyce/agent-residue-evidence/blob/main/docs/demo.md
Release: RELEASE_URL

## MCP community

Agent Residue Evidence v0.3.0 packages an eight-tool, evidence-only stdio MCP
server for task-scoped test and build residue.

It observes explicit roots, returns path-aliased evidence, isolates reports
behind opaque owner capabilities, and verifies stable candidates after
user-authorized Agent cleanup. It cannot delete files, stop processes, close
ports, execute commands, scan the full machine, use the network, or collect
telemetry.

Release: RELEASE_URL
Source: https://github.com/fantasyce/agent-residue-evidence

## Reddit

**Title:** Open-source MCP tool for seeing what an AI Agent test left behind

I released Agent Residue Evidence (ARE), an Apache-2.0 local residue checkpoint
for Agent tests and builds.

Before work starts, an Agent declares the exact workspace and task-owned temp
roots. ARE then reports files, directories, attributed processes, and listening
ports that appeared in scope. It does not clean them: evidence is reviewed,
the user authorizes action, the Agent uses normal host tools, and ARE verifies.

No daemon, network access, telemetry, full-host scan, raw command lines,
environment values, or transcripts.

Demo: https://github.com/fantasyce/agent-residue-evidence/blob/main/docs/demo.md
Release: RELEASE_URL

## X

Tests finish. Residue remains.

Agent Residue Evidence v0.3.0 gives Agents a local, task-scoped checkpoint for
files, processes, and ports left by tests/builds. Evidence first; human
authorization before cleanup; verification after. No cleanup tool or telemetry.

Demo: https://github.com/fantasyce/agent-residue-evidence/blob/main/docs/demo.md
RELEASE_URL

## LinkedIn

A test command returning successfully does not prove its temporary files,
helper processes, or listening ports are gone.

Agent Residue Evidence (ARE) adds a task-scoped checkpoint to AI Agent testing
and build workflows. It observes explicit roots, presents path-aliased evidence
for review, and verifies stable candidates after user-authorized cleanup.

ARE is deliberately evidence-only: no delete, execute, terminate, or close
tool; no telemetry, network use, or full-machine scan.

Demo: https://github.com/fantasyce/agent-residue-evidence/blob/main/docs/demo.md
Release: RELEASE_URL

## 中文

测试命令结束，不代表它创建的临时文件、辅助进程或监听端口已经消失。

Agent Residue Evidence（ARE）为 Agent 的测试与构建流程增加一个本地、任务级的残留证据检查点：开始前声明准确的工作区与任务临时目录；结束后由 Agent 向用户解释证据；只有用户授权后，Agent 才使用原有主机工具清理；ARE 再核验相同候选项。

ARE 本身没有删除、执行、终止进程或关闭端口的工具，也不会扫描整台机器、联网或收集遥测。

演示：https://github.com/fantasyce/agent-residue-evidence/blob/main/docs/demo.md
正式版本：RELEASE_URL
源码：https://github.com/fantasyce/agent-residue-evidence
