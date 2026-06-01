// Agent-as-tool demo.
//
// This shows a supervisor agent delegating to a specialist agent that is
// listed *directly* in the supervisor's tools: [...] list. The runtime
// auto-wraps the agent value as a tool whose params/schema are derived from
// the agent's declared parameters and output: shape; the agent's structured
// result.value is fed back to the supervisor as the tool result.
//
// For the explicit `tool wrapper` form, see TestLLMAgentScenarioAgentAsToolStructuredHandoff.

models {
    default: "supervisor"
    supervisor: "mock-supervisor"
    extractor: "mock-extractor"
}

agent extract_research(topic) {
    model: "extractor"
    system: "Extract a structured research handoff."
    user: "Research " .. topic
    output: {
        summary: "short finding"
        confidence: 1
    }
}

agent supervisor(question) {
    model: "supervisor"
    system: "Use delegated specialist agents as tools before answering."
    user: question
    tools: [extract_research]
}

result, err := supervisor("Should this workflow delegate research?")

final_text := result.text
outer_history_len := #result.history

// Use history.find to locate the tool result without depending on index order.
tool_msg, _ := history.find(result.history, {role: "tool"})
delegated_summary := tool_msg.value.summary
