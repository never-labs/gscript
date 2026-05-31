package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	stdlibai "github.com/never-labs/gscript/internal/stdlib/ai"
)

func BuildLLMLoopLib(call ScriptFunctionCaller, provider func() LLMProvider, maxHostResult func() int64, ctx func() context.Context, traces ...LLMTraceSink) *Table {
	t := NewTable()
	snapshots := map[string]Value{}
	var snapshotsMu sync.Mutex

	hostLimit := func() int64 {
		if maxHostResult == nil {
			return 0
		}
		return maxHostResult()
	}
	currentProvider := func() LLMProvider {
		if provider == nil {
			return nil
		}
		return provider()
	}
	currentContext := func() context.Context {
		if ctx == nil || ctx() == nil {
			return context.Background()
		}
		return ctx()
	}
	trace := func(event LLMTraceEvent) {
		if len(traces) == 0 || traces[0] == nil {
			return
		}
		traces[0](event)
	}
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{Name: "loop." + name, Fn: fn}))
	}

	set("react", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'loop.react' (table expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("loop.react requires a function caller")
		}
		p := currentProvider()
		if p == nil {
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "provider", Message: "llm provider not configured"})
			return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
		}
		opts, err := llmLoopOptions(args[0].Table(), 0)
		if err != nil {
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "validation", Message: err.Error()})
			return []Value{NilValue(), llmErrorValue("validation", err.Error())}, nil
		}
		return llmReact(opts, p, call, currentContext(), hostLimit(), trace, nil, &llmHITL{
			approveWhen: args[0].Table().RawGetString("approve_when"),
			store:       args[0].Table().RawGetString("store"),
			trace:       trace,
			snapshots:   snapshots,
			snapshotsMu: &snapshotsMu,
		})
	})

	set("simple", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'loop.simple' (table expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("loop.simple requires a function caller")
		}
		p := currentProvider()
		if p == nil {
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "provider", Message: "llm provider not configured"})
			return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
		}
		opts, err := llmLoopOptions(args[0].Table(), 1)
		if err != nil {
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "validation", Message: err.Error()})
			return []Value{NilValue(), llmErrorValue("validation", err.Error())}, nil
		}
		return llmReact(opts, p, call, currentContext(), hostLimit(), trace, nil)
	})

	set("plan_execute", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'loop.plan_execute' (table expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("loop.plan_execute requires a function caller")
		}
		p := currentProvider()
		if p == nil {
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "provider", Message: "llm provider not configured"})
			return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
		}
		opts, err := llmLoopOptions(args[0].Table(), 0)
		if err != nil {
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "validation", Message: err.Error()})
			return []Value{NilValue(), llmErrorValue("validation", err.Error())}, nil
		}
		planResult, planErr := llmPlanTurn(args[0].Table(), opts, p, currentContext(), hostLimit(), trace)
		if !planErr.IsNil() {
			return []Value{NilValue(), planErr}, nil
		}
		llmInjectPlan(opts, planResult.Text)
		if execModel := args[0].Table().RawGetString("exec_model"); !execModel.IsNil() {
			opts.RawSetString("model", execModel)
		}
		result, err := llmReact(opts, p, call, currentContext(), hostLimit(), trace, nil, &llmHITL{
			approveWhen: args[0].Table().RawGetString("approve_when"),
			store:       args[0].Table().RawGetString("store"),
			trace:       trace,
			snapshots:   snapshots,
			snapshotsMu: &snapshotsMu,
		})
		if err != nil || len(result) == 0 || !result[1].IsNil() || !result[0].IsTable() {
			return result, err
		}
		result[0].Table().RawSetString("plan", StringValue(planResult.Text))
		return result, nil
	})

	set("reflect", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'loop.reflect' (table expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("loop.reflect requires a function caller")
		}
		p := currentProvider()
		if p == nil {
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "provider", Message: "llm provider not configured"})
			return []Value{NilValue(), llmErrorValue("provider", "llm provider not configured")}, nil
		}
		opts, err := llmLoopOptions(args[0].Table(), 0)
		if err != nil {
			trace(LLMTraceEvent{Type: "react_error", ErrorKind: "validation", Message: err.Error()})
			return []Value{NilValue(), llmErrorValue("validation", err.Error())}, nil
		}
		result, err := llmReact(opts, p, call, currentContext(), hostLimit(), trace, nil, &llmHITL{
			approveWhen: args[0].Table().RawGetString("approve_when"),
			store:       args[0].Table().RawGetString("store"),
			trace:       trace,
			snapshots:   snapshots,
			snapshotsMu: &snapshotsMu,
		})
		if err != nil || len(result) == 0 || !result[1].IsNil() || !result[0].IsTable() {
			return result, err
		}
		if reflectErr := llmReflectResult(args[0].Table(), result[0].Table(), p, currentContext(), hostLimit(), trace); !reflectErr.IsNil() {
			return []Value{NilValue(), reflectErr}, nil
		}
		return result, nil
	})

	set("snapshot", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() || !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument to 'loop.snapshot' (history, pending_call expected)")
		}
		token, err := llmSnapshotToken()
		if err != nil {
			return nil, err
		}
		snapshot := NewTable()
		snapshot.RawSetString("history", args[0])
		snapshot.RawSetString("pending", args[1])
		snapshotsMu.Lock()
		snapshots[token] = TableValue(snapshot)
		snapshotsMu.Unlock()
		trace(LLMTraceEvent{Type: "snapshot_saved", Token: token})
		if len(args) >= 3 && llmIsSnapshotStore(args[2]) {
			if errVal, err := llmStoreSave(call, args[2].Table(), token, TableValue(snapshot)); err != nil {
				return nil, err
			} else if !errVal.IsNil() {
				return []Value{NilValue(), errVal}, nil
			}
			trace(LLMTraceEvent{Type: "snapshot_store_saved", Token: token, Store: true})
		}
		return []Value{StringValue(token), NilValue()}, nil
	})

	set("resume", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument to 'loop.resume' (token, approval expected)")
		}
		token := args[0].Str()
		var tools *Table
		var store Value
		if len(args) >= 3 && args[2].IsTable() {
			if llmIsSnapshotStore(args[2]) {
				store = args[2]
			} else {
				tools = args[2].Table()
			}
		}
		if len(args) >= 4 && llmIsSnapshotStore(args[3]) {
			store = args[3]
		}
		trace(LLMTraceEvent{Type: "resume_start", Token: token, Store: llmIsSnapshotStore(store)})
		snapshotsMu.Lock()
		snapshot, ok := snapshots[token]
		if !ok || !snapshot.IsTable() {
			snapshotsMu.Unlock()
			if llmIsSnapshotStore(store) {
				loaded, errVal, err := llmStoreLoad(call, store.Table(), token)
				if err != nil {
					return nil, err
				}
				if !errVal.IsNil() {
					return []Value{NilValue(), errVal}, nil
				}
				snapshot = loaded
				trace(LLMTraceEvent{Type: "resume_loaded", Token: token, Store: true})
			}
			if !snapshot.IsTable() {
				return []Value{NilValue(), llmErrorValue("validation", "snapshot not found")}, nil
			}
		} else {
			delete(snapshots, token)
			snapshotsMu.Unlock()
			trace(LLMTraceEvent{Type: "resume_loaded", Token: token})
		}
		result, err := llmResumeSnapshot(snapshot.Table(), args[1].Table(), tools, call)
		if err != nil {
			return nil, err
		}
		if llmIsSnapshotStore(store) {
			if errVal, err := llmStoreDelete(call, store.Table(), token); err != nil {
				return nil, err
			} else if !errVal.IsNil() {
				return []Value{NilValue(), errVal}, nil
			}
			trace(LLMTraceEvent{Type: "resume_store_deleted", Token: token, Store: true})
		}
		if len(result) > 0 && result[0].IsTable() {
			trace(LLMTraceEvent{Type: "resume_done", Token: token, Status: result[0].Table().RawGetString("status").Str(), Store: llmIsSnapshotStore(store)})
		}
		return result, nil
	})

	return t
}
func llmLoopOptions(src *Table, defaultMaxSteps int64) (*Table, error) {
	opts := NewTable()
	messages := src.RawGetString("messages")
	user := src.RawGetString("user")
	plan, err := stdlibai.NormalizeLoopOptions(stdlibai.LoopOptionInput{
		HasMessages:         messages.IsTable(),
		HasUser:             !user.IsNil(),
		HasMaxSteps:         !src.RawGetString("max_steps").IsNil(),
		HasResponseFormat:   !src.RawGetString("response_format").IsNil(),
		HasStructuredOutput: src.RawGetString("output").IsTable(),
		DefaultMaxSteps:     defaultMaxSteps,
	})
	if err != nil {
		return nil, err
	}
	if !plan.SynthesizeMessages {
		opts.RawSetString("messages", messages)
	} else {
		messages := NewAppendArrayTable(2)
		if system := src.RawGetString("system"); !system.IsNil() {
			messages.RawSet(IntValue(int64(messages.Length()+1)), TableValue(llmMessageTable("system", system.Str())))
		}
		messages.RawSet(IntValue(int64(messages.Length()+1)), TableValue(llmMessageTable("user", user.Str())))
		opts.RawSetString("messages", TableValue(messages))
	}
	for _, key := range stdlibai.LoopOptionKeys {
		if v := src.RawGetString(key); !v.IsNil() {
			opts.RawSetString(key, v)
		}
	}
	if plan.SetDefaultMaxSteps {
		opts.RawSetString("max_steps", IntValue(plan.DefaultMaxSteps))
	}
	if plan.SetJSONResponseFormat {
		opts.RawSetString("response_format", TableValue(llmJSONResponseFormatTable()))
	}
	return opts, nil
}

type llmHITL struct {
	approveWhen Value
	store       Value
	trace       func(LLMTraceEvent)
	snapshots   map[string]Value
	snapshotsMu *sync.Mutex
}

func llmReact(opts *Table, provider LLMProvider, call ScriptFunctionCaller, ctx context.Context, maxHostResult int64, trace func(LLMTraceEvent), ambient llmBudgetGroup, hitls ...*llmHITL) ([]Value, error) {
	var hitl *llmHITL
	if len(hitls) > 0 {
		hitl = hitls[0]
	}
	messagesValue := opts.RawGetString("messages")
	if !messagesValue.IsTable() {
		llmTrace(trace, LLMTraceEvent{Type: "react_error", ErrorKind: "validation", Message: "llm.react requires messages"})
		return []Value{NilValue(), llmErrorValue("validation", "llm.react requires messages")}, nil
	}
	toolsValue := opts.RawGetString("tools")
	tools := toolsValue.Table()
	if tools == nil {
		tools = NewTable()
	}
	history := llmMessageValuesFromTable(messagesValue.Table())
	controls := stdlibai.NormalizeReactControls(
		toInt(opts.RawGetString("max_steps")),
		toInt(opts.RawGetString("max_tool_retries")),
		toInt(opts.RawGetString("max_history_tokens")),
	)
	model := opts.RawGetString("model").Str()
	budgets := ambient.with(llmBudgetFromOptions(opts))
	cancel := llmCancelFromOptions(opts, ctx)
	for step := 0; step < controls.MaxSteps; step++ {
		if err := cancel.check(); !err.IsNil() {
			llmTrace(trace, LLMTraceEvent{Type: "react_error", ErrorKind: llmErrorKind(err), Message: err.Table().RawGetString("message").Str()})
			return []Value{NilValue(), err}, nil
		}
		if err := budgets.beforeTurn(); !err.IsNil() {
			llmTrace(trace, LLMTraceEvent{Type: "react_error", ErrorKind: llmErrorKind(err), Message: err.Table().RawGetString("message").Str()})
			return []Value{NilValue(), err}, nil
		}
		requestHistory := history
		if controls.MaxHistoryTokens > 0 {
			requestHistory = chatWindow(llmTableFromValues(history).Table(), controls.MaxHistoryTokens)
		}
		req := LLMTurnRequest{
			Model:          model,
			Messages:       llmMessagesFromValue(llmTableFromValues(requestHistory)),
			Tools:          llmToolsFromValue(toolsValue),
			ForceTool:      llmForceToolFromValue(opts.RawGetString("force_tool")),
			MaxTokens:      toInt(opts.RawGetString("max_tokens")),
			Temperature:    llmOptionalFloatFromValue(opts.RawGetString("temperature")),
			TopP:           llmOptionalFloatFromValue(opts.RawGetString("top_p")),
			ResponseFormat: llmAnyFromValue(opts.RawGetString("response_format")),
			Stream:         opts.RawGetString("stream").Truthy(),
			Stop:           llmStringSliceFromValue(opts.RawGetString("stop")),
			Metadata:       llmStringMapFromValue(opts.RawGetString("metadata")),
		}
		llmTrace(trace, LLMTraceEvent{Type: "turn_start", Model: req.Model, Step: int64(step), MessageCount: len(req.Messages), ToolCount: len(req.Tools)})
		res, err := provider.Turn(ctx, req)
		if err != nil {
			llmTrace(trace, LLMTraceEvent{Type: "turn_error", Model: req.Model, Step: int64(step), ErrorKind: ClassifyLLMProviderError(err), Message: err.Error()})
			return []Value{NilValue(), llmProviderErrorValue(err)}, nil
		}
		if err := cancel.check(); !err.IsNil() {
			llmTrace(trace, LLMTraceEvent{Type: "react_error", ErrorKind: llmErrorKind(err), Message: err.Table().RawGetString("message").Str()})
			return []Value{NilValue(), err}, nil
		}
		llmTrace(trace, LLMTraceEvent{Type: "turn_end", Model: req.Model, Step: int64(step), Status: llmResultStatus(res), MessageCount: len(req.Messages), ToolCount: len(req.Tools), Usage: res.Usage})
		turnValue := llmResultValue(res)
		if err := CheckHostResultBytes(maxHostResult, turnValue); err != nil {
			return nil, err
		}
		budgets.chargeTurn(res.Usage)
		switch res.Status {
		case "", "final_answer":
			result := llmReactResultValue("done", res.Text, "", turnValue, history)
			if value, errValue := llmStructuredOutputValue(opts, res.Text); !errValue.IsNil() {
				repairedValue, repairedTurn, repairedText, repairErr, repaired := llmRepairStructuredOutput(opts, provider, ctx, maxHostResult, trace, budgets, cancel, model, toolsValue, history, res.Text, errValue, int64(step))
				if !repairErr.IsNil() {
					llmTrace(trace, LLMTraceEvent{Type: "react_error", ErrorKind: llmErrorKind(repairErr), Message: repairErr.Table().RawGetString("message").Str()})
					return []Value{NilValue(), repairErr}, nil
				}
				if repaired {
					result = llmReactResultValue("done", repairedText, "", repairedTurn, history)
					result.Table().RawSetString("value", repairedValue)
					llmTrace(trace, LLMTraceEvent{Type: "react_done", Model: model, Step: int64(step), Status: "done"})
					return []Value{result, NilValue()}, nil
				}
				llmTrace(trace, LLMTraceEvent{Type: "react_error", ErrorKind: llmErrorKind(errValue), Message: errValue.Table().RawGetString("message").Str()})
				return []Value{NilValue(), errValue}, nil
			} else if !value.IsNil() {
				result.Table().RawSetString("value", value)
			}
			llmTrace(trace, LLMTraceEvent{Type: "react_done", Model: model, Step: int64(step), Status: "done"})
			return []Value{result, NilValue()}, nil
		case "stop":
			llmTrace(trace, LLMTraceEvent{Type: "react_stopped", Model: model, Step: int64(step), Status: "stopped", Message: res.Reason})
			return []Value{llmReactResultValue("stopped", "", res.Reason, turnValue, history), NilValue()}, nil
		case "tool_calls":
			for i := range res.Calls {
				callValue := llmToolCallValue(res.Calls[i])
				llmTrace(trace, LLMTraceEvent{Type: "tool_call", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID})
				if pending, err := llmMaybePauseForApproval(hitl, call, llmTableFromValues(history), callValue); !pending.IsNil() || err != nil {
					if err != nil {
						llmTrace(trace, LLMTraceEvent{Type: "tool_fatal", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID, ErrorKind: "internal", Message: err.Error()})
						return []Value{NilValue(), llmErrorValue("internal", err.Error())}, nil
					}
					llmTrace(trace, LLMTraceEvent{Type: "react_stopped", Model: model, Step: int64(step), Status: "pending", Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID})
					return []Value{pending, NilValue()}, nil
				}
				history = append(history, llmAssistantCallMessage(callValue))
				if err := cancel.check(); !err.IsNil() {
					llmTrace(trace, LLMTraceEvent{Type: "tool_fatal", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID, ErrorKind: llmErrorKind(err)})
					return []Value{NilValue(), err}, nil
				}
				if err := budgets.beforeToolCall(); !err.IsNil() {
					llmTrace(trace, LLMTraceEvent{Type: "tool_fatal", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID, ErrorKind: llmErrorKind(err)})
					return []Value{NilValue(), err}, nil
				}
				dispatchResult, err := llmDispatchWithRetry(call, callValue.Table(), tools, controls.MaxToolRetries, trace, int64(step), res.Calls[i])
				if !err.IsNil() {
					if llmErrorKind(err) == "pending" {
						pendingPayload := err.Table().RawGetString("pending")
						llmTrace(trace, LLMTraceEvent{Type: "react_stopped", Model: model, Step: int64(step), Status: "pending", Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID})
						if pendingPayload.IsTable() {
							pendingPayload.Table().RawSetString("history", llmTableFromValues(history))
							return []Value{pendingPayload, NilValue()}, nil
						}
						pending := NewTable()
						pending.RawSetString("status", StringValue("pending"))
						pending.RawSetString("history", llmTableFromValues(history))
						return []Value{TableValue(pending), NilValue()}, nil
					}
					llmTrace(trace, LLMTraceEvent{Type: "tool_fatal", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID, ErrorKind: llmErrorKind(err)})
					return []Value{NilValue(), err}, nil
				}
				if len(dispatchResult) >= 2 && !dispatchResult[1].IsNil() {
					if llmErrorKind(dispatchResult[1]) == "pending" {
						pendingPayload := dispatchResult[1].Table().RawGetString("pending")
						llmTrace(trace, LLMTraceEvent{Type: "react_stopped", Model: model, Step: int64(step), Status: "pending", Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID})
						if pendingPayload.IsTable() {
							pendingPayload.Table().RawSetString("history", llmTableFromValues(history))
							return []Value{pendingPayload, NilValue()}, nil
						}
						pending := NewTable()
						pending.RawSetString("status", StringValue("pending"))
						pending.RawSetString("history", llmTableFromValues(history))
						return []Value{TableValue(pending), NilValue()}, nil
					}
					message := dispatchResult[1].Table().RawGetString("message").Str()
					llmTrace(trace, LLMTraceEvent{Type: "tool_error", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID, ErrorKind: llmErrorKind(dispatchResult[1]), Message: message})
					history = append(history, llmToolErrorMessage(res.Calls[i].ID, message))
				} else {
					value := NilValue()
					if len(dispatchResult) > 0 {
						value = dispatchResult[0]
					}
					llmTrace(trace, LLMTraceEvent{Type: "tool_result", Step: int64(step), Tool: res.Calls[i].Tool, CallID: res.Calls[i].ID})
					history = append(history, llmToolResultMessage(res.Calls[i].ID, value))
				}
			}
		default:
			llmTrace(trace, LLMTraceEvent{Type: "react_stopped", Model: model, Step: int64(step), Status: "stopped", Message: res.Status})
			return []Value{llmReactResultValue("stopped", "", res.Status, turnValue, history), NilValue()}, nil
		}
	}
	llmTrace(trace, LLMTraceEvent{Type: "react_stopped", Model: model, Step: int64(controls.MaxSteps), Status: "stopped", Message: "max_steps"})
	return []Value{llmReactResultValue("stopped", "", "max_steps", NilValue(), history), NilValue()}, nil
}

func llmTrace(trace func(LLMTraceEvent), event LLMTraceEvent) {
	if trace != nil {
		trace(event)
	}
}

func llmMaybePauseForApproval(hitl *llmHITL, call ScriptFunctionCaller, history, pending Value) (Value, error) {
	if hitl == nil || hitl.approveWhen.IsNil() || !hitl.approveWhen.IsFunction() {
		return NilValue(), nil
	}
	results, err := call(hitl.approveWhen, []Value{pending})
	if err != nil {
		return NilValue(), err
	}
	if len(results) == 0 || !results[0].Truthy() {
		return NilValue(), nil
	}
	token, err := llmSnapshotToken()
	if err != nil {
		return NilValue(), err
	}
	snapshot := NewTable()
	snapshot.RawSetString("history", history)
	snapshot.RawSetString("pending", pending)
	hitl.snapshotsMu.Lock()
	hitl.snapshots[token] = TableValue(snapshot)
	hitl.snapshotsMu.Unlock()
	llmTrace(hitl.trace, LLMTraceEvent{Type: "snapshot_saved", Token: token, Tool: pending.Table().RawGetString("tool").Str(), CallID: pending.Table().RawGetString("id").Str()})
	if llmIsSnapshotStore(hitl.store) {
		if errVal, err := llmStoreSave(call, hitl.store.Table(), token, TableValue(snapshot)); err != nil {
			return NilValue(), err
		} else if !errVal.IsNil() {
			return NilValue(), fmt.Errorf("%s", errVal.String())
		}
		llmTrace(hitl.trace, LLMTraceEvent{Type: "snapshot_store_saved", Token: token, Store: true, Tool: pending.Table().RawGetString("tool").Str(), CallID: pending.Table().RawGetString("id").Str()})
	}

	result := NewTable()
	result.RawSetString("status", StringValue("pending"))
	result.RawSetString("token", StringValue(token))
	result.RawSetString("payload", pending)
	result.RawSetString("pending", pending)
	result.RawSetString("history", history)
	return TableValue(result), nil
}

func llmRepairStructuredOutput(opts *Table, provider LLMProvider, ctx context.Context, maxHostResult int64, trace func(LLMTraceEvent), budgets llmBudgetGroup, cancel llmCancel, model string, toolsValue Value, history []Value, previousText string, validationErr Value, step int64) (Value, Value, string, Value, bool) {
	maxRetries := llmOutputRepairRetries(opts)
	if maxRetries <= 0 {
		return NilValue(), NilValue(), "", NilValue(), false
	}
	lastErr := validationErr
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := cancel.check(); !err.IsNil() {
			return NilValue(), NilValue(), "", err, true
		}
		if err := budgets.beforeTurn(); !err.IsNil() {
			return NilValue(), NilValue(), "", err, true
		}
		repairHistory := llmStructuredOutputRepairHistory(opts, history, previousText, lastErr)
		req := LLMTurnRequest{
			Model:          model,
			Messages:       llmMessagesFromValue(llmTableFromValues(repairHistory)),
			Tools:          llmToolsFromValue(toolsValue),
			ForceTool:      llmForceToolFromValue(opts.RawGetString("force_tool")),
			MaxTokens:      toInt(opts.RawGetString("max_tokens")),
			Temperature:    llmOptionalFloatFromValue(opts.RawGetString("temperature")),
			TopP:           llmOptionalFloatFromValue(opts.RawGetString("top_p")),
			ResponseFormat: llmAnyFromValue(opts.RawGetString("response_format")),
			Stream:         opts.RawGetString("stream").Truthy(),
			Stop:           llmStringSliceFromValue(opts.RawGetString("stop")),
			Metadata:       llmStringMapFromValue(opts.RawGetString("metadata")),
		}
		llmTrace(trace, LLMTraceEvent{Type: "turn_start", Model: req.Model, Step: step, Attempt: int64(attempt), MessageCount: len(req.Messages), ToolCount: len(req.Tools)})
		res, err := provider.Turn(ctx, req)
		if err != nil {
			llmTrace(trace, LLMTraceEvent{Type: "turn_error", Model: req.Model, Step: step, Attempt: int64(attempt), ErrorKind: ClassifyLLMProviderError(err), Message: err.Error()})
			return NilValue(), NilValue(), "", llmProviderErrorValue(err), true
		}
		if err := cancel.check(); !err.IsNil() {
			return NilValue(), NilValue(), "", err, true
		}
		llmTrace(trace, LLMTraceEvent{Type: "turn_end", Model: req.Model, Step: step, Attempt: int64(attempt), Status: llmResultStatus(res), MessageCount: len(req.Messages), ToolCount: len(req.Tools), Usage: res.Usage})
		turnValue := llmResultValue(res)
		if err := CheckHostResultBytes(maxHostResult, turnValue); err != nil {
			return NilValue(), NilValue(), "", llmErrorValue("internal", err.Error()), true
		}
		budgets.chargeTurn(res.Usage)
		if llmResultStatus(res) != "final_answer" {
			lastErr = llmErrorValue("validation", "structured output repair did not return a final answer")
			previousText = res.Text
			continue
		}
		value, errValue := llmStructuredOutputValue(opts, res.Text)
		if errValue.IsNil() {
			return value, turnValue, res.Text, NilValue(), true
		}
		lastErr = errValue
		previousText = res.Text
	}
	return NilValue(), NilValue(), "", lastErr, true
}

func llmOutputRepairRetries(opts *Table) int {
	if opts == nil {
		return 0
	}
	retries := int(toInt(opts.RawGetString("output_retries")))
	if retries < 0 {
		retries = 0
	}
	if retries == 0 && opts.RawGetString("output_repair").Truthy() {
		retries = 1
	}
	return retries
}

func llmStructuredOutputRepairHistory(opts *Table, history []Value, previousText string, validationErr Value) []Value {
	repairHistory := make([]Value, 0, len(history)+1)
	repairHistory = append(repairHistory, history...)
	repairHistory = append(repairHistory, TableValue(llmMessageTable("user", llmStructuredOutputRepairPrompt(opts, previousText, validationErr))))
	return repairHistory
}

func llmStructuredOutputRepairPrompt(opts *Table, previousText string, validationErr Value) string {
	prompt := ""
	if repair := opts.RawGetString("output_repair"); repair.IsString() {
		prompt = strings.TrimSpace(repair.Str())
	}
	if prompt == "" {
		prompt = "Return only a JSON object that matches the requested output shape."
	}
	message := ""
	if validationErr.IsTable() {
		message = validationErr.Table().RawGetString("message").Str()
	}
	shape := llmStructuredOutputShapeJSON(opts)
	var b strings.Builder
	b.WriteString(prompt)
	if message != "" {
		b.WriteString("\nValidation error: ")
		b.WriteString(message)
	}
	if shape != "" {
		b.WriteString("\nOutput shape example: ")
		b.WriteString(shape)
	}
	b.WriteString("\nPrevious response:\n")
	b.WriteString(previousText)
	return b.String()
}

func llmStructuredOutputShapeJSON(opts *Table) string {
	if opts == nil || !opts.RawGetString("output").IsTable() {
		return ""
	}
	data := llmAnyFromValue(opts.RawGetString("output"))
	encoded, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(encoded)
}

type llmBudget struct {
	maxTokens int64
	maxTurns  int64
	maxCalls  int64
	maxMoney  float64
	maxTime   time.Duration

	usedTokens int64
	usedTurns  int64
	usedCalls  int64
	usedMoney  float64
	started    time.Time
}

type llmBudgetGroup []*llmBudget

func (g llmBudgetGroup) with(b *llmBudget) llmBudgetGroup {
	if b == nil {
		return g
	}
	out := make(llmBudgetGroup, 0, len(g)+1)
	out = append(out, g...)
	out = append(out, b)
	return out
}

func (g llmBudgetGroup) beforeTurn() Value {
	for _, b := range g {
		if err := b.checkTurn(); !err.IsNil() {
			return err
		}
	}
	for _, b := range g {
		b.startTurn()
	}
	return NilValue()
}

func (g llmBudgetGroup) chargeTurn(usage LLMTurnUsage) {
	for _, b := range g {
		b.chargeTurn(usage)
	}
}

func (g llmBudgetGroup) beforeToolCall() Value {
	for _, b := range g {
		if err := b.checkToolCall(); !err.IsNil() {
			return err
		}
	}
	for _, b := range g {
		b.startToolCall()
	}
	return NilValue()
}

func newLLMBudget() *llmBudget {
	return &llmBudget{
		maxTokens: -1,
		maxTurns:  -1,
		maxCalls:  -1,
		maxMoney:  -1,
		maxTime:   -1,
	}
}

func llmBudgetFromOptions(opts *Table) *llmBudget {
	b := newLLMBudget()
	if v := opts.RawGetString("budget_tokens"); !v.IsNil() {
		b.maxTokens = toInt(v)
	}
	if v := opts.RawGetString("budget_turns"); !v.IsNil() {
		b.maxTurns = toInt(v)
	}
	if v := opts.RawGetString("budget_calls"); !v.IsNil() {
		b.maxCalls = toInt(v)
	}
	if v := opts.RawGetString("budget_money"); !v.IsNil() {
		b.maxMoney = toFloat(v)
	}
	if v := opts.RawGetString("budget_time"); !v.IsNil() {
		b.maxTime = llmBudgetDuration(v)
	}
	if t := opts.RawGetString("budget"); t.IsTable() {
		llmApplyBudgetConfig(b, t.Table())
	}
	llmNormalizeBudget(b)
	return b
}

func llmBudgetFromConfig(config *Table) *llmBudget {
	b := newLLMBudget()
	llmApplyBudgetConfig(b, config)
	llmNormalizeBudget(b)
	return b
}

func llmApplyBudgetConfig(b *llmBudget, config *Table) {
	if b == nil || config == nil {
		return
	}
	if v := config.RawGetString("tokens"); !v.IsNil() {
		b.maxTokens = toInt(v)
	}
	if v := config.RawGetString("turns"); !v.IsNil() {
		b.maxTurns = toInt(v)
	}
	if v := config.RawGetString("calls"); !v.IsNil() {
		b.maxCalls = toInt(v)
	}
	if v := config.RawGetString("money"); !v.IsNil() {
		b.maxMoney = toFloat(v)
	}
	if v := config.RawGetString("time"); !v.IsNil() {
		b.maxTime = llmBudgetDuration(v)
	}
}

func llmNormalizeBudget(b *llmBudget) {
	if b == nil {
		return
	}
	if b.maxTokens < 0 {
		b.maxTokens = -1
	}
	if b.maxTurns < 0 {
		b.maxTurns = -1
	}
	if b.maxCalls < 0 {
		b.maxCalls = -1
	}
	if b.maxMoney < 0 {
		b.maxMoney = -1
	}
	if b.maxTime < 0 {
		b.maxTime = -1
	}
	if b.maxTime >= 0 {
		b.started = time.Now()
	}
}

func (b *llmBudget) beforeTurn() Value {
	if b == nil {
		return NilValue()
	}
	if err := b.checkTurn(); !err.IsNil() {
		return err
	}
	b.startTurn()
	return NilValue()
}

func (b *llmBudget) checkTurn() Value {
	if b == nil {
		return NilValue()
	}
	if err := b.beforeWork(); !err.IsNil() {
		return err
	}
	if b.maxTurns >= 0 && b.usedTurns >= b.maxTurns {
		return llmBudgetError("turns", b.maxTurns, b.usedTurns)
	}
	if b.maxTokens >= 0 && b.usedTokens >= b.maxTokens {
		return llmBudgetError("tokens", b.maxTokens, b.usedTokens)
	}
	if b.maxMoney >= 0 && b.usedMoney >= b.maxMoney {
		return llmBudgetError("money", 0, 0)
	}
	return NilValue()
}

func (b *llmBudget) startTurn() {
	if b != nil {
		b.usedTurns++
	}
}

func (b *llmBudget) chargeTurn(usage LLMTurnUsage) {
	if b == nil {
		return
	}
	b.usedTokens += usage.InputTokens + usage.OutputTokens
	b.usedMoney += usage.Cost
}

func (b *llmBudget) beforeToolCall() Value {
	if b == nil {
		return NilValue()
	}
	if err := b.checkToolCall(); !err.IsNil() {
		return err
	}
	b.startToolCall()
	return NilValue()
}

func (b *llmBudget) checkToolCall() Value {
	if b == nil {
		return NilValue()
	}
	if err := b.beforeWork(); !err.IsNil() {
		return err
	}
	if b.maxCalls >= 0 && b.usedCalls >= b.maxCalls {
		return llmBudgetError("calls", b.maxCalls, b.usedCalls)
	}
	return NilValue()
}

func (b *llmBudget) startToolCall() {
	if b != nil {
		b.usedCalls++
	}
}

func (b *llmBudget) beforeWork() Value {
	if b.maxTime >= 0 && !b.started.IsZero() && time.Since(b.started) >= b.maxTime {
		return llmCancelError("deadline exceeded")
	}
	return NilValue()
}

func llmBudgetDuration(v Value) time.Duration {
	secs := toFloat(v)
	if secs < 0 {
		return -1
	}
	return time.Duration(secs * float64(time.Second))
}

func llmBudgetError(dimension string, limit, used int64) Value {
	shape := stdlibai.BudgetExceededError(dimension, limit, used)
	t := NewTable()
	t.RawSetString("kind", StringValue(shape.Kind))
	t.RawSetString("dimension", StringValue(shape.Dimension))
	t.RawSetString("message", StringValue(shape.Message))
	if shape.Limit > 0 {
		t.RawSetString("limit", IntValue(shape.Limit))
	}
	if shape.Used > 0 {
		t.RawSetString("used", IntValue(shape.Used))
	}
	return TableValue(t)
}

type llmCancel struct {
	host context.Context
	done *Channel
	err  Value
}

func llmCancelFromOptions(opts *Table, host context.Context) llmCancel {
	c := llmCancel{host: host}
	for _, key := range []string{"ctx", "context", "cancel"} {
		if done, errFn, ok := contextDoneAndErr(opts.RawGetString(key)); ok {
			c.done = done
			c.err = errFn
			break
		}
	}
	return c
}

func (c llmCancel) check() Value {
	if c.host != nil {
		if err := c.host.Err(); err != nil {
			return llmCancelError(err.Error())
		}
	}
	if c.done != nil {
		if reason, cancelled := contextCancelledValue(c.done, c.err); cancelled {
			if reason.IsNil() {
				reason = StringValue("cancelled")
			}
			return llmCancelError(reason.String())
		}
	}
	return NilValue()
}

func llmCancelError(reason string) Value {
	shape := stdlibai.CancelledError(reason)
	t := NewTable()
	t.RawSetString("kind", StringValue(shape.Kind))
	t.RawSetString("message", StringValue(shape.Message))
	return TableValue(t)
}

func llmErrorKind(v Value) string {
	if !v.IsTable() {
		return ""
	}
	return v.Table().RawGetString("kind").Str()
}

func llmRecoverableToolError(kind string) bool {
	switch kind {
	case "validation", "policy", "user", "capability":
		return true
	default:
		return false
	}
}

func llmTransientToolError(kind string) bool {
	switch kind {
	case "network", "provider", "internal":
		return true
	default:
		return false
	}
}

func llmReactResultValue(status, text, reason string, turn Value, history []Value) Value {
	shape := stdlibai.ReactResult(status, text, reason)
	t := NewTable()
	t.RawSetString("status", StringValue(shape.Status))
	t.RawSetString("text", StringValue(shape.Text))
	t.RawSetString("reason", StringValue(shape.Reason))
	t.RawSetString("result", turn)
	t.RawSetString("history", llmTableFromValues(history))
	return TableValue(t)
}

func llmStructuredOutputValue(opts *Table, text string) (Value, Value) {
	if opts == nil || !opts.RawGetString("output").IsTable() {
		return NilValue(), NilValue()
	}
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var data any
	if err := dec.Decode(&data); err != nil {
		return NilValue(), llmErrorValue("validation", "structured output is not valid JSON: "+err.Error())
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return NilValue(), llmErrorValue("validation", "structured output contains trailing JSON")
		}
		return NilValue(), llmErrorValue("validation", "structured output is not valid JSON: "+err.Error())
	}
	value := jsonGoToGScript(data)
	if !value.IsTable() {
		return NilValue(), llmErrorValue("validation", "structured output JSON must decode to a table")
	}
	if message := llmValidateStructuredOutputShape(opts.RawGetString("output").Table(), value.Table()); message != "" {
		return NilValue(), llmErrorValue("validation", message)
	}
	return value, NilValue()
}

func llmValidateStructuredOutputShape(expected, actual *Table) string {
	return llmValidateStructuredOutputValue(TableValue(expected), TableValue(actual), "")
}

func llmValidateStructuredOutputValue(expected Value, actual Value, path string) string {
	expectedType := llmStructuredOutputExampleType(expected)
	if expectedType != "" && !llmStructuredOutputTypeMatches(expectedType, actual) {
		return fmt.Sprintf("structured output field %q has type %s, want %s", path, llmStructuredOutputActualType(actual), expectedType)
	}
	if !expected.IsTable() || !actual.IsTable() {
		return ""
	}
	expectedTable := expected.Table()
	actualTable := actual.Table()
	if expectedTable.Length() > 0 {
		return llmValidateStructuredOutputArray(expectedTable, actualTable, path)
	}
	return llmValidateStructuredOutputObject(expectedTable, actualTable, path)
}

func llmValidateStructuredOutputObject(expected, actual *Table, path string) string {
	if expected == nil || actual == nil {
		return ""
	}
	for _, key := range expected.PairsKeysSnapshot() {
		if !key.IsString() {
			continue
		}
		name := key.Str()
		expectedValue := expected.RawGetString(name)
		actualValue := actual.RawGetString(name)
		if actualValue.IsNil() {
			return fmt.Sprintf("structured output missing field %q", llmStructuredOutputPath(path, name))
		}
		if message := llmValidateStructuredOutputValue(expectedValue, actualValue, llmStructuredOutputPath(path, name)); message != "" {
			return message
		}
	}
	return ""
}

func llmValidateStructuredOutputArray(expected, actual *Table, path string) string {
	if expected == nil || actual == nil {
		return ""
	}
	if actual.Length() == 0 {
		return fmt.Sprintf("structured output field %q must contain at least one item", path)
	}
	exemplar := expected.RawGet(IntValue(1))
	if exemplar.IsNil() {
		return ""
	}
	for i := 1; i <= actual.Length(); i++ {
		item := actual.RawGet(IntValue(int64(i)))
		if item.IsNil() {
			return fmt.Sprintf("structured output missing field %q", llmStructuredOutputIndexPath(path, i))
		}
		if message := llmValidateStructuredOutputValue(exemplar, item, llmStructuredOutputIndexPath(path, i)); message != "" {
			return message
		}
	}
	return ""
}

func llmStructuredOutputPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func llmStructuredOutputIndexPath(prefix string, index int) string {
	if prefix == "" {
		return fmt.Sprintf("[%d]", index)
	}
	return fmt.Sprintf("%s[%d]", prefix, index)
}

func llmStructuredOutputExampleType(v Value) string {
	switch {
	case v.IsString():
		return "string"
	case v.IsNumber():
		return "number"
	case v.IsBool():
		return "bool"
	case v.IsTable():
		return "table"
	default:
		return ""
	}
}

func llmStructuredOutputTypeMatches(expected string, v Value) bool {
	switch expected {
	case "string":
		return v.IsString()
	case "number":
		return v.IsNumber()
	case "bool":
		return v.IsBool()
	case "table":
		return v.IsTable()
	default:
		return true
	}
}

func llmStructuredOutputActualType(v Value) string {
	if v.IsNumber() {
		return "number"
	}
	if v.IsBool() {
		return "bool"
	}
	if v.IsString() {
		return "string"
	}
	if v.IsTable() {
		return "table"
	}
	return v.TypeName()
}
