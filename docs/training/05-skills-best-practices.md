# Claude Code Skills 最佳实践

技能（Skills）是 Claude Code 的核心扩展机制。本文将首先介绍通用的高效技能，随后列举 HotPlex 项目专属的技能。

## 1. 通用高效技能 (General Skills)
这些技能是您在任何项目中提高生产力的利器：

- **代码精简与优化 (`/simplify`)**
  - **用途**：审查已更改的代码，检查复用性、质量和效率。
  - **推荐用法**：`/simplify 审查代码并遵循 DRY 与 SOLID`
  - **特点**：通过显式要求遵循架构原则，能更精准地识别重复逻辑并进行结构化重构。

- **技能创造者 (`/skill-creator`)**
  - **用途**：从零开始创建新技能，或修改、优化现有技能。
  - **特点**：当您发现某些操作在重复时，用它来固化您的工作流。

- **项目说明助手 (`/claude-md-improver`)**
  - **用途**：自动分析项目结构并优化 `CLAUDE.md`。
  - **核心技巧**：以下两种情况**必须**执行该指令以更新 AI 的“全局视野”：
    1. **纠错后**：当 AI 在某个问题上反复犯错而你终于纠正过来时，记录正确解法，防止回归（Regression）。
    2. **变更后**：当您**新增了功能模块**或进行了**重大架构变更**（重构、分层调整）时，确保 AI 了解最新的项目地图。

- **头脑风暴 (`/brainstorming`)**
  - **用途**：在涉及创意工作（如创建新功能、构建组件）前必须使用的技能。
  - **Superpower 创造性工作流演示**：
    ```text
    [需求意图] 
       ↓
    [/superpowers:brainstorm] ➔ (Socratic Questioning 澄清需求与边界)
       ↓
    [/superpowers:write-plan] ➔ (生成详细的 spec/plan 任务清单)
       ↓
    [/superpowers:execute-plan] ➔ (调用子代理执行具体编码任务)
    ```
  - **特点**：通过苏格拉底式提问深入探索用户意图，将模糊想法固化为可执行的架构方案。

- **功能开发流 (`/feature-dev`)**
  - **用途**：将复杂功能开发拆解为标准流程的官方技能。
  - **7 阶段工作流演示**：
    ```text
    [1. Discovery] ➔ [2. Exploration] ➔ [3. Clarification]
           分析需求            探索代码库            澄清模糊点
                                                     ↓
    [6. Quality Review] ⇠ [5. Implementation] ⇠ [4. Architecture]
         质量审查             编码开发与实现          架构设计与评估
           ↓
    [7. Final Summary] ➔ (完成)
         交付总结
    ```
  - **核心价值**：强制执行“先设计后编码”，通过 7 个阶段确保功能的 predictability (可预测性) 和代码质量。


## 2. Skills 调用最佳实践

### 2.1 描述需具体
不要模糊地问“帮我看看”，而应使用具体动词：
- **坏例子**：`看看代码`
- **好例子**：`使用 /simplify 检查当前文件的复用性和性能瓶颈`

### 2.2 组合技能使用
Claude 可以跨技能协作。例如：先用 `/brainstorming` 敲定方案，再用 `/feature-dev` 执行开发。

---

## 3. HotPlex 专属技能
以下是专为 HotPlex 项目定制开发的自动化技能：

- **Docker 容器管理 (`docker-container-ops`)**
  - **用途**：一键重启 hotplex、查看容器状态、启动或停止 Bot。
- **项目诊断大师 (`hotplex-diagnostics`)**
  - **用途**：系统异常时，快速获取健康报告和容器日志分析。
- **PR 与 Issue 管理 (`hotplex-pr-master` & `hotplex-issue-master`)**
  - **用途**：自动化 GitHub 的 PR 审查及 Issue 的生命周期管理。
- **文档防腐同步 (`hotplex-doc-sync`)**
  - **用途**：确保文档与代码实现实时同步，防止文档过期。

## 4. 如何管理技能
- **查看所有可用技能**：在会话中直接问 `有哪些已安装的 skills?`。
- **查看技能详情**：`view_file` 查看 `.agent/skills/` 下的 `SKILL.md`。

---
*提示：建议优先尝试通用技能以建立直觉，再结合 HotPlex 专属技能提升项目效率。*
