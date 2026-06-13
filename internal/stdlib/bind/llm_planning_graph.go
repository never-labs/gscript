package bind

import "fmt"

func (b *llmLibBuilder) registerPlanningGraphHelpers() {
	planNode := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.plan_node' (node id string expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.plan_node' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmPlanNodeValue(args[0].Str(), opts))}, nil
	}
	b.set("plan_node", planNode)
	b.set("planNode", planNode)

	planningGraph := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.planning_graph' (nodes table expected)")
		}
		edges := NewSequentialArrayTable(0)
		opts := NewTable()
		if len(args) >= 2 {
			if args[1].IsTable() {
				edges = args[1].Table()
			} else {
				return nil, fmt.Errorf("bad argument #2 to 'llm.planning_graph' (edges table expected)")
			}
		}
		if len(args) >= 3 {
			if !args[2].IsTable() {
				return nil, fmt.Errorf("bad argument #3 to 'llm.planning_graph' (options table expected)")
			}
			opts = args[2].Table()
		}
		return []Value{TableValue(llmPlanningGraphValue(args[0].Table(), edges, opts))}, nil
	}
	b.set("planning_graph", planningGraph)
	b.set("planningGraph", planningGraph)

	validateGraph := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.validate_planning_graph' (planning graph table expected)")
		}
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.validate_planning_graph' (options table expected)")
			}
			opts = args[1].Table()
		}
		return []Value{TableValue(llmValidatePlanningGraphValue(args[0].Table(), opts))}, nil
	}
	b.set("validate_planning_graph", validateGraph)
	b.set("validatePlanningGraph", validateGraph)

	planningTrace := func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'llm.planning_trace' (planning graph table expected)")
		}
		events := NewSequentialArrayTable(0)
		opts := NewTable()
		if len(args) >= 2 {
			if !args[1].IsTable() {
				return nil, fmt.Errorf("bad argument #2 to 'llm.planning_trace' (events table expected)")
			}
			events = args[1].Table()
		}
		if len(args) >= 3 {
			if !args[2].IsTable() {
				return nil, fmt.Errorf("bad argument #3 to 'llm.planning_trace' (options table expected)")
			}
			opts = args[2].Table()
		}
		return []Value{TableValue(llmPlanningTraceValue(args[0].Table(), events, opts))}, nil
	}
	b.set("planning_trace", planningTrace)
	b.set("planningTrace", planningTrace)
}

func llmPlanNodeValue(id string, opts *Table) *Table {
	node := NewTable()
	node.RawSetString("kind", StringValue("plan_node"))
	node.RawSetString("schema_version", IntValue(1))
	node.RawSetString("version", StringValue("plan_node.v1"))
	node.RawSetString("id", StringValue(id))
	node.RawSetString("node_type", StringValue(llmReplayOptionString(opts, "node_type", "transform")))
	node.RawSetString("depends_on", llmPlanningStringTable(opts.RawGetString("depends_on")))
	node.RawSetString("branches", llmPlanningStringTable(opts.RawGetString("branches")))
	node.RawSetString("trace_evidence", llmPlanningStringTable(opts.RawGetString("trace_evidence")))
	node.RawSetString("retry_policy", TableValue(llmPlanningRetryPolicy(opts.RawGetString("retry_policy"))))
	node.RawSetString("merge_policy", TableValue(llmPlanningMergePolicy(opts.RawGetString("merge_policy"))))
	for _, field := range []string{"capability", "input_ref", "output_ref", "fixture_key", "description"} {
		if value := opts.RawGetString(field); !value.IsNil() {
			node.RawSetString(field, llmCloneValue(value))
		}
	}
	return node
}

func llmPlanningGraphValue(nodes, edges, opts *Table) *Table {
	graph := NewTable()
	graph.RawSetString("__llm_planning_graph", BoolValue(true))
	graph.RawSetString("kind", StringValue("planning_graph"))
	graph.RawSetString("schema_version", IntValue(int64(llmReplayOptionInt(opts, "schema_version", 1))))
	graph.RawSetString("version", StringValue(llmReplayOptionString(opts, "version", "planning_graph.v1")))
	graph.RawSetString("id", StringValue(llmReplayOptionString(opts, "id", llmReplayOptionString(opts, "graph_id", "planning-graph"))))
	graph.RawSetString("provider_free", BoolValue(llmReplayOptionBool(opts, "provider_free", true)))
	graph.RawSetString("domain_specific", BoolValue(llmReplayOptionBool(opts, "domain_specific", false)))
	graph.RawSetString("live_network", BoolValue(llmReplayOptionBool(opts, "live_network", false)))
	graph.RawSetString("live_model", BoolValue(llmReplayOptionBool(opts, "live_model", false)))
	graph.RawSetString("real_dependency_imports", BoolValue(llmReplayOptionBool(opts, "real_dependency_imports", false)))
	graph.RawSetString("capability", StringValue(llmReplayOptionString(opts, "capability", "generic.planning.graph.define")))
	graph.RawSetString("capability_refs", llmPlanningStringTable(opts.RawGetString("capability_refs")))
	normalizedNodes := llmPlanningNormalizeNodes(nodes)
	normalizedEdges := llmPlanningNormalizeEdges(edges, normalizedNodes)
	graph.RawSetString("plan_nodes", TableValue(normalizedNodes))
	graph.RawSetString("nodes", TableValue(normalizedNodes))
	graph.RawSetString("edges", TableValue(normalizedEdges))
	graph.RawSetString("node_count", IntValue(int64(normalizedNodes.Length())))
	graph.RawSetString("edge_count", IntValue(int64(normalizedEdges.Length())))
	graph.RawSetString("summary", TableValue(llmPlanningGraphSummary(normalizedNodes, normalizedEdges)))
	graph.RawSetString("redaction", TableValue(llmPlanningGraphRedaction(opts)))
	return graph
}

func llmValidatePlanningGraphValue(graph, opts *Table) *Table {
	out := NewTable()
	out.RawSetString("kind", StringValue("planning_graph_validation"))
	out.RawSetString("schema_version", IntValue(1))
	out.RawSetString("version", StringValue("planning_graph_validation.v1"))
	out.RawSetString("provider_free", BoolValue(true))
	findings := NewSequentialArrayTable(0)
	addFinding := func(kind, message, field string) {
		f := NewTable()
		f.RawSetString("kind", StringValue(kind))
		f.RawSetString("message", StringValue(message))
		if field != "" {
			f.RawSetString("field", StringValue(field))
		}
		findings.RawSet(IntValue(int64(findings.Length()+1)), TableValue(f))
	}
	if !llmReplayOptionBool(graph, "provider_free", false) {
		addFinding("provider_free", "planning graph must be provider-free", "provider_free")
	}
	if llmReplayOptionBool(graph, "live_network", false) {
		addFinding("live_network", "planning graph must not require live network", "live_network")
	}
	if llmReplayOptionBool(graph, "live_model", false) {
		addFinding("live_model", "planning graph must not require a live model", "live_model")
	}
	if llmReplayOptionBool(graph, "real_dependency_imports", false) {
		addFinding("real_dependency_imports", "planning graph must not require real dependency imports", "real_dependency_imports")
	}
	nodes := graph.RawGetString("plan_nodes")
	if !nodes.IsTable() {
		nodes = graph.RawGetString("nodes")
	}
	edges := graph.RawGetString("edges")
	llmValidatePlanningGraphShape(nodes, edges, opts, addFinding)
	out.RawSetString("findings", TableValue(findings))
	out.RawSetString("finding_count", IntValue(int64(findings.Length())))
	ok := findings.Length() == 0
	out.RawSetString("ok", BoolValue(ok))
	if ok {
		out.RawSetString("status", StringValue("ok"))
	} else {
		out.RawSetString("status", StringValue("failed"))
	}
	return out
}

func llmPlanningTraceValue(graph, events, opts *Table) *Table {
	trace := NewTable()
	trace.RawSetString("kind", StringValue("planning_trace"))
	trace.RawSetString("schema_version", IntValue(1))
	trace.RawSetString("version", StringValue("planning_trace.v1"))
	trace.RawSetString("fixture_key", StringValue(llmReplayOptionString(opts, "fixture_key", "")))
	trace.RawSetString("contract_id", StringValue(llmReplayOptionString(opts, "contract_id", llmReplayOptionString(graph, "id", ""))))
	trace.RawSetString("provider_free", BoolValue(true))
	trace.RawSetString("domain_specific", BoolValue(false))
	trace.RawSetString("live_network", BoolValue(false))
	trace.RawSetString("live_model", BoolValue(false))
	trace.RawSetString("real_dependency_imports", BoolValue(false))
	trace.RawSetString("run_id", StringValue(llmReplayOptionString(opts, "run_id", "planning-run:1")))
	normalized := llmPlanningTraceEvents(graph, events)
	trace.RawSetString("events", TableValue(normalized))
	trace.RawSetString("summary", TableValue(llmPlanningTraceSummary(graph, normalized, opts)))
	trace.RawSetString("redaction", TableValue(llmPlanningGraphRedaction(opts)))
	return trace
}

func llmPlanningNormalizeNodes(nodes *Table) *Table {
	out := NewSequentialArrayTable(0)
	for i := 1; i <= nodes.Length(); i++ {
		item := nodes.RawGet(IntValue(int64(i)))
		if !item.IsTable() {
			continue
		}
		node := item.Table()
		id := node.RawGetString("id").Str()
		if id == "" {
			id = node.RawGetString("name").Str()
		}
		if id == "" {
			id = fmt.Sprintf("node_%d", i)
		}
		normalized := llmCloneTable(node)
		normalized.RawSetString("kind", StringValue("plan_node"))
		normalized.RawSetString("id", StringValue(id))
		normalized.RawSetString("node_type", StringValue(llmReplayOptionString(normalized, "node_type", "transform")))
		if !normalized.RawGetString("depends_on").IsTable() {
			normalized.RawSetString("depends_on", TableValue(NewSequentialArrayTable(0)))
		}
		if !normalized.RawGetString("branches").IsTable() {
			normalized.RawSetString("branches", TableValue(NewSequentialArrayTable(0)))
		}
		if !normalized.RawGetString("trace_evidence").IsTable() {
			normalized.RawSetString("trace_evidence", TableValue(NewSequentialArrayTable(0)))
		}
		if !normalized.RawGetString("retry_policy").IsTable() {
			normalized.RawSetString("retry_policy", TableValue(llmPlanningRetryPolicy(NilValue())))
		}
		if !normalized.RawGetString("merge_policy").IsTable() {
			normalized.RawSetString("merge_policy", TableValue(llmPlanningMergePolicy(NilValue())))
		}
		out.RawSet(IntValue(int64(out.Length()+1)), TableValue(normalized))
	}
	return out
}

func llmPlanningNormalizeEdges(edges, nodes *Table) *Table {
	out := NewSequentialArrayTable(0)
	for i := 1; i <= edges.Length(); i++ {
		item := edges.RawGet(IntValue(int64(i)))
		if !item.IsTable() {
			continue
		}
		edge := item.Table()
		from := llmReplayOptionString(edge, "from", edge.RawGet(IntValue(1)).Str())
		to := llmReplayOptionString(edge, "to", edge.RawGet(IntValue(2)).Str())
		if from == "" || to == "" {
			continue
		}
		normalized := NewTable()
		normalized.RawSetString("from", StringValue(from))
		normalized.RawSetString("to", StringValue(to))
		normalized.RawSetString("edge_type", StringValue(llmReplayOptionString(edge, "edge_type", "control")))
		if branch := edge.RawGetString("branch_id"); !branch.IsNil() {
			normalized.RawSetString("branch_id", llmCloneValue(branch))
		}
		out.RawSet(IntValue(int64(out.Length()+1)), TableValue(normalized))
	}
	if out.Length() == 0 {
		for i := 1; i <= nodes.Length(); i++ {
			node := nodes.RawGet(IntValue(int64(i)))
			if !node.IsTable() {
				continue
			}
			id := node.Table().RawGetString("id").Str()
			deps := node.Table().RawGetString("depends_on")
			if !deps.IsTable() {
				continue
			}
			for j := 1; j <= deps.Table().Length(); j++ {
				dep := deps.Table().RawGet(IntValue(int64(j))).Str()
				if dep == "" || id == "" {
					continue
				}
				edge := NewTable()
				edge.RawSetString("from", StringValue(dep))
				edge.RawSetString("to", StringValue(id))
				edge.RawSetString("edge_type", StringValue("control"))
				out.RawSet(IntValue(int64(out.Length()+1)), TableValue(edge))
			}
		}
	}
	return out
}

func llmValidatePlanningGraphShape(nodesValue, edgesValue Value, opts *Table, addFinding func(kind, message, field string)) {
	if !nodesValue.IsTable() || nodesValue.Table().Length() == 0 {
		addFinding("plan_nodes", "planning graph must contain plan nodes", "plan_nodes")
		return
	}
	nodes := nodesValue.Table()
	seen := map[string]int{}
	for i := 1; i <= nodes.Length(); i++ {
		item := nodes.RawGet(IntValue(int64(i)))
		if !item.IsTable() {
			addFinding("plan_node", "plan node must be a table", "plan_nodes")
			continue
		}
		node := item.Table()
		id := node.RawGetString("id").Str()
		if id == "" {
			addFinding("id", "plan node must have an id", "id")
			continue
		}
		if _, ok := seen[id]; ok {
			addFinding("duplicate_node", "plan node ids must be unique", "id")
		}
		seen[id] = i
		if node.RawGetString("node_type").Str() == "" {
			addFinding("node_type", "plan node must have a node type", "node_type")
		}
		if !node.RawGetString("retry_policy").IsTable() {
			addFinding("retry_policy", "plan node must have a retry policy", "retry_policy")
		}
		if !node.RawGetString("trace_evidence").IsTable() {
			addFinding("trace_evidence", "plan node must have trace evidence refs", "trace_evidence")
		}
	}
	if edgesValue.IsTable() {
		llmValidatePlanningEdges(edgesValue.Table(), seen, addFinding)
	}
}

func llmValidatePlanningEdges(edges *Table, seen map[string]int, addFinding func(kind, message, field string)) {
	for i := 1; i <= edges.Length(); i++ {
		item := edges.RawGet(IntValue(int64(i)))
		if !item.IsTable() {
			addFinding("edge", "planning graph edge must be a table", "edges")
			continue
		}
		edge := item.Table()
		from := edge.RawGetString("from").Str()
		to := edge.RawGetString("to").Str()
		if from == "" || to == "" {
			addFinding("edge_endpoint", "planning graph edge must have from and to", "edges")
			continue
		}
		fromIndex, fromOK := seen[from]
		toIndex, toOK := seen[to]
		if !fromOK || !toOK {
			addFinding("unknown_node", "planning graph edge references an unknown node", "edges")
			continue
		}
		if fromIndex >= toIndex {
			addFinding("acyclic", "planning graph edges must be topologically ordered and acyclic", "edges")
		}
		if edge.RawGetString("edge_type").Str() == "" {
			addFinding("edge_type", "planning graph edge must have an edge type", "edge_type")
		}
	}
}

func llmPlanningTraceEvents(graph, events *Table) *Table {
	out := NewSequentialArrayTable(0)
	if events != nil && events.Length() > 0 {
		for i := 1; i <= events.Length(); i++ {
			item := events.RawGet(IntValue(int64(i)))
			if !item.IsTable() {
				continue
			}
			event := llmCloneTable(item.Table())
			llmPlanningTraceEventDefaults(event, i)
			out.RawSet(IntValue(int64(out.Length()+1)), TableValue(event))
		}
		return out
	}
	nodes := graph.RawGetString("plan_nodes")
	if !nodes.IsTable() {
		nodes = graph.RawGetString("nodes")
	}
	if !nodes.IsTable() {
		return out
	}
	for i := 1; i <= nodes.Table().Length(); i++ {
		nodeValue := nodes.Table().RawGet(IntValue(int64(i)))
		if !nodeValue.IsTable() {
			continue
		}
		node := nodeValue.Table()
		event := NewTable()
		event.RawSetString("trace_id", StringValue("planning-trace"))
		event.RawSetString("event_id", StringValue(node.RawGetString("id").Str()+":completed"))
		event.RawSetString("evidence_kind", StringValue("plan.node.completed"))
		event.RawSetString("plan_node_id", llmCloneValue(node.RawGetString("id")))
		event.RawSetString("sequence", IntValue(int64(out.Length()+1)))
		event.RawSetString("status", StringValue("completed"))
		event.RawSetString("provider_free", BoolValue(true))
		event.RawSetString("live_model", BoolValue(false))
		event.RawSetString("attempt", IntValue(1))
		out.RawSet(IntValue(int64(out.Length()+1)), TableValue(event))
	}
	return out
}

func llmPlanningTraceEventDefaults(event *Table, index int) {
	if event.RawGetString("trace_id").IsNil() {
		event.RawSetString("trace_id", StringValue("planning-trace"))
	}
	if event.RawGetString("event_id").IsNil() {
		event.RawSetString("event_id", StringValue(fmt.Sprintf("event-%d", index)))
	}
	if event.RawGetString("evidence_kind").IsNil() {
		event.RawSetString("evidence_kind", StringValue("plan.node.completed"))
	}
	if event.RawGetString("sequence").IsNil() {
		event.RawSetString("sequence", IntValue(int64(index)))
	}
	if event.RawGetString("status").IsNil() {
		event.RawSetString("status", StringValue("completed"))
	}
	event.RawSetString("provider_free", BoolValue(true))
	event.RawSetString("live_model", BoolValue(false))
	if event.RawGetString("attempt").IsNil() {
		event.RawSetString("attempt", IntValue(1))
	}
}

func llmPlanningTraceSummary(graph, events, opts *Table) *Table {
	summary := NewTable()
	summary.RawSetString("plan_node_count", llmCloneValue(graph.RawGetString("node_count")))
	summary.RawSetString("edge_count", llmCloneValue(graph.RawGetString("edge_count")))
	summary.RawSetString("retry_attempt_count", IntValue(int64(llmPlanningTraceCountKind(events, "plan.retry.attempt"))))
	summary.RawSetString("branch_count", IntValue(int64(llmPlanningTraceCountKind(events, "plan.branch.selected"))))
	summary.RawSetString("merge_count", IntValue(int64(llmPlanningTraceCountKind(events, "plan.merge.completed"))))
	summary.RawSetString("final_status", StringValue(llmReplayOptionString(opts, "final_status", "completed")))
	return summary
}

func llmPlanningTraceCountKind(events *Table, kind string) int {
	count := 0
	for i := 1; i <= events.Length(); i++ {
		item := events.RawGet(IntValue(int64(i)))
		if item.IsTable() && item.Table().RawGetString("evidence_kind").Str() == kind {
			count++
		}
	}
	return count
}

func llmPlanningGraphSummary(nodes, edges *Table) *Table {
	summary := NewTable()
	summary.RawSetString("plan_node_count", IntValue(int64(nodes.Length())))
	summary.RawSetString("edge_count", IntValue(int64(edges.Length())))
	summary.RawSetString("branch_count", IntValue(int64(llmPlanningCountNodes(nodes, "node_type", "branch"))))
	summary.RawSetString("merge_count", IntValue(int64(llmPlanningCountNodes(nodes, "node_type", "merge"))))
	summary.RawSetString("retryable_count", IntValue(int64(llmPlanningRetryableCount(nodes))))
	return summary
}

func llmPlanningCountNodes(nodes *Table, field, want string) int {
	count := 0
	for i := 1; i <= nodes.Length(); i++ {
		item := nodes.RawGet(IntValue(int64(i)))
		if item.IsTable() && item.Table().RawGetString(field).Str() == want {
			count++
		}
	}
	return count
}

func llmPlanningRetryableCount(nodes *Table) int {
	count := 0
	for i := 1; i <= nodes.Length(); i++ {
		item := nodes.RawGet(IntValue(int64(i)))
		if !item.IsTable() {
			continue
		}
		retry := item.Table().RawGetString("retry_policy")
		if retry.IsTable() && retry.Table().RawGetString("retryable").Truthy() {
			count++
		}
	}
	return count
}

func llmPlanningRetryPolicy(value Value) *Table {
	if value.IsTable() {
		return llmCloneTable(value.Table())
	}
	retry := NewTable()
	retry.RawSetString("max_attempts", IntValue(1))
	retry.RawSetString("retryable", BoolValue(false))
	retry.RawSetString("backoff", StringValue("none"))
	return retry
}

func llmPlanningMergePolicy(value Value) *Table {
	if value.IsTable() {
		return llmCloneTable(value.Table())
	}
	merge := NewTable()
	merge.RawSetString("mode", StringValue("none"))
	merge.RawSetString("required_inputs", TableValue(NewSequentialArrayTable(0)))
	return merge
}

func llmPlanningStringTable(value Value) Value {
	if value.IsTable() {
		return llmCloneValue(value)
	}
	if value.IsString() {
		out := NewSequentialArrayTable(1)
		out.RawSet(IntValue(1), value)
		return TableValue(out)
	}
	return TableValue(NewSequentialArrayTable(0))
}

func llmPlanningGraphRedaction(opts *Table) *Table {
	redaction := NewTable()
	redaction.RawSetString("enabled", BoolValue(true))
	redaction.RawSetString("policy", StringValue(llmReplayOptionString(opts, "redaction_policy", "planning_graph_metadata_only")))
	redaction.RawSetString("raw_inputs_stored", BoolValue(false))
	redaction.RawSetString("raw_outputs_stored", BoolValue(false))
	redaction.RawSetString("raw_prompt_stored", BoolValue(false))
	redaction.RawSetString("raw_completion_stored", BoolValue(false))
	redaction.RawSetString("secret_values_present", BoolValue(false))
	return redaction
}
