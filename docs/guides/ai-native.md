# AI-Native Leia

Leia's AI-native surface is standard-library first. Syntax such as `agent`,
`turn`, `tool`, `messages`, and model declarations should desugar to ordinary
runtime modules:

- `llm`: model turns, tools, validation, budget, record/replay;
- `msg`: message constructors and normalized role records;
- `history`: conversation memory helpers;
- `loop`: higher-level agent loops;
- `chat`: lightweight chat/session utilities.

Provider credentials are host configuration. Scripts may select named models,
but embedders decide which providers and capabilities are installed.

Provider setup should prefer environment variables or host options, never
committed secrets. Live-provider tests must be opt-in; offline tests should use
mock or replay providers.

The language goal is concise script syntax with Go-host control:

```leia
models {
    fast: openai_compatible {
        env: "LEIA_LLM_API_KEY",
        model: "fast-model"
    }
}

summarize := agent {
    model: fast,
    system: "Return concise JSON.",
    output: { summary: "string" }
}

result, err := summarize("Summarize this file.")
```

The stable user contract is documented in the
[AI-native reference](../reference/ai/index.md) and the syntax remains governed
by [the language spec](../spec/language.md). New AI features should reuse the
existing stdlib implementation rather than forking an independent AI runtime.
