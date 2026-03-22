# Global Types

The `types` package contains the fundamental data structures used across the entire HotPlex ecosystem. This shared library ensures type safety and consistency between the **Engine**, **ChatApps**, and **SDK**.

## 🧱 Core Models

- **`Config`**: Turn-level execution context (WorkDir, SessionID, Instructions).
- **`StreamMessage`**: The fundamental wire protocol envelope for all events.
- **`AssistantMessage`**: Structured message containing hierarchical content blocks.
- **`ContentBlock`**: Atomic unit of model output (text, tool_use, or tool_result).
- **`UsageStats`**: Detailed token consumption breakdown.
- **`MessageType`**: Type alias for categorizing interaction intents.

## 🌊 Message Types

HotPlex uses `MessageType` (string) to categorize event intents.

| Type | Description | Storable |
| :--- | :--- | :--- |
| `user_input` | Raw user command | Yes |
| `final_response` | Complete assistant answer | Yes |
| `thinking` | Model's internal reasoning | No |
| `tool_use` | Tool invocation with arguments | No |
| `tool_result` | Output or error from a tool | No |
| `status` | Progress indicator/Generic log | No |
| `error` | System or execution error | No |
| `danger_block` | WAF hit/Security intervention | No |
| `session_stats` | Final token and cost summary | No |
| `permission_request` | Interactive approval needed | No |
| `answer` | Streaming token or final text | No |

## 📐 Interface Definitions

This package also defines the core interfaces for:
- **`Provider`**: The abstraction layer for AI CLI tools.
- **`Storage`**: Backend persistence contracts for session history.
