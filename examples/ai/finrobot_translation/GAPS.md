# FinRobot Translation Gaps

## Core Agents

- AutoGen `GroupChat` speaker selection is translated as explicit coordinator history and assertions. Leia currently has primitives for agents, tools, turns, and history, but no direct drop-in object equivalent for mutable `GroupChatManager` speaker policy.
- FinRobot `UserProxyAgent` code execution is represented as ordinary tools or tool-result history. This avoids adding financial or execution-specific runtime APIs, but it does not preserve AutoGen's exact proxy lifecycle knobs such as `human_input_mode`, `max_consecutive_auto_reply`, or `code_execution_config`.
- FinRobot RAG wiring through `get_rag_function(...)` can be expressed as a normal Leia tool, but this skeleton does not translate vector-store setup or retrieval backends because those are application-specific resources rather than core agent/workflow semantics.
- Nested chat summary modes such as `summary_method="reflection_with_llm"` are modeled with structured specialist output plus a follow-up leader turn. There is not yet a named Leia dialect field that exactly mirrors AutoGen summary methods.
- `TERMINATE` remains a prompt-level convention in the translated examples. Leia structured outputs and assertions can validate completion state, but they do not reinterpret `TERMINATE` as a built-in control signal.
