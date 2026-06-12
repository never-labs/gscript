package bind

import "fmt"

func registerLLMWorkflowHelpers(t *Table, call ScriptFunctionCaller) {
	setLLMFunction(t, "llm", "step", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsFunction() {
			return nil, fmt.Errorf("bad argument to 'llm.step' (name, function expected)")
		}
		return []Value{TableValue(llmWorkflowStepTable(args[0], args[1], llmWorkflowOptionalOpts(args), false))}, nil
	})

	setLLMFunction(t, "llm", "stage", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsFunction() {
			return nil, fmt.Errorf("bad argument to 'llm.stage' (name, function expected)")
		}
		return []Value{TableValue(llmWorkflowStepTable(args[0], args[1], llmWorkflowOptionalOpts(args), true))}, nil
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

	setLLMFunction(t, "llm", "workflow_graph", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.workflow_graph' (config table expected)")
		}
		if call == nil {
			return nil, fmt.Errorf("llm.workflow_graph requires a function caller")
		}
		graph, err := llmWorkflowGraphFromConfig(args[0].Table())
		if err != nil {
			return nil, err
		}
		fixtures := graph.fixtures
		if len(args) >= 2 && args[1].IsTable() {
			fixtures = args[1].Table()
		}
		return []Value{TableValue(newLLMWorkflowGraphTable(call, graph, fixtures))}, nil
	})
}

type llmWorkflowStep struct {
	Name  string
	Fn    Value
	Opts  Value
	Stage bool
}

type llmWorkflowGraph struct {
	WorkflowID string
	Entrypoint string
	Steps      []llmWorkflowStep
	Edges      [][2]string
	Meta       Value
	fixtures   *Table
}

func llmWorkflowStepTable(name, fn, opts Value, stage bool) *Table {
	step := NewTable()
	step.RawSetString("__llm_workflow_step", BoolValue(true))
	if stage {
		step.RawSetString("__llm_workflow_stage", BoolValue(true))
	}
	step.RawSetString("name", name)
	step.RawSetString("fn", fn)
	if opts.IsTable() {
		step.RawSetString("opts", opts)
	}
	return step
}

func llmWorkflowOptionalOpts(args []Value) Value {
	if len(args) >= 3 && args[2].IsTable() {
		return args[2]
	}
	return NilValue()
}

func llmWorkflowStepsFromValue(v Value) ([]llmWorkflowStep, error) {
	src := v.Table()
	if src == nil {
		return nil, fmt.Errorf("bad argument #1 to 'llm.workflow' (steps table expected)")
	}
	if stepsValue := src.RawGetString("steps"); stepsValue.IsTable() {
		src = stepsValue.Table()
	}
	if src.RawGetString("fn").IsFunction() || src.RawGetString("__llm_workflow_step").Truthy() {
		step, err := llmWorkflowStepFromValue(TableValue(src), 1)
		if err != nil {
			return nil, err
		}
		return []llmWorkflowStep{step}, nil
	}
	steps := make([]llmWorkflowStep, 0, src.Length())
	for i := 1; i <= src.Length(); i++ {
		item := src.RawGet(IntValue(int64(i)))
		if item.IsNil() {
			continue
		}
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
	return llmWorkflowStep{Name: name, Fn: fn, Opts: t.RawGetString("opts"), Stage: t.RawGetString("__llm_workflow_stage").Truthy()}, nil
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
		if step.Stage {
			item.RawSetString("stage", BoolValue(true))
		}
		if !step.Opts.IsNil() {
			item.RawSetString("opts", step.Opts)
		}
		out.RawSet(IntValue(int64(i+1)), TableValue(item))
	}
	return TableValue(out)
}

func newLLMWorkflowGraphTable(call ScriptFunctionCaller, graph llmWorkflowGraph, fixtures *Table) *Table {
	workflow := newLLMWorkflowTable(call, graph.Steps, fixtures)
	workflow.RawSetString("__llm_workflow_graph", BoolValue(true))
	workflow.RawSetString("workflow_id", StringValue(graph.WorkflowID))
	workflow.RawSetString("entrypoint", StringValue(graph.Entrypoint))
	workflow.RawSetString("graph", graph.Meta)
	workflow.RawSetString("run", FunctionValue(&GoFunction{Name: "llm.workflow_graph.run", Fn: func(args []Value) ([]Value, error) {
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
		result, errValue, err := llmRunWorkflow(call, graph.Steps, input, runFixtures)
		if err != nil {
			return nil, err
		}
		if rt := result.Table(); rt != nil {
			rt.RawSetString("workflow_id", StringValue(graph.WorkflowID))
			rt.RawSetString("entrypoint", StringValue(graph.Entrypoint))
			rt.RawSetString("graph", graph.Meta)
		}
		return []Value{result, errValue}, nil
	}}))
	workflow.RawSetString("mock", FunctionValue(&GoFunction{Name: "llm.workflow_graph.mock", Fn: func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.workflow_graph.mock' (fixtures table expected)")
		}
		return []Value{TableValue(newLLMWorkflowGraphTable(call, graph, args[0].Table()))}, nil
	}}))
	return workflow
}

func llmWorkflowGraphFromConfig(config *Table) (llmWorkflowGraph, error) {
	if config == nil {
		return llmWorkflowGraph{}, fmt.Errorf("bad argument #1 to 'llm.workflow_graph' (config table expected)")
	}
	stageValue := config.RawGetString("stages")
	if !stageValue.IsTable() {
		stageValue = config.RawGetString("steps")
	}
	if !stageValue.IsTable() {
		return llmWorkflowGraph{}, fmt.Errorf("llm.workflow_graph requires a stages table")
	}
	steps, err := llmWorkflowStepsFromValue(stageValue)
	if err != nil {
		return llmWorkflowGraph{}, err
	}
	edges, err := llmWorkflowGraphEdges(config.RawGetString("edges"))
	if err != nil {
		return llmWorkflowGraph{}, err
	}
	workflowID := llmWorkflowConfigString(config, "workflow_id", "workflow")
	entrypoint := llmWorkflowConfigString(config, "entrypoint", "workflow.run")
	if err := llmValidateWorkflowGraph(steps, edges); err != nil {
		return llmWorkflowGraph{}, err
	}
	fixtures := NewTable()
	if value := config.RawGetString("fixtures"); value.IsTable() {
		fixtures = value.Table()
	}
	meta := llmWorkflowGraphMeta(config, workflowID, entrypoint, steps, edges)
	return llmWorkflowGraph{WorkflowID: workflowID, Entrypoint: entrypoint, Steps: steps, Edges: edges, Meta: meta, fixtures: fixtures}, nil
}

func llmWorkflowGraphMeta(config *Table, workflowID, entrypoint string, steps []llmWorkflowStep, edges [][2]string) Value {
	graph := NewTable()
	graph.RawSetString("workflow_id", StringValue(workflowID))
	graph.RawSetString("entrypoint", StringValue(entrypoint))
	graph.RawSetString("provider_free", BoolValue(llmWorkflowConfigBool(config, "provider_free", true)))
	graph.RawSetString("live_network", BoolValue(llmWorkflowConfigBool(config, "live_network", false)))
	graph.RawSetString("real_dependency_imports", BoolValue(llmWorkflowConfigBool(config, "real_dependency_imports", false)))
	graph.RawSetString("stages", llmWorkflowGraphStageListValue(steps))
	graph.RawSetString("edges", llmWorkflowGraphEdgeListValue(edges))
	return TableValue(graph)
}

func llmWorkflowGraphStageListValue(steps []llmWorkflowStep) Value {
	out := NewSequentialArrayTable(len(steps))
	for i, step := range steps {
		item := NewTable()
		item.RawSetString("id", StringValue(step.Name))
		item.RawSetString("name", StringValue(step.Name))
		item.RawSetString("stage", BoolValue(step.Stage))
		item.RawSetString("depends_on", llmWorkflowStringListValue(llmWorkflowStepDependsOn(step)))
		if step.Opts.IsTable() {
			opts := step.Opts.Table()
			for _, field := range []string{"input_ref", "output_ref", "input_schema", "output_schema", "capability", "fixture_key"} {
				if value := opts.RawGetString(field); !value.IsNil() {
					item.RawSetString(field, value)
				}
			}
		}
		out.RawSet(IntValue(int64(i+1)), TableValue(item))
	}
	return TableValue(out)
}

func llmWorkflowGraphEdgeListValue(edges [][2]string) Value {
	out := NewSequentialArrayTable(len(edges))
	for i, edge := range edges {
		item := NewTable()
		item.RawSetString("from", StringValue(edge[0]))
		item.RawSetString("to", StringValue(edge[1]))
		out.RawSet(IntValue(int64(i+1)), TableValue(item))
	}
	return TableValue(out)
}

func llmValidateWorkflowGraph(steps []llmWorkflowStep, edges [][2]string) error {
	seen := map[string]int{}
	for i, step := range steps {
		if step.Name == "" {
			return fmt.Errorf("llm.workflow_graph stage #%d has empty name", i+1)
		}
		if _, ok := seen[step.Name]; ok {
			return fmt.Errorf("llm.workflow_graph duplicate stage %q", step.Name)
		}
		for _, dep := range llmWorkflowStepDependsOn(step) {
			depIndex, ok := seen[dep]
			if !ok {
				return fmt.Errorf("llm.workflow_graph stage %q depends on unknown or later stage %q", step.Name, dep)
			}
			if depIndex >= i {
				return fmt.Errorf("llm.workflow_graph stage %q is not topologically ordered after %q", step.Name, dep)
			}
		}
		seen[step.Name] = i
	}
	for _, edge := range edges {
		fromIndex, fromOK := seen[edge[0]]
		toIndex, toOK := seen[edge[1]]
		if !fromOK || !toOK {
			return fmt.Errorf("llm.workflow_graph edge references unknown stage %q -> %q", edge[0], edge[1])
		}
		if fromIndex >= toIndex {
			return fmt.Errorf("llm.workflow_graph edge %q -> %q is not topologically ordered", edge[0], edge[1])
		}
	}
	return nil
}

func llmWorkflowStepDependsOn(step llmWorkflowStep) []string {
	if !step.Opts.IsTable() {
		return nil
	}
	return llmWorkflowStringList(step.Opts.Table().RawGetString("depends_on"))
}

func llmWorkflowGraphEdges(v Value) ([][2]string, error) {
	if v.IsNil() {
		return nil, nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("llm.workflow_graph edges must be a table")
	}
	table := v.Table()
	edges := make([][2]string, 0, table.Length())
	for i := 1; i <= table.Length(); i++ {
		item := table.RawGet(IntValue(int64(i)))
		if !item.IsTable() {
			return nil, fmt.Errorf("llm.workflow_graph edge #%d must be a table", i)
		}
		edgeTable := item.Table()
		from := llmWorkflowConfigString(edgeTable, "from", "")
		to := llmWorkflowConfigString(edgeTable, "to", "")
		if from == "" {
			from = edgeTable.RawGet(IntValue(1)).Str()
		}
		if to == "" {
			to = edgeTable.RawGet(IntValue(2)).Str()
		}
		if from == "" || to == "" {
			return nil, fmt.Errorf("llm.workflow_graph edge #%d requires from and to stages", i)
		}
		edges = append(edges, [2]string{from, to})
	}
	return edges, nil
}

func llmWorkflowStringList(v Value) []string {
	if v.IsNil() {
		return nil
	}
	if v.IsString() {
		return []string{v.Str()}
	}
	if !v.IsTable() {
		return nil
	}
	table := v.Table()
	out := make([]string, 0, table.Length())
	for i := 1; i <= table.Length(); i++ {
		if value := table.RawGet(IntValue(int64(i))); value.IsString() && value.Str() != "" {
			out = append(out, value.Str())
		}
	}
	return out
}

func llmWorkflowStringListValue(values []string) Value {
	out := NewSequentialArrayTable(len(values))
	for i, value := range values {
		out.RawSet(IntValue(int64(i+1)), StringValue(value))
	}
	return TableValue(out)
}

func llmWorkflowConfigString(t *Table, key, fallback string) string {
	if t == nil {
		return fallback
	}
	if value := t.RawGetString(key); value.IsString() && value.Str() != "" {
		return value.Str()
	}
	return fallback
}

func llmWorkflowConfigBool(t *Table, key string, fallback bool) bool {
	if t == nil {
		return fallback
	}
	value := t.RawGetString(key)
	if value.IsNil() {
		return fallback
	}
	return value.Truthy()
}

func llmRunWorkflow(call ScriptFunctionCaller, steps []llmWorkflowStep, initialInput Value, fixtures *Table) (Value, Value, error) {
	records := NewSequentialArrayTable(0)
	traceChildren := NewSequentialArrayTable(0)
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
		if rt := record.Table(); rt != nil {
			if trace := rt.RawGetString("trace"); !trace.IsNil() {
				traceChildren.RawSet(IntValue(int64(traceChildren.Length()+1)), trace)
			}
		}
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
	status := "ok"
	if !workflowErr.IsNil() {
		status = "error"
		result.RawSetString("err", workflowErr)
	}
	for i := 1; i <= traceChildren.Length(); i++ {
		trace := traceChildren.RawGet(IntValue(int64(i)))
		if trace.IsTable() {
			parent := trace.Table().RawGetString("parent")
			if parent.IsTable() {
				parent.Table().RawSetString("status", StringValue(status))
			}
		}
	}
	result.RawSetString("status", StringValue(status))
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
	result.RawSetString("trace", llmTraceContractNode("workflow", "workflow", status, NilValue(), TableValue(traceChildren), workflowErr))
	return TableValue(result), workflowErr, nil
}

func llmRunWorkflowStep(call ScriptFunctionCaller, step llmWorkflowStep, index int, initialInput, input, previous, results, context Value, fixtures *Table) (Value, error) {
	if fixture := llmWorkflowFixture(fixtures, step, index); !fixture.IsNil() {
		value, errValue := llmWorkflowNormalizeFixture(fixture)
		return llmWorkflowRecord(step, index, input, value, errValue, true), nil
	}
	ctx := NewTable()
	ctx.RawSetString("name", StringValue(step.Name))
	ctx.RawSetString("index", IntValue(int64(index)))
	ctx.RawSetString("stage", BoolValue(step.Stage))
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
	status := "ok"
	if !errValue.IsNil() {
		status = "error"
	}
	record.RawSetString("name", StringValue(step.Name))
	record.RawSetString("index", IntValue(int64(index)))
	record.RawSetString("stage", BoolValue(step.Stage))
	record.RawSetString("status", StringValue(status))
	record.RawSetString("input", input)
	record.RawSetString("result", value)
	record.RawSetString("value", llmWorkflowOutputValue(value))
	record.RawSetString("text", StringValue(llmWorkflowOutputText(value)))
	record.RawSetString("err", errValue)
	record.RawSetString("mocked", BoolValue(mocked))
	trace := llmTraceContractNode("workflow_step", step.Name, status, llmTraceContractParentRef("workflow", "workflow", ""), NilValue(), errValue)
	if tt := trace.Table(); tt != nil {
		metadata := NewTable()
		metadata.RawSetString("index", IntValue(int64(index)))
		metadata.RawSetString("mocked", BoolValue(mocked))
		metadata.RawSetString("stage", BoolValue(step.Stage))
		llmWorkflowCopyStepMetadata(metadata, step)
		tt.RawSetString("metadata", TableValue(metadata))
	}
	record.RawSetString("trace", trace)
	return TableValue(record)
}

func llmWorkflowCopyStepMetadata(metadata *Table, step llmWorkflowStep) {
	if metadata == nil || !step.Opts.IsTable() {
		return
	}
	opts := step.Opts.Table()
	for _, field := range []string{"capability", "fixture_key", "input_ref", "output_ref", "input_schema", "output_schema"} {
		if value := opts.RawGetString(field); !value.IsNil() {
			metadata.RawSetString(field, llmCloneValue(value))
		}
	}
	if value := opts.RawGetString("depends_on"); !value.IsNil() {
		metadata.RawSetString("depends_on", llmCloneValue(value))
	}
}

func llmWorkflowFixture(fixtures *Table, step llmWorkflowStep, index int) Value {
	if fixtures == nil {
		return NilValue()
	}
	if fixtureKey := llmWorkflowStepFixtureKey(step); fixtureKey != "" {
		if byFixtureKey := fixtures.RawGetString(fixtureKey); !byFixtureKey.IsNil() {
			return byFixtureKey
		}
	}
	if byName := fixtures.RawGetString(step.Name); !byName.IsNil() {
		return byName
	}
	return fixtures.RawGet(IntValue(int64(index)))
}

func llmWorkflowStepFixtureKey(step llmWorkflowStep) string {
	if !step.Opts.IsTable() {
		return ""
	}
	return llmWorkflowConfigString(step.Opts.Table(), "fixture_key", "")
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
