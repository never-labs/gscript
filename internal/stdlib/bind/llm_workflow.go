package bind

import "fmt"

func registerLLMWorkflowHelpers(t *Table, call ScriptFunctionCaller) {
	setLLMFunction(t, "llm", "step", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsFunction() {
			return nil, fmt.Errorf("bad argument to 'llm.step' (name, function expected)")
		}
		step := NewTable()
		step.RawSetString("__llm_workflow_step", BoolValue(true))
		step.RawSetString("name", args[0])
		step.RawSetString("fn", args[1])
		if len(args) >= 3 && args[2].IsTable() {
			step.RawSetString("opts", args[2])
		}
		return []Value{TableValue(step)}, nil
	})

	setLLMFunction(t, "llm", "workflow", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.workflow' (steps table expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("llm.workflow requires a function caller")
		}
		steps, err := llmWorkflowStepsFromValue(args[0])
		if err != nil {
			return nil, err
		}
		fixtures := NewTable()
		if len(args) >= 2 && args[1].IsTable() {
			fixtures = args[1].Table()
		}
		return []Value{TableValue(newLLMWorkflowTable(call, steps, fixtures))}, nil
	})
}

type llmWorkflowStep struct {
	Name string
	Fn   Value
	Opts Value
}

func llmWorkflowStepsFromValue(v Value) ([]llmWorkflowStep, error) {
	src := v.Table()
	if src == nil {
		return nil, fmt.Errorf("bad argument #1 to 'llm.workflow' (steps table expected)")
	}
	if stepsValue := src.RawGetString("steps"); stepsValue.IsTable() {
		src = stepsValue.Table()
	}
	steps := make([]llmWorkflowStep, 0, src.Length())
	for i := 1; i <= src.Length(); i++ {
		item := src.RawGet(IntValue(int64(i)))
		step, err := llmWorkflowStepFromValue(item, i)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("llm.workflow requires at least one step")
	}
	return steps, nil
}

func llmWorkflowStepFromValue(v Value, index int) (llmWorkflowStep, error) {
	if v.IsFunction() {
		return llmWorkflowStep{Name: fmt.Sprintf("step_%d", index), Fn: v}, nil
	}
	if !v.IsTable() {
		return llmWorkflowStep{}, fmt.Errorf("bad workflow step #%d (step table or function expected)", index)
	}
	t := v.Table()
	fn := t.RawGetString("fn")
	if !fn.IsFunction() {
		return llmWorkflowStep{}, fmt.Errorf("bad workflow step #%d (fn function expected)", index)
	}
	name := t.RawGetString("name").Str()
	if name == "" {
		name = fmt.Sprintf("step_%d", index)
	}
	return llmWorkflowStep{Name: name, Fn: fn, Opts: t.RawGetString("opts")}, nil
}

func newLLMWorkflowTable(call ScriptFunctionCaller, steps []llmWorkflowStep, fixtures *Table) *Table {
	workflow := NewTable()
	workflow.RawSetString("__llm_workflow", BoolValue(true))
	workflow.RawSetString("steps", llmWorkflowStepListValue(steps))
	workflow.RawSetString("run", FunctionValue(&GoFunction{Name: "llm.workflow.run", Fn: func(args []Value) ([]Value, error) {
		input := NilValue()
		if len(args) >= 1 {
			input = args[0]
		}
		runFixtures := fixtures
		if len(args) >= 2 && args[1].IsTable() {
			if mock := args[1].Table().RawGetString("mock"); mock.IsTable() {
				runFixtures = mock.Table()
			}
		}
		result, errValue, err := llmRunWorkflow(call, steps, input, runFixtures)
		if err != nil {
			return nil, err
		}
		return []Value{result, errValue}, nil
	}}))
	workflow.RawSetString("mock", FunctionValue(&GoFunction{Name: "llm.workflow.mock", Fn: func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.workflow.mock' (fixtures table expected)")
		}
		return []Value{TableValue(newLLMWorkflowTable(call, steps, args[0].Table()))}, nil
	}}))
	return workflow
}

func llmWorkflowStepListValue(steps []llmWorkflowStep) Value {
	out := NewSequentialArrayTable(len(steps))
	for i, step := range steps {
		item := NewTable()
		item.RawSetString("name", StringValue(step.Name))
		if !step.Opts.IsNil() {
			item.RawSetString("opts", step.Opts)
		}
		out.RawSet(IntValue(int64(i+1)), TableValue(item))
	}
	return TableValue(out)
}

func llmRunWorkflow(call ScriptFunctionCaller, steps []llmWorkflowStep, initialInput Value, fixtures *Table) (Value, Value, error) {
	records := NewSequentialArrayTable(0)
	context := NewTable()
	currentInput := initialInput
	var previous Value = NilValue()
	var lastRecord Value = NilValue()
	var workflowErr Value = NilValue()

	for i, step := range steps {
		record, err := llmRunWorkflowStep(call, step, i+1, initialInput, currentInput, previous, TableValue(records), TableValue(context), fixtures)
		if err != nil {
			return NilValue(), NilValue(), err
		}
		records.RawSet(IntValue(int64(i+1)), record)
		context.RawSetString(step.Name, record)
		lastRecord = record
		previous = record
		if rt := record.Table(); rt != nil {
			if errValue := rt.RawGetString("err"); !errValue.IsNil() {
				workflowErr = errValue
				break
			}
			currentInput = llmWorkflowNextInput(rt)
		}
	}

	result := NewTable()
	result.RawSetString("status", StringValue("ok"))
	if !workflowErr.IsNil() {
		result.RawSetString("status", StringValue("error"))
		result.RawSetString("err", workflowErr)
	}
	result.RawSetString("input", initialInput)
	result.RawSetString("value", NilValue())
	result.RawSetString("text", StringValue(""))
	if lt := lastRecord.Table(); lt != nil {
		result.RawSetString("value", lt.RawGetString("value"))
		result.RawSetString("text", lt.RawGetString("text"))
		result.RawSetString("result", lt.RawGetString("result"))
	}
	result.RawSetString("steps", TableValue(records))
	result.RawSetString("context", TableValue(context))
	return TableValue(result), workflowErr, nil
}

func llmRunWorkflowStep(call ScriptFunctionCaller, step llmWorkflowStep, index int, initialInput, input, previous, results, context Value, fixtures *Table) (Value, error) {
	if fixture := llmWorkflowFixture(fixtures, step.Name, index); !fixture.IsNil() {
		value, errValue := llmWorkflowNormalizeFixture(fixture)
		return llmWorkflowRecord(step, index, input, value, errValue, true), nil
	}
	ctx := NewTable()
	ctx.RawSetString("name", StringValue(step.Name))
	ctx.RawSetString("index", IntValue(int64(index)))
	ctx.RawSetString("input", input)
	ctx.RawSetString("initial_input", initialInput)
	ctx.RawSetString("previous", previous)
	ctx.RawSetString("results", results)
	ctx.RawSetString("context", context)
	if !step.Opts.IsNil() {
		ctx.RawSetString("opts", step.Opts)
	}
	values, err := call(step.Fn, []Value{TableValue(ctx)})
	if err != nil {
		return NilValue(), err
	}
	var value Value = NilValue()
	var errValue Value = NilValue()
	if len(values) >= 1 {
		value = values[0]
	}
	if len(values) >= 2 {
		errValue = values[1]
	}
	return llmWorkflowRecord(step, index, input, value, errValue, false), nil
}

func llmWorkflowRecord(step llmWorkflowStep, index int, input, value, errValue Value, mocked bool) Value {
	record := NewTable()
	record.RawSetString("name", StringValue(step.Name))
	record.RawSetString("index", IntValue(int64(index)))
	record.RawSetString("input", input)
	record.RawSetString("result", value)
	record.RawSetString("value", llmWorkflowOutputValue(value))
	record.RawSetString("text", StringValue(llmWorkflowOutputText(value)))
	record.RawSetString("err", errValue)
	record.RawSetString("mocked", BoolValue(mocked))
	return TableValue(record)
}

func llmWorkflowFixture(fixtures *Table, name string, index int) Value {
	if fixtures == nil {
		return NilValue()
	}
	if byName := fixtures.RawGetString(name); !byName.IsNil() {
		return byName
	}
	return fixtures.RawGet(IntValue(int64(index)))
}

func llmWorkflowNormalizeFixture(v Value) (Value, Value) {
	if !v.IsTable() {
		return v, NilValue()
	}
	t := v.Table()
	errValue := t.RawGetString("err")
	if !t.RawGetString("result").IsNil() {
		return t.RawGetString("result"), errValue
	}
	if !t.RawGetString("value").IsNil() || !t.RawGetString("text").IsNil() {
		out := NewTable()
		if value := t.RawGetString("value"); !value.IsNil() {
			out.RawSetString("value", value)
		}
		if text := t.RawGetString("text"); !text.IsNil() {
			out.RawSetString("text", text)
		}
		return TableValue(out), errValue
	}
	return v, errValue
}

func llmWorkflowOutputValue(v Value) Value {
	if v.IsTable() {
		t := v.Table()
		if value := t.RawGetString("value"); !value.IsNil() {
			return value
		}
	}
	return v
}

func llmWorkflowOutputText(v Value) string {
	if v.IsTable() {
		t := v.Table()
		if text := t.RawGetString("text"); text.IsString() {
			return text.Str()
		}
		if value := t.RawGetString("value"); !value.IsNil() {
			return value.Str()
		}
	}
	return v.Str()
}

func llmWorkflowNextInput(record *Table) Value {
	if text := record.RawGetString("text"); text.IsString() && text.Str() != "" {
		return text
	}
	if value := record.RawGetString("value"); !value.IsNil() {
		return value
	}
	return record.RawGetString("result")
}
