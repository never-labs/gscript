// AI-native agent-as-tool demo.
//
// This shows a supervisor agent dispatching a normal tool whose implementation
// calls a specialist agent. The specialist returns structured output, and the
// tool returns that table into the supervisor's ReAct history as a tool result.

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

// gscript:requires none
// gscript:param topic research topic delegated by the supervisor
tool delegate_research(topic) {
    result, err := extract_research(topic)
    if err != nil {
        return nil, err
    }
    return {
        topic: topic
        summary: result.value.summary
        confidence: result.value.confidence
    }, nil
}

agent supervisor(question) {
    model: "supervisor"
    system: "Use delegated specialist agents as tools before answering."
    user: question
    tools: [delegate_research]
}

result, err := supervisor("Should this workflow delegate research?")

final_text := result.text
outer_history_len := #result.history
delegated_summary := result.history[4].value.summary
