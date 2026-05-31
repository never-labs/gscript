// LLM agent incident response demo.
//
// The workflow is intentionally provider-agnostic: embedders can supply a mock,
// local, or remote llm provider. The script demonstrates three common agent
// shapes in one small application:
// 1. a direct single-turn FAQ answer,
// 2. an auto-dispatch research agent with tools,
// 3. a custom flow that manually dispatches tool calls while enforcing defaults.

models {
    default: "ops"
    ops: {
        provider_model: "mock-ops"
    }
    planner: "mock-planner"
}

//gscript:requires runbooks.read
//gscript:param service production service name
//gscript:param symptom current symptom or alert summary
tool search_runbook(service, symptom) {
    return "runbook:" .. service .. ":" .. symptom, nil
}

//gscript:requires metrics.read
//gscript:param service production service name
tool get_metrics(service) {
    return "metrics:" .. service .. ":latency=high,error_rate=2%", nil
}

agent defaults {
    model: "ops"
    tools: [search_runbook, get_metrics]
    budget: {turns: 5, calls: 4, tokens: 3000, time: 45}
}

agent answer_faq(question) {
    system: "Answer internal operations questions in one concise paragraph."
    user: question
    tools: []
    budget: {turns: 1, calls: 0, tokens: 256}
}

agent investigate_alert(service, symptom) {
    system: "Use runbook and metrics evidence before giving incident guidance."
    user: "Investigate " .. service .. " alert: " .. symptom
}

agent incident_brief(service, symptom) {
    model: "planner"
    system: "Create a brief incident update from verified tool evidence."
    tools: [search_runbook, get_metrics]
    budget: {turns: 2, calls: 2, tokens: 800}
} flow {
    history := messages {
        system: system
        user: "Prepare incident brief for " .. service .. ": " .. symptom
    }

    first, first_err := turn {
        model: model
        messages: history
        tools: tools
    }
    if first_err != nil {
        return nil, first_err
    }

    call := first.calls[1]
    evidence, dispatch_err := llm.dispatch(call, tools)
    if dispatch_err != nil {
        return nil, dispatch_err
    }
    history[#history + 1] = msg.assistant_call(call)
    history[#history + 1] = msg.tool_result(call.id, evidence)

    final, final_err := turn {
        model: model
        messages: history
        tools: tools
        max_tokens: 320
    }
    return {
        status: final.status,
        text: final.text,
        evidence: evidence,
        history_len: #history
    }, final_err
}

faq, faq_err := answer_faq("Who owns checkout incidents?")
research, research_err := investigate_alert("checkout", "p95 latency spike")
brief, brief_err := incident_brief("checkout", "p95 latency spike")

faq_text := faq.text
research_text := research.text
brief_text := brief.text
brief_evidence := brief.evidence
brief_history_len := brief.history_len
