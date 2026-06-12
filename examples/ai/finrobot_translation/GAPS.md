# FinRobot Translation Gap Register

This register tracks the remaining gaps after translating FinRobot into generic
Leia AI dialect examples. It intentionally avoids FinRobot-specific language
features: domain behavior belongs in packages, package data, replay fixtures, or
external adapters.

Provider-free coverage evidence is summarized in `COVERAGE.md`. Reproducible
checks are listed in `VERIFICATION.md`. Live production package work that
replaces fixtures with vendor integrations is tracked in
`live_package_plan.md` and `live_package_plan_manifest.json`.

## Gap Categories

| Category | Meaning for FinRobot translation |
| --- | --- |
| `language` | Missing syntax or core language feature. Current audit finds no FinRobot-specific language gap. |
| `AI dialect` | Missing or incomplete `model`, `turn`, `tool`, `agent`, `workflow`, approval, trace, replay, or capability contract. Current audit finds no open generic AI dialect gap because those boundaries are checked in as reusable `live_packages/generic_*` packages, not FinRobot-only surfaces. |
| `stdlib` | Missing generic API/web/file/config/data/table/document/chart/report/db support. |
| `external library` | Missing finance, vendor, model, chart, report, notebook, or backtesting package. |
| `product layer` | Missing deployment, auth, UI, compliance, logging, operational, or packaging work. |

## Open Gaps

No open gaps remain for the provider-free Leia translation slice in this
directory. The current provider-free surface is 82 registered examples plus
35 checked-in live-package skeleton directories:
`live_packages/analytics_report`, `live_packages/analyzer_report`,
`live_packages/backtest_strategy`, `live_packages/chart_renderer`,
`live_packages/coding_notebook`, `live_packages/document_pipeline`,
`live_packages/earnings_transcript`, `live_packages/equity_analysis_pipeline`,
`live_packages/factor_research`, `live_packages/finance_facade`,
`live_packages/finance_normalizers`,
`live_packages/generic_agent_runner`, `live_packages/generic_approval_policy`,
`live_packages/generic_evaluation_harness`, `live_packages/generic_model_io_envelope`,
`live_packages/generic_model_registry`,
`live_packages/generic_package_boundary_auditor`, `live_packages/generic_planning_graph`,
`live_packages/generic_record_replay`, `live_packages/generic_tool_contracts`,
`live_packages/generic_tool_registry`, `live_packages/generic_trace_events`,
`live_packages/generic_turn_runner`, `live_packages/generic_workflow_orchestrator`,
`live_packages/html_ui_snapshots`, `live_packages/news_catalyst`,
`live_packages/optional_integrations`, `live_packages/product_workflow`,
`live_packages/prompt_roles`, `live_packages/report_renderer`,
`live_packages/retail_sentiment`, `live_packages/tutorial_demo_parity`,
`live_packages/sec_filings`, `live_packages/valuation_engine`, and
`live_packages/vendor_adapters`.
Remaining production work is external package
implementation work, not missing language or AI dialect surface: promoting the
checked-in skeleton contracts into live external packages, hardening product UI,
adding real renderers, and shipping optional provider integrations behind
capability gates.

The generic AI dialect convergence status is checked-in package boundary, not
missing or planned. The `live_packages/generic_*` directories define reusable AI
package contracts for model, model IO envelopes, turn, tool, agent, workflow, evaluation, replay,
trace, approval, capability, and package-audit behavior. They are intentionally
generic AI surfaces that the FinRobot translation consumes; they are not
FinRobot-specific language features or FinRobot-owned product packages.

The 2026-06-12 documentation semantic-guard refresh used read-only inventory to
confirm 13 registered `live_packages/generic_*` examples and three top-level
generic composition examples. It also rechecked approval/model/workflow/trace/eval
wording in the coverage, verification, and gap documents. That guard keeps
generic AI dialect capability in the closed/non-gap bucket unless a future
release gate or inventory check regresses.

## Translation Slice Gaps

### Core Agents

- AutoGen `GroupChat` speaker selection is translated as explicit coordinator history and assertions. Leia has agents, tools, turns, and history, but no drop-in mutable `GroupChatManager` object.
- FinRobot `UserProxyAgent` code execution is represented as ordinary tools or tool-result history, not exact `human_input_mode`, `max_consecutive_auto_reply`, or `code_execution_config` lifecycle knobs.
- FinRobot RAG wiring through `get_rag_function(...)` is expressible as a normal Leia tool, but vector-store setup and retrieval backends remain package resources.
- Nested chat summary modes such as `summary_method="reflection_with_llm"` are modeled with structured specialist output plus a follow-up leader turn.
- `TERMINATE` remains a prompt-level convention; Leia validates completion through structured outputs and assertions instead of reinterpreting it as a control signal.

### Data Tools

- Real provider clients are intentionally not translated: yfinance, Finnhub, SEC API, FMP, Reddit/PRAW, FinNLP downloaders, and earnings-call HTTP calls are represented by replay documents, `llm.tool`, and vendor-adapter skeleton contracts.
- DataFrame-specific behavior is approximated as Leia tables with field projection and row limits; CSV/file save paths, pandas index handling, and provider pagination are not modeled.
- SEC filing download/render/PDF conversion/cache behavior is reduced to replay metadata plus section text evidence.
- Earnings-call transcript parsing is represented as pre-split speaker segments; retry behavior, date correction, and LangChain `Document` interoperability remain out of scope.
- Toolkit registration is translated as a `register_data_toolkit()` list of `llm.tool` values, not AutoGen caller/executor registration or class-method decoration.

### Equity Report

- Live FMP/market/news/retail-sentiment API access is represented by local fixtures so the workflow can replay offline.
- Pandas dataframe transformations, CSV persistence, and chart generation are summarized as normalized fixture tools rather than translated as runtime data processing.
- Professional HTML, legacy page rendering, and PDF generation are outside this workflow because rendering is covered by the reporting boundary.
- Enhanced modules such as sensitivity heatmaps, technical indicators, valuation waterfall charts, and retail sentiment details are represented as structured summaries only.
- Web search for leadership, competitors, and valuation context is replaced with replay fixtures.

### Quant Experiments

- AutoGen nested-chat behavior is represented as declarative Leia agents and local tools; there is no caller/executor registration, cache disk state, Docker execution, or human-input mode.
- External market and filing sources are not invoked. Dow 30 factor data, PDD/peer committee inputs, SEC 20-F evidence, and news/sentiment inputs are fixed replay fixtures.
- BackTrader integration is reduced to a deterministic portfolio-stat tool.
- `ReportAnalysisUtils`, `CodingUtils`, and `IPythonUtils` are represented by replayable prompt/code tools only.
- q coverage is limited to table ranking, aggregation, weighting, caps, and order skeletons needed for these examples; this work does not add or change q runtime behavior.

### Reporting/Web

- Chart rendering is represented as chart specs and report artifact skeletons only. A real translation still needs a chart package for stock price, share-performance, PE/EPS, revenue/EBITDA, EV/EBITDA, margin, sensitivity, technical-indicator, waterfall, radar, and comparison charts.
- HTML rendering is modeled as a deterministic artifact boundary. The full template system still needs reusable template assets, table renderers, markdown-to-HTML conversion, fallback formatting, disclosure components, and source-provenance markup.
- PDF generation has a provider-free artifact skeleton but no live renderer. A document/export package must provide styled A4 layouts, frames/columns, cover pages, table pagination, image fitting, page headers/footers, font registration, and HTML-to-PDF or report-object-to-PDF conversion.
- Web orchestration is captured as staged task metadata only; routes, workers, logs, persistence, downloads, auth, admin views, static assets, and status recovery remain product-layer work.
- Artifact contracts have provider-free skeleton coverage; live package APIs still need to promote those schemas for report sections, chart specs, source annotations, AI markers, stale-data checks, HTML/PDF output manifests, and user-visible warnings.

## Closed Or Non-Gaps

| Item | Decision | Validation |
| --- | --- | --- |
| FinRobot-specific parser syntax | Not needed. FinRobot maps to general AI, data, web/API, document, chart, report, workflow, and package capabilities. | Translation review rejects new parser keywords for `finrobot`, `ticker`, `sec`, `trading`, valuation, or report-section concepts. |
| FR-GAP-001 typed tool contracts | Implemented generically for `llm.tool` and agent-as-tool contracts via `llm.tool_info` and `llm.validate_tools`, exporting schema, capabilities, result, error, and replay key metadata. | `tests/llm/llm_tool_contract_test.go` asserts inventory export, validation failures, and provider-schema compatibility for agent-as-tool. |
| FR-GAP-002 agent workflow traces | Implemented generically for agent-as-tool and nested workflow trace contracts, with FinRobot core-agent handoff fixtures in the evaluation inventory. | `tests/llm/llm_trace_workflow_contract_test.go` and `tests/llm/finrobot_evaluation_harness_test.go` assert trace hierarchy and handoff replay. |
| FR-GAP-003 provider-free replay | Implemented through offline replay fixtures plus evaluation manifest checks for all current FinRobot translation records. | `tests/llm/finrobot_evaluation_harness_test.go` runs `leia evaluate --gate --replay` without provider environment. |
| FR-GAP-004 model alias routing | Implemented through `llm.config.aliases` and `llm.config.route`, with trace-visible provider/model selection and deterministic replay keys. | `tests/llm/llm_model_alias_routing_test.go` and `examples/ai/finrobot_translation/model_alias_routing_example.leia`. |
| FR-GAP-005 config/env/secret handling | Implemented through config secret resolution, missing-key diagnostics, redaction, environment capability checks, and local deploy defaults. | `tests/llm/llm_config_secret_test.go` plus FR-GAP-021 environment manifest checks. |
| FR-GAP-006 API substrate | Implemented as a provider-free API replay substrate covering auth metadata, JSON payload shape, pagination, rate-limit/cache metadata, and traceable errors. | `tests/llm/llm_api_replay_example_test.go` runs `examples/ai/finrobot_translation/api_replay.leia`. |
| FR-GAP-007 web/download substrate | Implemented at the replay contract level for redirects/download metadata, generated artifacts, scraping terms, and provider-free web replay. | `tests/llm/llm_api_replay_example_test.go` validates web/download replay metadata in `api_replay.leia`. |
| FR-GAP-008 typed table/data support | Implemented as fixture-backed typed table/data normalization with joins, rolling metrics, CSV/JSON-shaped records, nested JSON flattening, and provenance checks. | `tests/llm/llm_data_normalization_test.go` runs `examples/ai/finrobot_translation/data_normalization.leia`. |
| FR-GAP-009 vector/matrix/math support | Implemented for the translation slice as deterministic sensitivity matrix and optimization-tolerance fixtures without changing q/runtime internals. | `tests/llm/llm_valuation_analytics_test.go` runs `examples/ai/finrobot_translation/sensitivity_math.leia`. |
| FR-GAP-010 document parsing | Implemented as local document artifact/chunk contracts for SEC filings and earnings-call text, including artifact IDs and parse metadata. | `tests/llm/llm_document_rag_contract_test.go` runs `examples/ai/finrobot_translation/document_rag.leia`. |
| FR-GAP-011 RAG contracts | Implemented as provider-free local corpus/retrieval contracts over document chunks with citations and reset behavior. | `tests/llm/llm_document_rag_contract_test.go` asserts retrieval inputs, chunks, citations, and no live provider dependency. |
| FR-GAP-012 chart/report contracts | Implemented as artifact/report contracts with declared inputs, sources, artifact IDs, dimensions, stale-data checks, and AI disclosure markers. | `tests/llm/llm_artifact_report_contract_test.go` runs `examples/ai/finrobot_translation/report_contract.leia`. |
| FR-GAP-013 finance vendor adapters | Implemented as external-package skeleton manifests for Yahoo, Finnhub, FMP, SEC, earnings, Reddit, and related providers with credentials, schemas, rate limits, fixtures, and terms metadata. | `tests/llm/llm_vendor_adapter_normalizer_test.go` runs `examples/ai/finrobot_translation/vendor_adapters.leia`; `tests/llm/llm_vendor_live_package_test.go` validates `live_packages/vendor_adapters`. |
| FR-GAP-014 finance normalizers | Implemented as fixture-backed schemas for statements, ratios, market data, recommendations, SEC sections, peers, and news/sentiment records with provenance and stale-field checks. | `tests/llm/llm_vendor_adapter_normalizer_test.go` runs `examples/ai/finrobot_translation/finance_normalizers.leia`; `tests/llm/llm_analytics_report_live_package_test.go` validates the analytics report skeleton normalizer contracts. |
| FR-GAP-015 valuation and analytics packages | Implemented for the translation slice as deterministic DCF, EV/EBITDA, P/E, target-price synthesis, sensitivity, and tolerance fixtures. | `tests/llm/llm_valuation_analytics_test.go` runs `examples/ai/finrobot_translation/valuation_analytics.leia`; `tests/llm/llm_analytics_report_live_package_test.go` validates the checked-in valuation/report skeleton. |
| FR-GAP-016 prompt/role/report-section packages | Implemented as data-only role registry, prompt render snapshots, section output schemas, and report taxonomy fixtures. | `tests/llm/llm_role_section_package_data_test.go` runs `examples/ai/finrobot_translation/role_profiles.leia` and `section_agents.leia`. |
| FR-GAP-017 generated-code and notebook tooling | Implemented as capability-gated replay envelopes for file read/write, generated-code execution, image display, and denied command cases. | `tests/llm/llm_generated_code_tooling_test.go` runs `examples/ai/finrobot_translation/generated_code_tooling.leia`. |
| FR-GAP-018 optional integrations | Implemented as optional package manifests and clean skip/capability gates for FinGPT, FinRL, FinML, Backtrader, mplfinance, Ollama, and OpenBB without importing those dependencies. | `tests/llm/llm_optional_integration_gating_test.go` runs `examples/ai/finrobot_translation/optional_integrations.leia`; `tests/llm/llm_optional_integrations_live_package_test.go` validates `live_packages/optional_integrations`. |
| FR-GAP-019 equity report CLI workflow | Implemented as named stages with dependencies, artifacts, retry/stale-section metadata, chart/report outputs, and manifest checks. | `tests/llm/llm_equity_cli_workflow_test.go` runs `examples/ai/finrobot_translation/equity_cli_workflow.leia`. |
| FR-GAP-020 web app product layer | Implemented as provider-free web product smoke fixtures covering route parity, auth/session, background task logs, downloads, CRUD state, report artifacts, and the checked-in product workflow skeleton contracts. | `tests/llm/llm_web_product_smoke_test.go` runs `examples/ai/finrobot_translation/web_product.leia`; `tests/llm/finrobot_product_workflow_live_package_test.go` validates `live_packages/product_workflow`. |
| Finance vendors as built-ins | Not needed. Vendors belong in external packages with capabilities, credentials, schemas, rate-limit metadata, and terms metadata. | Vendor package manifests declare capabilities and replay fixtures. |
| Role profiles as language concepts | Not needed. Roles are package data consumed by generic `agent` declarations. | Role registry snapshot is data-only and uses no parser changes. |
| FR-GAP-021 deployment/package metadata | Implemented as provider-free package/deploy metadata outside the core dialect: requirements, setup extras, Dockerfile, gcloud deploy script, run_web_app health entrypoint, manifest commands, and environment checks. | `tests/llm/finrobot_package_deploy_test.go` asserts package manifest smoke, optional extras, Docker/gcloud/run_web_app references, and environment setup checks. |
| FR-GAP-022 compliance and safety gates | Implemented as explicit capability-policy gates for trading, generated code execution, external network calls, credentials, and report publication. | `tests/llm/llm_approval_policy_test.go` runs `examples/ai/finrobot_translation/compliance_policy.leia`. |
| FR-GAP-023 evaluation harness | Implemented as offline records inventory, fixture versioning, checksums, golden outputs, generic AI evaluation dialect specimen, judge spec, metric registry, dataset manifest, strict replay matching, scoring trace, failure envelope, and golden threshold gates. | `tests/llm/finrobot_evaluation_harness_test.go` asserts manifest checksums and runs `leia evaluate --gate --report --replay`; `tests/llm/finrobot_evaluation_harness_parity_test.go` validates provider-free generic evaluation parity and replay mismatch envelopes. |
| FR-GAP-024 product UI snapshots | Implemented as static/template snapshot manifests and accessibility checklist fixtures for the translation slice. | `tests/llm/llm_web_product_smoke_test.go` validates the web product snapshot metadata. |

## First Landing Slice

1. Define a provider-free fixture set for one equity-report run: recorded FMP
   statements/metrics/news, recorded model outputs for section agents, and
   golden report artifacts.
2. Translate the core flow as docs/examples only: config, data fetch tools,
   normalization, valuation, section agents, chart/report generation, and
   replay/evaluate records.
3. Keep every provider, valuation formula, prompt, chart theme, and report
   template in packages or package data. The language substrate should only
   provide composition, capabilities, tracing, and replay.
