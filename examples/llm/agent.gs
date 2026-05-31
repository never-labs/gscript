// Minimal AI-native syntax sample.
//
// This file demonstrates parser/tooling support for the AI-native surface:
// models, agent defaults, tool declarations, agents, messages, turns, and
// budget blocks. Running it requires the LLM stdlib with a host/provider
// configuration; `gscript fmt` and `gscript lint` can validate it without
// contacting a provider.

models {
    default: "demo"
    demo: {
        provider_model: "mock-demo"
    }
}

// lookup searches local documentation.
//gscript:requires docs.read
//gscript:param query natural-language search query
tool lookup(query) {
    return "doc:" .. query, nil
}

agent defaults {
    model: "demo"
    tools: [lookup]
    budget: {turns: 4, calls: 4, tokens: 2000, time: 30s}
}

agent answer(question) {
    system: "Use local documentation when useful."
    user: question
    tools: [lookup]
}

agent review(question) {
    system: "Return a short answer."
    user: question
} flow {
    history := messages {
        system: system
        user: question
    }
    result, err := turn {
        model: model
        messages: history
        tools: tools
    }
    return result, err
}

budget { turns: 1 } {
    result, err := answer("What is GScript?")
    _ = result
    _ = err
}
