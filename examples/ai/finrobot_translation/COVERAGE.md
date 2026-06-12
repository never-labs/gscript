# FinRobot Translation Coverage Audit

Audit date: 2026-06-12.

Baseline:

- Leia branch: `codex/ai-dialect-polish` branch head.
- FinRobot source: local `.external/FinRobot` checkout at `6a8161f`.
- Translation directory: `examples/ai/finrobot_translation`.
- Current translation directory file count: 695 files, including 43
  provider-free live-package skeleton directories and the upstream coverage
  ledger artifact.
- Registered examples: 89 runnable/checkable examples discovered by
  `go run ./cmd/leia examples list --json`.
- Fixture index normalization: 42 live-package skeletons use
  `fixtures/provider_free_fixture_index.json`; `live_packages/vendor_adapters`
  retains its provider-specific `fixtures/offline_replay_index.json`.
- Runtime changes in this audit: none.
- Inventory-only refresh commands run for this audit:
  `rg --files examples/ai/finrobot_translation | wc -l`,
  `go run ./cmd/leia examples list --json`, live-package directory inventory,
  provider-free fixture-index inventory, and documentation semantic-guard
  searches for approval/model/workflow/trace/eval coverage. These commands are
  read-only.

## Executive Summary

The provider-free FinRobot translation slice is complete for the current
registered example surface: every registered example under
`examples/ai/finrobot_translation` is runnable and checkable without live
provider credentials. The slice covers the generic Leia contracts needed to
model FinRobot: agent turns, tool contracts, replay, trace, config/secret
resolution, API/web replay metadata, finance schemas, valuation fixtures,
document/RAG fixtures, report/artifact contracts, compliance gates, packaging
metadata, generic AI evaluation harness parity, quant workflow fixtures, and
web-product smoke metadata.

This is not production-package parity with FinRobot. The remaining uncovered
work is live-package implementation beyond the checked-in skeleton contracts:
real provider clients, real document parsers, real chart/HTML/PDF renderers,
real database/web orchestration, optional integrations, and broader
notebook/tutorial parity.

The generic AI dialect convergence items are no longer missing or planned
FinRobot work. They are checked-in package boundaries under
`live_packages/generic_*`, with manifests, contracts, fixtures, registered
examples, and tests. Current inventory finds 21 registered generic live-package
examples covering model resolution, model IO envelopes, coding workspaces,
document RAG, evidence/report artifacts, UI snapshot evaluation, chart render
contracts, optional adapter boundaries, prompt/role catalogs, memory stores, turns, tools,
agents, workflows, evaluation, replay, trace, approval, and package auditing; FinRobot is only one
consumer of those generic boundaries. The same inventory finds three top-level
generic composition examples, including `generic_workflow_orchestration.leia`;
these are covered examples, not planned or missing dialect work.

## Inventory Commands

The following read-only inventory commands were run from this worktree on
2026-06-12:

| Command | Result |
| --- | --- |
| `rg --files examples/ai/finrobot_translation \| wc -l \| tr -d ' '` | `685` files |
| `go run ./cmd/leia examples list --json` filtered to `examples/ai/finrobot_translation/` | `88` examples: `78` `host-vm`, `6` `llm-replay`, `5` `evaluate` |
| `find examples/ai/finrobot_translation/live_packages -mindepth 1 -maxdepth 1 -type d ...` | `43` live-package skeleton directories |
| `find examples/ai/finrobot_translation/live_packages -path '*/fixtures/provider_free_fixture_index.json' -type f ...` | `42` provider-free fixture indexes |
| `go run ./cmd/leia examples list --json` filtered to `/live_packages/generic_` | `21` registered generic AI live-package examples |
| Same example inventory filtered to top-level `generic_*.leia` examples | `3` registered generic AI composition examples |
| Same example inventory filtered to `/tutorial_parity/runnable/` | `13` registered tutorial parity examples |
| `rg -n "generic\|AI dialect\|dialect\|planned\|missing\|guard\|semantic\|inventory" examples/ai/finrobot_translation/{COVERAGE.md,VERIFICATION.md,GAPS.md}` | Documentation semantic-guard search confirmed generic AI dialect status is documented as checked-in coverage, not planned or missing work |

This refresh updated the file-inventory count to 695 `rg --files` inventory files. The registered
example, runner, live-package, generic live-package, and tutorial runnable
counts now include the generic optional adapter boundary package.

## Registered Example Inventory

| ID | Path | Runner | Coverage role |
| --- | --- | --- | --- |
| `repo-ai-finrobot_translation-agent_builder_replay` | `agent_builder_replay.leia` | `host-vm` | Provider-free AgentBuilder roster, config save/load, max-round, tool assignment, and trace replay |
| `repo-ai-finrobot_translation-api_replay` | `api_replay.leia` | `host-vm` | API/web replay substrate, provider metadata, pagination, errors |
| `repo-ai-finrobot_translation-compliance_policy` | `compliance_policy.leia` | `host-vm` | Safety, approval, capability, publication, network, credential gates |
| `repo-ai-finrobot_translation-config_secret_example` | `config_secret_example.leia` | `host-vm` | Secret-free config/env handling and redaction |
| `repo-ai-finrobot_translation-core_agents-main` | `core_agents/main.leia` | `llm-replay` | Role/tool/agent contract fixture |
| `repo-ai-finrobot_translation-core_agents-workflow_handoff` | `core_agents/workflow_handoff.leia` | `llm-replay` | Parent/child workflow handoff and trace fixture |
| `repo-ai-finrobot_translation-core_agents-workflow_lifecycle` | `core_agents/workflow_lifecycle.leia` | `evaluate` | Provider-free workflow lifecycle fixture for reset, cache, trigger, TERMINATE, nested summary, handoff trace, and max round |
| `repo-ai-finrobot_translation-data_normalization` | `data_normalization.leia` | `host-vm` | Typed table and nested JSON normalization |
| `repo-ai-finrobot_translation-data_tools` | `data_tools.leia` | `host-vm` | Finance tool inventory and provider-free tool outputs |
| `repo-ai-finrobot_translation-document_rag` | `document_rag.leia` | `host-vm` | SEC/earnings document chunks and RAG corpus contracts |
| `repo-ai-finrobot_translation-equity_cli_workflow` | `equity_cli_workflow.leia` | `evaluate` | Equity-report stage DAG and artifact metadata |
| `repo-ai-finrobot_translation-equity_report` | `equity_report.leia` | `llm-replay` | End-to-end equity report replay fixture |
| `repo-ai-finrobot_translation-finance_normalizers` | `finance_normalizers.leia` | `host-vm` | Finance statement, market, peer, SEC, news schemas |
| `repo-ai-finrobot_translation-generated_code_tooling` | `generated_code_tooling.leia` | `host-vm` | Capability-gated code/file/image tooling envelope |
| `repo-ai-finrobot_translation-generic_agent_loop_composition` | `generic_agent_loop_composition.leia` | `evaluate` | Provider-free generic AI agent loop composition across agent runner, turn runner, tool contracts, and trace events |
| `repo-ai-finrobot_translation-generic_ai_workflow_composition` | `generic_ai_workflow_composition.leia` | `host-vm` | Provider-free Leia-native composition across generic model, model IO envelope, turn, tool, agent, workflow, eval, replay, trace, approval, and package-audit boundaries |
| `repo-ai-finrobot_translation-generic_workflow_orchestration` | `generic_workflow_orchestration.leia` | `evaluate` | Provider-free generic workflow orchestration guard for stage DAG, replayed agent handoff, trace hooks, and gate reporting |
| `repo-ai-finrobot_translation-live_packages-analytics_report-analytics_report` | `live_packages/analytics_report/analytics_report.leia` | `host-vm` | Checked-in analytics report live-package skeleton for normalizers, valuation, chart specs, report manifests, and renderer contracts |
| `repo-ai-finrobot_translation-live_packages-analyzer_report-main` | `live_packages/analyzer_report/main.leia` | `host-vm` | Checked-in analyzer report live-package skeleton for section schemas, evidence rules, citations, and report envelopes |
| `repo-ai-finrobot_translation-live_packages-backtest_strategy-main` | `live_packages/backtest_strategy/main.leia` | `host-vm` | Checked-in backtest strategy live-package skeleton for strategy manifests, data feeds, trade ledgers, metrics, risk limits, deterministic seeds, and optional dependency skips |
| `repo-ai-finrobot_translation-live_packages-chart_renderer-main` | `live_packages/chart_renderer/main.leia` | `host-vm` | Checked-in chart renderer live-package skeleton for chart specs, render envelopes, source metadata, stale warnings, dimensions/theme, snapshot hashes, and clean skips |
| `repo-ai-finrobot_translation-live_packages-coding_notebook-main` | `live_packages/coding_notebook/main.leia` | `host-vm` | Checked-in coding notebook live-package smoke for sandbox approvals, denied commands, stdout/stderr capture, deterministic replay, file/image artifacts, and capability metadata |
| `repo-ai-finrobot_translation-live_packages-document_pipeline-main` | `live_packages/document_pipeline/main.leia` | `host-vm` | Checked-in document pipeline live-package skeleton for SEC filing search/fetch, markdown conversion, chunks, citations, and retriever adapters |
| `repo-ai-finrobot_translation-live_packages-earnings_transcript-main` | `live_packages/earnings_transcript/main.leia` | `host-vm` | Checked-in earnings transcript live-package skeleton for speaker cleaning, date correction, quarter/year lookup, segment provenance, chunking, and HTTP clean skips |
| `repo-ai-finrobot_translation-live_packages-equity_analysis_pipeline-main` | `live_packages/equity_analysis_pipeline/main.leia` | `host-vm` | Checked-in equity analysis pipeline skeleton for fetch, normalize, forecast, section-agent handoff, artifact plan, failure hooks, and provider-free trace |
| `repo-ai-finrobot_translation-live_packages-factor_research-main` | `live_packages/factor_research/main.leia` | `host-vm` | Checked-in factor research live-package skeleton for factor transforms, market/factor data fixtures, exposures, and agent handoff boundaries |
| `repo-ai-finrobot_translation-live_packages-finance_facade-main` | `live_packages/finance_facade/main.leia` | `host-vm` | Checked-in finance facade live-package skeleton for provider fallback, typed tables, cache/retry metadata, provenance, and error envelopes |
| `repo-ai-finrobot_translation-live_packages-finance_normalizers-main` | `live_packages/finance_normalizers/main.leia` | `host-vm` | Checked-in finance normalizers live-package skeleton for statements, ratios, market, news, SEC, peer, provenance, field policy, and deterministic ordering |
| `repo-ai-finrobot_translation-live_packages-generic_agent_runner-main` | `live_packages/generic_agent_runner/main.leia` | `host-vm` | Checked-in generic AI agent runner skeleton for declarative agent config, replayed tool history, loop traces, structured output, and max-step guards |
| `repo-ai-finrobot_translation-live_packages-generic_approval_policy-main` | `live_packages/generic_approval_policy/main.leia` | `host-vm` | Checked-in generic AI approval/capability policy skeleton for default-deny gates, approval traces, clean skips, risk levels, and policy outcomes |
| `repo-ai-finrobot_translation-live_packages-generic_chart_render_contracts-main` | `live_packages/generic_chart_render_contracts/main.leia` | `host-vm` | Checked-in generic AI chart render contracts skeleton for chart specs, recipe semantic matrices, render request/result envelopes, source metadata, deterministic snapshot hashes, and unsupported renderer clean skips |
| `repo-ai-finrobot_translation-live_packages-generic_coding_workspace-main` | `live_packages/generic_coding_workspace/main.leia` | `host-vm` | Checked-in generic coding workspace skeleton for sandbox command envelopes, approval gates, stdout/stderr captures, file/image artifacts, notebook display metadata, cleanup policy, deterministic replay, and clean skips |
| `repo-ai-finrobot_translation-live_packages-generic_document_rag_pipeline-main` | `live_packages/generic_document_rag_pipeline/main.leia` | `host-vm` | Checked-in generic document RAG skeleton for document conversion, section extraction, chunk provenance, corpus manifests, retrieval citations, and adapter clean skips |
| `repo-ai-finrobot_translation-live_packages-generic_evidence_report_artifacts-main` | `live_packages/generic_evidence_report_artifacts/main.leia` | `host-vm` | Checked-in generic evidence/report/artifact skeleton for source annotations, citation envelopes, report outlines, render manifests, snapshot metadata, stale warnings, accessibility checks, and renderer clean skips |
| `repo-ai-finrobot_translation-live_packages-generic_evaluation_harness-main` | `live_packages/generic_evaluation_harness/main.leia` | `host-vm` | Checked-in generic AI evaluation harness skeleton for datasets, cases, metrics, replayed judges, findings, and golden gates |
| `repo-ai-finrobot_translation-live_packages-generic_memory_store-main` | `live_packages/generic_memory_store/main.leia` | `host-vm` | Checked-in generic AI memory store skeleton for namespace policy, memory item provenance, deterministic retrieval ranking, context windows, and clean-skip adapter boundaries |
| `repo-ai-finrobot_translation-live_packages-generic_model_io_envelope-main` | `live_packages/generic_model_io_envelope/main.leia` | `host-vm` | Checked-in generic AI model IO envelope skeleton for request, stream chunk, response, usage, replay correlation, and redaction contracts |
| `repo-ai-finrobot_translation-live_packages-generic_model_registry-main` | `live_packages/generic_model_registry/main.leia` | `host-vm` | Checked-in generic AI model registry skeleton for aliases, provider policy, replay-safe execution descriptors, redaction, and capability flags |
| `repo-ai-finrobot_translation-live_packages-generic_optional_adapter_boundary-main` | `live_packages/generic_optional_adapter_boundary/main.leia` | `host-vm` | Checked-in generic optional adapter boundary skeleton for optional dependency registries, clean-skip gates, result envelopes, version and terms metadata, credential redaction, and no-live-import defaults |
| `repo-ai-finrobot_translation-live_packages-generic_package_boundary_auditor-main` | `live_packages/generic_package_boundary_auditor/main.leia` | `host-vm` | Checked-in generic AI package-boundary auditor skeleton for manifest, fixture index, example registry, capability policy, findings, and missing boundary records |
| `repo-ai-finrobot_translation-live_packages-generic_planning_graph-main` | `live_packages/generic_planning_graph/main.leia` | `host-vm` | Checked-in generic AI planning graph skeleton for plan nodes, dependencies, retry policy, branch/merge joins, and trace evidence |
| `repo-ai-finrobot_translation-live_packages-generic_prompt_role_catalog-main` | `live_packages/generic_prompt_role_catalog/main.leia` | `host-vm` | Checked-in generic prompt/role catalog skeleton for role profile versions, prompt templates, delegation triggers, output schemas, evidence validation, and termination conventions |
| `repo-ai-finrobot_translation-live_packages-generic_record_replay-main` | `live_packages/generic_record_replay/main.leia` | `host-vm` | Checked-in generic AI record/replay skeleton for record schemas, strict ordered matching, mismatch findings, unconsumed records, and deterministic summaries |
| `repo-ai-finrobot_translation-live_packages-generic_tool_contracts-main` | `live_packages/generic_tool_contracts/main.leia` | `host-vm` | Checked-in generic AI tool contract skeleton for argument validation, approval state, result envelopes, normalized errors, and artifact refs |
| `repo-ai-finrobot_translation-live_packages-generic_tool_registry-main` | `live_packages/generic_tool_registry/main.leia` | `host-vm` | Checked-in generic AI tool registry skeleton for descriptors, schema validation, invocation traces, and approval edges |
| `repo-ai-finrobot_translation-live_packages-generic_trace_events-main` | `live_packages/generic_trace_events/main.leia` | `host-vm` | Checked-in generic AI trace event skeleton for turns, streams, tools, artifacts, approvals, replay markers, redaction, and correlation IDs |
| `repo-ai-finrobot_translation-live_packages-generic_turn_runner-main` | `live_packages/generic_turn_runner/main.leia` | `host-vm` | Checked-in generic AI turn runner skeleton for single-turn requests, response/usage/error envelopes, tool requests, and replay matching |
| `repo-ai-finrobot_translation-live_packages-generic_ui_snapshot_evaluator-main` | `live_packages/generic_ui_snapshot_evaluator/main.leia` | `host-vm` | Checked-in generic UI snapshot evaluator skeleton for route DOM schemas, viewport matrices, visual diff budgets, accessibility summaries, artifact URI manifests, redaction policy, static asset policy, and browser clean skips |
| `repo-ai-finrobot_translation-live_packages-generic_workflow_orchestrator-main` | `live_packages/generic_workflow_orchestrator/main.leia` | `host-vm` | Checked-in generic AI workflow orchestrator skeleton for workflow graphs, stage I/O, handoff traces, retry/cache policy, workflow results, and trace hooks |
| `repo-ai-finrobot_translation-live_packages-html_ui_snapshots-main` | `live_packages/html_ui_snapshots/main.leia` | `host-vm` | Checked-in HTML/UI snapshot live-package skeleton for template inventory, required sections, table/chart placeholders, disclosure/provenance markup, accessibility, static assets, and deterministic snapshot hashes |
| `repo-ai-finrobot_translation-live_packages-news_catalyst-main` | `live_packages/news_catalyst/main.leia` | `host-vm` | Checked-in news/catalyst live-package skeleton for news relevance, source ranking, retail sentiment, and adapter boundaries |
| `repo-ai-finrobot_translation-live_packages-optional_integrations-main` | `live_packages/optional_integrations/main.leia` | `host-vm` | Checked-in optional integrations live-package skeleton for FinGPT, FinRL, FinML, Backtrader, mplfinance, OpenBB, and Ollama gates |
| `repo-ai-finrobot_translation-live_packages-product_workflow-main` | `live_packages/product_workflow/main.leia` | `host-vm` | Checked-in product workflow live-package smoke for routes, auth/session, task logs, downloads, DB, UI snapshots, accessibility, and deployment contracts |
| `repo-ai-finrobot_translation-live_packages-prompt_roles-main` | `live_packages/prompt_roles/main.leia` | `host-vm` | Checked-in prompt/role live-package skeleton for role profile versioning, section output schemas, TERMINATE convention, and source evidence validation |
| `repo-ai-finrobot_translation-live_packages-report_renderer-main` | `live_packages/report_renderer/main.leia` | `host-vm` | Checked-in report renderer live-package skeleton for HTML/PDF render requests, output manifests, page snapshots, warnings, disclosures, source annotations, missing charts, and fixture hashes |
| `repo-ai-finrobot_translation-live_packages-retail_sentiment-main` | `live_packages/retail_sentiment/main.leia` | `host-vm` | Checked-in retail sentiment live-package skeleton for Adanos, Reddit, X, and Polymarket snapshots, aggregates, redaction, terms metadata, stale warnings, prompt formatting, and clean skips |
| `repo-ai-finrobot_translation-live_packages-sec_filings-main` | `live_packages/sec_filings/main.leia` | `host-vm` | Checked-in SEC filings live-package skeleton for 10-K/10-Q search, fetch, section extraction, provenance, cache, user-agent, terms metadata, and live gate contracts |
| `repo-ai-finrobot_translation-live_packages-tutorial_demo_parity-main` | `live_packages/tutorial_demo_parity/main.leia` | `host-vm` | Checked-in tutorial/demo parity live-package skeleton for replay records, optional gates, and notebook conversion checks |
| `repo-ai-finrobot_translation-live_packages-valuation_engine-main` | `live_packages/valuation_engine/main.leia` | `host-vm` | Checked-in valuation engine live-package skeleton for DCF, EV/EBITDA, P/E, target synthesis, football-field data, assumption audit, tolerance gates, currency/period, and provenance |
| `repo-ai-finrobot_translation-live_packages-vendor_adapters-main` | `live_packages/vendor_adapters/main.leia` | `host-vm` | Checked-in vendor adapter live-package skeleton with schemas, fixtures, capabilities, terms metadata, and network-off policy |
| `repo-ai-finrobot_translation-model_alias_routing_example` | `model_alias_routing_example.leia` | `host-vm` | Model alias and route selection contract |
| `repo-ai-finrobot_translation-optional_integrations` | `optional_integrations.leia` | `host-vm` | Optional package manifest and skip/capability gates |
| `repo-ai-finrobot_translation-quant_experiments-investment_group` | `quant_experiments/investment_group.leia` | `llm-replay` | Investment committee workflow fixture |
| `repo-ai-finrobot_translation-quant_experiments-multi_factor_agents` | `quant_experiments/multi_factor_agents.leia` | `llm-replay` | Multi-factor workflow fixture |
| `repo-ai-finrobot_translation-quant_experiments-portfolio_optimization` | `quant_experiments/portfolio_optimization.leia` | `llm-replay` | Portfolio optimizer workflow fixture |
| `repo-ai-finrobot_translation-report_contract` | `report_contract.leia` | `host-vm` | Chart/report artifact schema boundary |
| `repo-ai-finrobot_translation-reporting` | `reporting.leia` | `host-vm` | HTML/PDF/product reporting boundary metadata |
| `repo-ai-finrobot_translation-role_profiles` | `role_profiles.leia` | `host-vm` | FinRobot-style role registry as data |
| `repo-ai-finrobot_translation-section_agents` | `section_agents.leia` | `host-vm` | Section prompt/output schemas |
| `repo-ai-finrobot_translation-sensitivity_math` | `sensitivity_math.leia` | `host-vm` | Matrix/sensitivity fixture math |
| `repo-ai-finrobot_translation-tutorial_parity-runnable-advanced_agent_annual_report` | `tutorial_parity/runnable/advanced_agent_annual_report.leia` | `host-vm` | Runnable provider-free advanced annual-report tutorial parity fixture |
| `repo-ai-finrobot_translation-tutorial_parity-runnable-advanced_agent_fingpt_forecaster` | `tutorial_parity/runnable/advanced_agent_fingpt_forecaster.leia` | `host-vm` | Runnable provider-free advanced FinGPT forecaster tutorial parity fixture |
| `repo-ai-finrobot_translation-tutorial_parity-runnable-advanced_agent_openbb` | `tutorial_parity/runnable/advanced_agent_openbb.leia` | `host-vm` | Runnable provider-free advanced OpenBB tutorial parity fixture |
| `repo-ai-finrobot_translation-tutorial_parity-runnable-advanced_agent_trade_strategist` | `tutorial_parity/runnable/advanced_agent_trade_strategist.leia` | `host-vm` | Runnable provider-free advanced trade strategist tutorial parity fixture |
| `repo-ai-finrobot_translation-tutorial_parity-runnable-advanced_lmm_agent_mplfinance` | `tutorial_parity/runnable/advanced_lmm_agent_mplfinance.leia` | `host-vm` | Runnable provider-free advanced mplfinance tutorial parity fixture |
| `repo-ai-finrobot_translation-tutorial_parity-runnable-advanced_lmm_agent_opt_smacross` | `tutorial_parity/runnable/advanced_lmm_agent_opt_smacross.leia` | `host-vm` | Runnable provider-free advanced SMA-cross optional strategy tutorial parity fixture |
| `repo-ai-finrobot_translation-tutorial_parity-runnable-beginner_agent_annual_report` | `tutorial_parity/runnable/beginner_agent_annual_report.leia` | `host-vm` | Runnable provider-free beginner annual-report tutorial parity fixture |
| `repo-ai-finrobot_translation-tutorial_parity-runnable-beginner_agent_fingpt_forecaster` | `tutorial_parity/runnable/beginner_agent_fingpt_forecaster.leia` | `host-vm` | Runnable provider-free beginner FinGPT forecaster tutorial parity fixture |
| `repo-ai-finrobot_translation-tutorial_parity-runnable-beginner_agent_rag_earnings_call_sec_filings` | `tutorial_parity/runnable/beginner_agent_rag_earnings_call_sec_filings.leia` | `host-vm` | Runnable provider-free beginner RAG earnings-call and SEC-filings tutorial parity fixture |
| `repo-ai-finrobot_translation-tutorial_parity-runnable-beginner_agent_rag_qa` | `tutorial_parity/runnable/beginner_agent_rag_qa.leia` | `host-vm` | Runnable provider-free beginner RAG QA tutorial parity fixture |
| `repo-ai-finrobot_translation-tutorial_parity-runnable-beginner_agent_rag_qa_up` | `tutorial_parity/runnable/beginner_agent_rag_qa_up.leia` | `host-vm` | Runnable provider-free beginner RAG QA update tutorial parity fixture |
| `repo-ai-finrobot_translation-tutorial_parity-runnable-beginner_ollama_function_call` | `tutorial_parity/runnable/beginner_ollama_function_call.leia` | `host-vm` | Runnable provider-free beginner Ollama function-call tutorial parity fixture |
| `repo-ai-finrobot_translation-tutorial_parity-runnable-beginner_ollama_stock_chart` | `tutorial_parity/runnable/beginner_ollama_stock_chart.leia` | `host-vm` | Runnable provider-free beginner Ollama stock-chart tutorial parity fixture |
| `repo-ai-finrobot_translation-tutorials-notebook_parity-annual_report` | `tutorials/notebook_parity/annual_report.leia` | `host-vm` | Provider-free annual-report notebook parity fixture |
| `repo-ai-finrobot_translation-tutorials-notebook_parity-ollama_function_call_optional_gate` | `tutorials/notebook_parity/ollama_function_call_optional_gate.leia` | `host-vm` | Provider-free optional Ollama function-call gate fixture |
| `repo-ai-finrobot_translation-tutorials-notebook_parity-rag_earnings_sec` | `tutorials/notebook_parity/rag_earnings_sec.leia` | `host-vm` | Provider-free RAG earnings/SEC notebook parity fixture |
| `repo-ai-finrobot_translation-valuation_analytics` | `valuation_analytics.leia` | `host-vm` | DCF, multiples, target synthesis fixture analytics |
| `repo-ai-finrobot_translation-vendor_adapters` | `vendor_adapters.leia` | `host-vm` | Vendor adapter package skeleton contracts |
| `repo-ai-finrobot_translation-web_product` | `web_product.leia` | `evaluate` | Web route/auth/task/download/CRUD smoke metadata |

Runner summary: 78 `host-vm`, 6 `llm-replay`, 5 `evaluate`, 89 runnable,
89 checkable.

## Module Coverage Matrix

Status key:

- `Covered`: provider-free example coverage exists and is registered or checked
  through the current slice.
- `Partial`: provider-free contracts exist, but production behavior is only
  represented by fixtures or metadata.
- `Uncovered live`: source behavior still needs live-package implementation.
- `Mapped only`: source is audited and mapped, but no direct executable parity
  exists.

| FinRobot source module or group | Current translation files | Status | What is covered now | Uncovered production implementation |
| --- | --- | --- | --- | --- |
| `README.md`, architecture, Pro pipeline | `README.md`, `GAPS.md`, `equity_report.leia`, `equity_cli_workflow.leia`, this audit | Covered | Workload decomposition, no-runtime-change boundary, provider-free equity report slice | Full Pro parity and live operational deployment |
| `finrobot/agents/agent_library.py` | `role_profiles.leia`, `core_agents/main.leia` | Covered | Role profiles, tool lists, prompt-style metadata as data | Full role catalog drift tracking against upstream changes |
| `finrobot/agents/prompts.py`, `finrobot/agents/utils.py` | `role_profiles.leia`, `section_agents.leia`, `core_agents/*.leia` | Partial | Prompt snapshots, role prompts, section output schemas | Complete prompt template library, trigger parsing parity, prompt version package |
| `finrobot/agents/workflow.py` | `core_agents/workflow_handoff.leia`, `core_agents/workflow_lifecycle.leia`, `evaluation_harness/manifest.json` | Covered | Provider-free workflow handoff, reset/cache/trigger lifecycle, TERMINATE convention, nested summary, trace hierarchy, replay records | Mutable AutoGen `GroupChatManager` runtime object parity, live provider workflow package |
| `finrobot/toolkits.py` | `data_tools.leia`, `vendor_adapters.leia`, `live_packages/vendor_adapters/main.leia` | Covered | Generic typed tool descriptors, capabilities, replayable result envelopes, checked-in vendor skeleton schemas and fixtures | AutoGen caller/executor registration parity, live network-backed tool invocations |
| `finrobot/utils.py`, package init | `config_secret_example.leia`, `model_alias_routing_example.leia`, `package_deploy_manifest.json` | Covered | Config aliases, env/secret diagnostics, package metadata | Complete package bootstrap and install-time config helpers |
| `finrobot/data_source/yfinance_utils.py` | `vendor_adapters.leia`, `finance_normalizers.leia`, `data_tools.leia`, `live_packages/vendor_adapters/*` | Partial | Yahoo adapter manifest, live-package skeleton fixture, schema, and output contracts | Live yfinance client, price history, dividends, company info, statements, cache/retry behavior |
| `finrobot/data_source/finnhub_utils.py` | `vendor_adapters.leia`, `finance_normalizers.leia`, `api_replay.leia`, `live_packages/vendor_adapters/*` | Partial | Finnhub manifest, auth/rate-limit metadata, live-package skeleton fixture, and schema | Live Finnhub client for profile, news, metrics, financials, errors |
| `finrobot/data_source/fmp_utils.py` | `vendor_adapters.leia`, `finance_normalizers.leia`, `api_replay.leia`, `live_packages/vendor_adapters/*` | Partial | FMP manifest, pagination fixture, key metrics, competitor schema metadata, and live-package skeleton schema | Live FMP client for statements, market cap, targets, peers, ratings, historical metrics |
| `finrobot/data_source/sec_utils.py` | `vendor_adapters.leia`, `document_rag.leia`, `api_replay.leia`, `live_packages/vendor_adapters/*` | Partial | SEC metadata, download replay, filing chunk contracts, live-package skeleton fixture and schema | Live SEC downloader, user-agent policy, cache, HTML/PDF conversion, section extraction |
| `finrobot/data_source/finnlp_utils.py`, optional `FinNLP` sources | `optional_integrations.leia`, `optional_integrations_manifest.json`, `live_packages/optional_integrations/*` | Partial | Optional integration gating, manifest metadata, clean-skip contract, and live-package skeleton fixture index | Live FinNLP dataset downloaders and schema normalizers |
| `finrobot/data_source/reddit_utils.py` | `vendor_adapters.leia`, `finance_normalizers.leia`, `optional_integrations.leia`, `live_packages/vendor_adapters/*` | Partial | Reddit adapter manifest, sentiment schema, terms/capability metadata, live-package skeleton fixture and schema | Live PRAW/auth/pagination/redaction implementation |
| `finrobot/data_source/finance_data.py` | `data_tools.leia`, `finance_normalizers.leia` | Partial | Provider facade shape through replayable tools and normalized fixtures | Live facade dispatch, provider fallback order, typed table package |
| `finrobot/data_source/earnings_calls_src/*` | `document_rag.leia`, `vendor_adapters.leia`, `live_packages/vendor_adapters/*` | Partial | Transcript chunk fixtures, RAG metadata, live-package skeleton fixture and schema | Live transcript retrieval, speaker parsing, date correction, LangChain document parity |
| `finrobot/data_source/filings_src/*` | `document_rag.leia`, `api_replay.leia` | Partial | SEC filing chunks, section-name metadata, replayed errors | Live filing search/fetch, HTML parser, section extractor, redirect/download cache |
| `finrobot/data_source/marker_sec_src/*` | `document_rag.leia`, `report_contract.leia` | Partial | PDF/document artifact provenance contract | Live PDF-to-markdown, parallel conversion, artifact persistence |
| `finrobot/functional/rag.py`, `ragquery.py` | `document_rag.leia`, `core_agents/main.leia` | Covered | Local corpus, citations, retrieval inputs/outputs, no live model dependency | Vector-store package and live retrieval providers |
| `finrobot/functional/analyzer.py` | `section_agents.leia`, `equity_report.leia` | Partial | Section-specific schema and replayed analysis text | Complete finance analyzer prompt library with source evidence enforcement |
| `finrobot/functional/charting.py` | `report_contract.leia`, `reporting.leia`, `equity_cli_workflow.leia`, `live_packages/analytics_report/*` | Partial | Chart spec/artifact metadata, dimensions, sources, stale-data checks, renderer skeleton contracts | Real stock, PE/EPS, revenue, EBITDA, margin, sensitivity, waterfall, radar chart rendering |
| `finrobot/functional/coding.py` | `generated_code_tooling.leia`, `compliance_policy.leia` | Covered | File/code/image tool envelopes, denied-command and approval gates | Sandboxed live Python execution and notebook display integration |
| `finrobot/functional/quantitative.py` | `quant_experiments/portfolio_optimization.leia`, `optional_integrations.leia`, `live_packages/optional_integrations/*` | Partial | Deterministic portfolio-stat fixture and optional Backtrader gate skeleton | Live Backtrader/strategy package, trade ledger, seed and data-source controls |
| `finrobot/functional/reportlab.py` | `report_contract.leia`, `reporting.leia`, `package_deploy_manifest.json`, `live_packages/analytics_report/*` | Partial | Report object/artifact boundaries, disclosure metadata, HTML/PDF renderer skeleton contracts | Styled PDF generation, pagination, fonts, image fitting, export package |
| `finrobot/functional/text.py` | `section_agents.leia`, `reporting.leia` | Covered | Text length/schema constraints through section/report fixtures | Broader reusable text utility package, if needed |
| `experiments/investment_group.py` | `quant_experiments/investment_group.leia` | Covered | Offline multi-agent investment group replay | Live market/filing/provider calls and full AutoGen execution lifecycle |
| `experiments/multi_factor_agents.py` | `quant_experiments/multi_factor_agents.leia` | Covered | Offline multi-factor agent replay | Live factor data package and full provider-backed factor transforms |
| `experiments/portfolio_optimization.py` | `quant_experiments/portfolio_optimization.leia` | Covered | Offline optimizer replay and deterministic stats | Live optimizer/backtest library and market-data integration |
| `tutorials_beginner/*` | `tutorial_parity/runnable/*`, `tutorials/notebook_parity/*`, existing registered examples | Partial | Provider-free runnable equivalents for annual report, RAG QA, RAG earnings/SEC, FinGPT forecaster, Ollama function call, and stock-chart concepts | Live optional-provider execution and broader notebook artifact parity |
| `tutorials_advanced/*` | `tutorial_parity/runnable/*`, `optional_integrations.leia`, `quant_experiments/*.leia`, `generated_code_tooling.leia`, `tutorial_parity/ledger.json`, `live_packages/optional_integrations/*` | Partial | Runnable provider-free parity for annual report, FinGPT forecaster, OpenBB, trade strategist, mplfinance, and optional SMA-cross examples; replayed quant workflows and tutorial parity ledger | Multimodal chart/document analysis and live optional package implementations |
| `agent_builder_demo.py`, `test_module.py` | `core_agents/main.leia`, `evaluation_harness/manifest.json` | Partial | Small deterministic agent/tool fixtures plus generic AI evaluation harness parity | Direct demo/test translation with matching assertions |
| `configs/*`, `OAI_CONFIG_LIST`, `config_api_keys` | `config_secret_example.leia`, `model_alias_routing_example.leia`, `package_deploy_manifest.json` | Covered | Secret-free config, missing key diagnostics, route metadata, deploy env docs | Full config migration helpers and live provider profile loading |
| `requirements*.txt`, `setup.py`, `Dockerfile`, `deploy*.sh`, `run_web_app.py` | `package_deploy_*`, `package_deploy_manifest.json` | Covered | Provider-free package/deploy metadata, Docker/gcloud/run commands, health entrypoint | Release packaging, real dependency extras, deployment smoke in target cloud |
| `finrobot_equity/README.md` | `equity_report/README.md`, `equity_report.leia`, `equity_cli_workflow.leia` | Covered | Equity research product workflow, report replay, stage metadata | Full generated product parity over live data and renderers |
| `finrobot_equity/core/src/generate_financial_analysis.py` | `equity_cli_workflow.leia`, `equity_report.leia` | Partial | Stage DAG, fixture inputs, replayed model outputs, report artifacts | Live fetch/process/forecast/agent orchestration and persistence |
| `finrobot_equity/core/src/create_equity_report.py`, `generate_pdf_report.py` | `report_contract.leia`, `reporting.leia`, `equity_cli_workflow.leia`, `live_packages/analytics_report/*` | Partial | HTML/PDF/report boundary specs, artifact manifests, and renderer skeleton contracts | Real HTML/PDF renderers, layout checks, image/table integration |
| `finrobot_equity/core/src/modules/market_data_api.py` | `vendor_adapters.leia`, `finance_normalizers.leia`, `live_packages/vendor_adapters/*` | Partial | FMP/YFinance source schemas, provenance fixtures, and checked-in provider skeleton fixtures | Live market data client, retry/cache/rate-limit policy |
| `finrobot_equity/core/src/modules/financial_data_processor.py` | `data_normalization.leia`, `finance_normalizers.leia`, `valuation_analytics.leia`, `live_packages/analytics_report/*` | Partial | Numeric cleaning, normalized financial schemas, fixture transforms, and normalizer skeleton contracts | Full pandas/dataframe parity, CSV persistence, missing-data behavior |
| `finrobot_equity/core/src/modules/valuation_engine.py` | `valuation_analytics.leia`, `sensitivity_math.leia`, `live_packages/analytics_report/*` | Partial | DCF, EV/EBITDA, P/E, target-price synthesis fixture math and valuation skeleton contracts | Production valuation engine package and full assumption sensitivity |
| `finrobot_equity/core/src/modules/sensitivity_analyzer.py` | `sensitivity_math.leia` | Covered | Revenue/margin sensitivity matrix fixture | Full heatmap/report integration |
| `finrobot_equity/core/src/modules/catalyst_analyzer.py`, `news_integrator.py` | `finance_normalizers.leia`, `section_agents.leia`, `equity_report.leia` | Partial | News/catalyst schemas and replayed section summaries | Live news fetch/classification, source ranking, sentiment impact model |
| `finrobot_equity/core/src/modules/retail_sentiment_client.py` | `vendor_adapters.leia`, `finance_normalizers.leia`, `optional_integrations.leia` | Partial | Retail sentiment schema and optional capability metadata | Live Adanos/Reddit/X/Polymarket snapshots and prompt formatting |
| `finrobot_equity/core/src/modules/text_generator_agents.py`, `enhanced_text_generator.py`, `equity_agents/*` | `section_agents.leia`, `role_profiles.leia`, `equity_report.leia` | Partial | Section agent roles, output schemas, replay records | Full per-agent prompt package, live model calls, source-evidence validators |
| `finrobot_equity/core/src/modules/chart_generator.py`, `enhanced_chart_generator.py` | `report_contract.leia`, `reporting.leia`, `live_packages/analytics_report/*` | Partial | Chart specs, report artifact references, and renderer skeleton contracts | Real chart package and image snapshot tests |
| `finrobot_equity/core/src/modules/html_renderer.py`, `html_template_professional.py` | `reporting.leia`, `web_product.leia` | Partial | HTML artifact boundary, template/snapshot metadata | Reusable templates, markdown/table rendering, accessibility and visual snapshots |
| `finrobot_equity/core/src/modules/pdf_generator.py`, `professional_pdf_report.py` | `reporting.leia`, `package_deploy_manifest.json` | Partial | PDF manifest boundary and export metadata | Styled PDF package, page-level render verification |
| `finrobot_equity/core/src/modules/report_structure.py`, `report_data_loader.py`, `common_utils.py` | `report_contract.leia`, `reporting.leia`, `equity_cli_workflow.leia` | Covered | Ordered section schemas, provenance, AI disclosure, stale checks, data manifest | Full loader/config utility parity across all report variants |
| `finrobot_equity/core/tests/*` | `evaluation_harness/manifest.json`, `evaluation_harness/README.md`, registered examples | Partial | Representative replay/evaluate checks and checksums | Port broader Python assertions into Leia tests/evaluations |
| `finrobot_equity/web_app/main.py`, `admin_routes.py`, `auth.py` | `web_product.leia`, `package_deploy_run_web_app.py`, `live_packages/product_workflow/*` | Partial | Route/auth/session/task/download smoke metadata, health entrypoint, and product workflow skeleton contracts | Real server, auth flow, background workers, status recovery |
| `finrobot_equity/web_app/database/*` | `web_product.leia`, `live_packages/product_workflow/*` | Partial | CRUD state model fixture plus DB migration skeleton contract and schema | SQLite/SQLAlchemy migration and CRUD package parity |
| `finrobot_equity/web_app/templates/*`, `static/*` | `web_product.leia`, `reporting.leia`, `live_packages/product_workflow/*` | Mapped only | Static/template snapshot metadata, accessibility checklist, and product workflow UI snapshot contract | Full UI asset parity, rendered page snapshots, visual regression |

## Uncovered Production Implementation Items

These items are intentionally outside the provider-free slice and should not be
mistaken for missing language/runtime support.

| Area | Uncovered item | Source modules |
| --- | --- | --- |
| Live finance providers | yfinance, Finnhub, FMP, SEC, Reddit/PRAW, FinNLP, earnings-call HTTP clients with auth, pagination, retry, cache, rate-limit, and terms metadata | `finrobot/data_source/*`, `finrobot_equity/core/src/modules/market_data_api.py`, `retail_sentiment_client.py` |
| Typed finance packages | Provider facade, typed table outputs, provenance-preserving statement/ratio/market/news schemas, provider fallback order | `finance_data.py`, `financial_data_processor.py`, `finance_normalizers` target |
| Document parsing | SEC HTML/PDF download, PDF-to-markdown, section extraction, local artifact persistence, deterministic chunking | `sec_utils.py`, `filings_src/*`, `marker_sec_src/*` |
| RAG backends | Vector-store setup, retrieval provider adapters, corpus lifecycle, live model-backed summary modes | `functional/rag.py`, `ragquery.py`, workflow RAG helpers |
| Chart rendering | Actual images for stock price, share performance, PE/EPS, revenue/EBITDA, EV/EBITDA, margin, sensitivity, waterfall, radar, technical indicators | `functional/charting.py`, chart generator modules |
| Report rendering | HTML template rendering, markdown/table rendering, PDF export, page layout, fonts, image fitting, accessibility and visual snapshots | `functional/reportlab.py`, HTML/PDF generator modules |
| Valuation and quant libraries | Production DCF/multiples engine, Backtrader strategy execution, optimizer package, live factor data transforms | `valuation_engine.py`, `sensitivity_analyzer.py`, `functional/quantitative.py`, experiments |
| Section-agent package | Complete prompt library, all equity-agent classes, output evidence validators, prompt versioning | `analyzer.py`, `text_generator_agents.py`, `enhanced_text_generator.py`, `equity_agents/*` |
| Optional integrations | FinGPT, FinRL, FinML, Backtrader, mplfinance, OpenBB, Ollama live package implementations beyond the checked-in clean-skip skeleton | `tutorials_*`, `optional_integrations` targets |
| Web product | Real web routes, auth, admin views, background workers, logs, report downloads, persistence, session recovery | `finrobot_equity/web_app/*` |
| Packaging/release | Installable external packages, dependency extras, cloud deployment smoke, artifact publish process | `setup.py`, requirements, Dockerfile, deploy scripts |
| Tutorial parity | Live optional-provider execution and broader notebook artifact parity beyond the registered provider-free runnable tutorial examples | `tutorials_beginner/*`, `tutorials_advanced/*` |

## Provider-Free Slice Completion

| Slice component | Evidence | Completion |
| --- | --- | --- |
| Registered example inventory | 89 registered examples under `examples/ai/finrobot_translation`; all runnable/checkable | Complete |
| File inventory | 695 files in the translation directory, including checked-in live-package skeleton directories, status docs, and the upstream coverage ledger | Complete |
| Replay-backed AI workflows | 6 `llm-replay` examples with checked-in records for core agents, equity report, and quant experiments | Complete |
| Host-VM contract examples | 78 `host-vm` examples for config, tools, schemas, API replay, reports, compliance, packaging, and live-package skeleton contracts | Complete |
| Evaluate-runner examples | 5 `evaluate` examples for equity CLI workflow, web product smoke metadata, generic workflow orchestration, and generic agent-loop composition | Complete |
| Live-package skeletons | 43 checked-in skeleton directories, including 21 reusable `live_packages/generic_*` AI boundaries and 22 FinRobot/finance package boundaries; all 43 include registered `.leia` examples | Complete |
| Generic AI dialect package boundary | The reusable `live_packages/generic_*` set is checked in as package-owned generic AI surface, not a FinRobot-specific dialect or language/runtime change | Complete |
| Provider independence | Examples use fixtures, replay records, manifests, and optional capability gates instead of live credentials | Complete |
| Evaluation harness | `evaluation_harness/manifest.json` inventories replay records, golden checksums, gates, and report metadata | Complete for current registered records |
| Gap ledger alignment | `GAPS.md` records no open gaps for the provider-free slice; remaining work is package/product implementation | Complete |
| Production parity | Live provider clients, renderers, DB/web orchestration, optional integrations, and full notebooks beyond the skeleton contracts | Not complete by design |

Provider-free completion result: complete for the currently registered
89-example translation slice, including live-package skeleton contracts.
Production/live-package parity result: incomplete by design.

## Next-Phase Live-Package Tasks

1. Finance provider packages: turn the checked-in vendor-adapter skeleton into
   Yahoo, Finnhub, FMP, SEC, Reddit, FinNLP, and earnings-call clients behind
   explicit capabilities, credentials, rate limits, terms metadata, replay
   recording, and fixture refresh tooling.
2. Finance normalizer package: promote the fixture schemas in
   `finance_normalizers.leia` into reusable typed packages with provenance,
   stale-data checks, statement cleaning, peer/news normalization, and provider
   fallback behavior.
3. Document/RAG package: implement SEC filing download, HTML/PDF parsing,
   PDF-to-markdown, chunking, vector-store adapters, corpus lifecycle, and
   citation validation while keeping provider-free replay gates.
4. Valuation and quant package: implement production DCF/multiples,
   sensitivity, portfolio optimization, and Backtrader-style backtest packages
   with deterministic fixture tests.
5. Chart/report package: replace chart specs and report manifests with real
   chart rendering, HTML rendering, PDF export, page-level render checks,
   disclosure components, and accessibility snapshots.
6. Section-agent package: package the complete prompt library and equity-agent
   role set with source-evidence validators, structured output schemas, prompt
   versioning, and live/replay model gates.
7. Web product package: implement the product server, auth, admin routes,
   background workflow runner, logs, downloads, SQLite/CRUD persistence, status
   recovery, and static/template visual checks.
8. Optional integration gates: add live smoke gates for FinGPT, FinRL, FinML,
   Backtrader, mplfinance, OpenBB, and Ollama without making them default CI
   dependencies.
9. Tutorial parity: translate each beginner and advanced notebook into a
   provider-free example plus an optional live-provider gate where appropriate.
10. Release packaging: ship package manifests, dependency extras, Docker/cloud
    smoke checks, fixture refresh commands, and documentation that distinguishes
    offline examples from live-package usage.
