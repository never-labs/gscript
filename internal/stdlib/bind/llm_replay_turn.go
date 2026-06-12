package bind

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/never-labs/leia/internal/runtime"
)

const llmReplayModeFixture = "fixture_replay"

type llmTurnReplayPlan struct {
	enabled      bool
	mode         string
	replayKey    string
	requestHash  string
	responseHash string
	response     Value
}

func llmTurnReplayPlanFromOptions(opts *Table) llmTurnReplayPlan {
	if opts == nil {
		return llmTurnReplayPlan{}
	}
	replay := opts.RawGetString("replay")
	if !replay.IsTable() {
		return llmTurnReplayPlan{}
	}
	rt := replay.Table()
	mode := rt.RawGetString("mode").Str()
	if mode != llmReplayModeFixture {
		return llmTurnReplayPlan{}
	}
	return llmTurnReplayPlan{
		enabled:      true,
		mode:         mode,
		replayKey:    rt.RawGetString("replay_key").Str(),
		requestHash:  rt.RawGetString("request_hash").Str(),
		responseHash: rt.RawGetString("response_hash").Str(),
		response:     rt.RawGetString("response"),
	}
}

func llmTurnReplayValidate(plan llmTurnReplayPlan, req runtime.LLMTurnRequest) Value {
	if !plan.enabled {
		return NilValue()
	}
	if plan.mode != llmReplayModeFixture {
		return llmErrorValue("validation", fmt.Sprintf("llm.turn replay mode %q is unsupported", plan.mode))
	}
	if plan.response.IsNil() {
		return llmErrorValue("validation", "llm.turn replay requires replay.response")
	}
	actualHash := llmTurnRequestHash(req)
	if plan.requestHash != "" && plan.requestHash != actualHash {
		return llmErrorValue("validation", fmt.Sprintf("llm.turn replay request_hash mismatch: got %s want %s", actualHash, plan.requestHash))
	}
	return NilValue()
}

func llmTurnReplayResult(plan llmTurnReplayPlan, req runtime.LLMTurnRequest) Value {
	result := llmCloneReplayResponse(plan.response)
	if !result.IsTable() {
		result = llmResultValue(runtime.LLMTurnResult{Status: "final_answer", Text: result.Str()})
	}
	rt := result.Table()
	replay := NewTable()
	replay.RawSetString("mode", StringValue(plan.mode))
	replay.RawSetString("provider_free", BoolValue(true))
	replay.RawSetString("live_network", BoolValue(false))
	replay.RawSetString("created_from_provider", BoolValue(false))
	if plan.replayKey != "" {
		replay.RawSetString("replay_key", StringValue(plan.replayKey))
	}
	requestHash := llmTurnRequestHash(req)
	replay.RawSetString("request_hash", StringValue(requestHash))
	responseHash := plan.responseHash
	if responseHash == "" {
		responseHash = llmStableValueHash(plan.response)
	}
	replay.RawSetString("response_hash", StringValue(responseHash))
	rt.RawSetString("replay", TableValue(replay))
	return result
}

func llmCloneReplayResponse(v Value) Value {
	if !v.IsTable() {
		return v
	}
	return TableValue(llmCloneTable(v.Table()))
}

func llmTurnRequestHash(req runtime.LLMTurnRequest) string {
	type replayMessage struct {
		Role      string `json:"role,omitempty"`
		Text      string `json:"text,omitempty"`
		ToolUseID string `json:"tool_use_id,omitempty"`
		Error     string `json:"error,omitempty"`
		Value     any    `json:"value,omitempty"`
		ToolCall  any    `json:"tool_call,omitempty"`
	}
	type replayTool struct {
		Name        string   `json:"name,omitempty"`
		Description string   `json:"description,omitempty"`
		Params      []string `json:"params,omitempty"`
		Requires    []string `json:"requires,omitempty"`
		Schema      any      `json:"schema,omitempty"`
	}
	payload := struct {
		Model          string            `json:"model,omitempty"`
		Messages       []replayMessage   `json:"messages,omitempty"`
		Tools          []replayTool      `json:"tools,omitempty"`
		ForceTool      string            `json:"force_tool,omitempty"`
		MaxTokens      int64             `json:"max_tokens,omitempty"`
		Temperature    *float64          `json:"temperature,omitempty"`
		TopP           *float64          `json:"top_p,omitempty"`
		ResponseFormat any               `json:"response_format,omitempty"`
		Stop           []string          `json:"stop,omitempty"`
		Metadata       map[string]string `json:"metadata,omitempty"`
	}{
		Model:          req.Model,
		ForceTool:      req.ForceTool,
		MaxTokens:      req.MaxTokens,
		Temperature:    req.Temperature,
		TopP:           req.TopP,
		ResponseFormat: req.ResponseFormat,
		Stop:           append([]string(nil), req.Stop...),
		Metadata:       req.Metadata,
	}
	for _, msg := range req.Messages {
		rm := replayMessage{
			Role:      msg.Role,
			Text:      msg.Text,
			ToolUseID: msg.ToolUseID,
			Error:     msg.Error,
			Value:     msg.Value,
		}
		if msg.ToolCall != nil {
			rm.ToolCall = map[string]any{"id": msg.ToolCall.ID, "tool": msg.ToolCall.Tool, "args": msg.ToolCall.Args}
		}
		payload.Messages = append(payload.Messages, rm)
	}
	for _, tool := range req.Tools {
		payload.Tools = append(payload.Tools, replayTool{
			Name:        tool.Name,
			Description: tool.Description,
			Params:      append([]string(nil), tool.Params...),
			Requires:    append([]string(nil), tool.Requires...),
			Schema:      tool.Schema,
		})
	}
	return llmStableJSONHash(payload)
}

func llmStableValueHash(v Value) string {
	return llmStableJSONHash(llmAnyFromValue(v))
}

func llmStableJSONHash(v any) string {
	normalized := llmStableNormalize(v)
	bytes, _ := json.Marshal(normalized)
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func llmStableNormalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(x))
		for _, k := range keys {
			out[k] = llmStableNormalize(x[k])
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = llmStableNormalize(item)
		}
		return out
	default:
		return x
	}
}
