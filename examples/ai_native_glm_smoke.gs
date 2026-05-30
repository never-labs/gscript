// Real GLM AI-native multi-turn memory smoke demo.
//
// This mirrors the local glm_cc wrapper's configuration without invoking it:
//   endpoint: https://open.bigmodel.cn/api/anthropic
//   key:      SENTINEL_GLM_API_KEY or GLM_API_KEY
//   model:    GLM_MODEL, default glm-5.1
//
// Run without committing tokens:
//   GSCRIPT_GLM_BASE_URL=https://open.bigmodel.cn/api/anthropic \
//   GSCRIPT_GLM_API_KEY=... \
//   GSCRIPT_GLM_MODEL=glm-5.1 \
//   gscript examples/ai_native_glm_smoke.gs

models {
    default: "glm-smoke"
    "glm-smoke": {
        provider: "glm"
        protocol: "anthropic_compatible"
        base_url: os.getenv("GSCRIPT_GLM_BASE_URL")
        api_key: os.getenv("GSCRIPT_GLM_API_KEY")
        provider_model: os.getenv("GSCRIPT_GLM_MODEL")
    }
}

history := messages {
    system: "You are a deterministic memory smoke-test assistant. Follow exact reply instructions. Keep answers short."
    user: "Store this memory: project codename is ORCHID and owner is ADA. Reply exactly: MEMORY_STORED"
}

stored, err := turn {
    messages: history
    max_tokens: 32
    temperature: 0
}
if err != nil {
    print(err.message)
    return
}

history[#history + 1] = msg.assistant(stored.text)
history[#history + 1] = msg.user("Using only the stored memory, reply exactly: project=ORCHID;owner=ADA")

recalled, err := turn {
    messages: history
    max_tokens: 48
    temperature: 0
}
if err != nil {
    print(err.message)
    return
}

history[#history + 1] = msg.assistant(recalled.text)

extractor := agent(summary) {
    model: "glm-smoke"
    system: "Return only compact JSON with exactly these keys: project, owner, remembered, meta. meta must be an object with source. Do not include Markdown."
    user: "Convert this memory recall into JSON. Use project=\"ORCHID\", owner=\"ADA\", remembered=true, meta.source=\"history\" when the recall says project=ORCHID;owner=ADA. Recall: " .. summary
    output: {
        project: "ORCHID"
        owner: "ADA"
        remembered: true
        meta: {
            source: "history"
        }
    }
    max_tokens: 96
    temperature: 0
}

extracted, err := extractor(recalled.text)
if err != nil {
    print(err.message)
    return
}

print("stored=" .. stored.text)
print("recalled=" .. recalled.text)
print("project=" .. extracted.value.project)
print("owner=" .. extracted.value.owner)
print("remembered=" .. tostring(extracted.value.remembered))
print("source=" .. extracted.value.meta.source)
