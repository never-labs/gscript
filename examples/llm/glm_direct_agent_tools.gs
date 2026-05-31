// Real GLM AI-native direct agent-as-tool demo.
//
// This mirrors the local glm_cc wrapper's environment shape without invoking it:
//   endpoint: GSCRIPT_GLM_BASE_URL, default https://open.bigmodel.cn/api/anthropic
//   key:      GSCRIPT_GLM_API_KEY, SENTINEL_GLM_API_KEY, or GLM_API_KEY
//   model:    GSCRIPT_GLM_MODEL or GLM_MODEL, default glm-5.1
//
// Run without committing tokens:
//   GSCRIPT_GLM_API_KEY=... gscript -jit=false examples/llm/glm_direct_agent_tools.gs

func env_first(a, b, c, fallback) {
    v := os.getenv(a)
    if v != "" {
        return v
    }
    v = os.getenv(b)
    if v != "" {
        return v
    }
    v = os.getenv(c)
    if v != "" {
        return v
    }
    return fallback
}

models {
    default: "glm-smoke"
    "glm-smoke": {
        provider: "glm"
        protocol: "anthropic_compatible"
        base_url: env_first("GSCRIPT_GLM_BASE_URL", "ANTHROPIC_BASE_URL", "", "https://open.bigmodel.cn/api/anthropic")
        api_key: env_first("GSCRIPT_GLM_API_KEY", "SENTINEL_GLM_API_KEY", "GLM_API_KEY", "")
        provider_model: env_first("GSCRIPT_GLM_MODEL", "GLM_MODEL", "ANTHROPIC_MODEL", "glm-5.1")
    }
}

agent extract_memory(note) {
    model: "glm-smoke"
    system: "Return only compact JSON. Extract the project codename and owner from the user note."
    user: note
    output: {
        project: "ORCHID"
        owner: "ADA"
        remembered: true
        source: "direct-agent-tool"
    }
    max_tokens: 96
    temperature: 0
}

agent supervisor(question) {
    model: "glm-smoke"
    system: "You are testing tool use. You must call extract_memory exactly once before answering. After the tool result, answer in one short sentence that includes DIRECT_AGENT_TOOL_OK."
    user: question
    tools: [extract_memory]
    max_steps: 4
    max_tokens: 128
    temperature: 0
}

result, err := supervisor("Use the extract_memory tool with this note: project codename is ORCHID and owner is ADA. Do not answer from memory; call the tool first.")
if err != nil {
    print(err.message)
    return
}

print("text=" .. result.text)
print("history_len=" .. tostring(#result.history))
print("roles=" .. result.history[1].role .. "/" .. result.history[2].role .. "/" .. result.history[3].role .. "/" .. result.history[4].role)
print("tool=" .. result.history[3].tool_call.tool)
print("project=" .. result.history[4].value.project)
print("owner=" .. result.history[4].value.owner)
print("source=" .. result.history[4].value.source)
