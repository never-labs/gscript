# FinRobot Live External Package Plan

This plan keeps the remaining live FinRobot work out of the replay slice and
targets external package implementations. The replay examples now include
provider-free skeleton contracts; live vendors, finance formulas, renderers, and
web services are not Leia built-ins.

## Package Skeleton Status

Current registered-example status: 99 runnable/checkable FinRobot translation
examples are discovered by `go run ./cmd/leia examples list --json`. All 53
checked-in live-package skeletons are registered provider-free examples. The
FinRobot package skeletons cover finance, reporting, product, and optional
integration boundaries; the `generic_*` skeletons cover reusable generic AI
model, model IO, memory, turn, tool, coding workspace, data provider boundaries, agent, workflow,
evaluation, replay, trace, approval, document RAG, prompt/role catalogs,
event intelligence boundaries, strategy simulation boundaries, evidence/report artifacts, report render contracts, UI snapshot evaluation, chart render contracts,
optional adapter boundaries, product app boundaries, and package-audit boundaries
consumed by the translation.

| Checked-in skeleton | Registered example | Target external package | Target directory | Contract |
| --- | --- | --- | --- | --- |
| `live_packages/vendor_adapters` | `live_packages/vendor_adapters/main.leia` | `leia-finrobot-vendor-adapters` | `packages/finrobot/vendor_adapters` | `contracts/vendor_adapter_contract.json` |
| `live_packages/factor_research` | `live_packages/factor_research/main.leia` | `leia-finrobot-factor-research` | `packages/finrobot/factor_research` | `contracts/factor_research_contract.json` |
| `live_packages/analytics_report` | `live_packages/analytics_report/analytics_report.leia` | `leia-finrobot-analytics-report` | `packages/finrobot/analytics_report` | `contracts/analytics_report_contract.json` |
| `live_packages/finance_normalizers` | `live_packages/finance_normalizers/main.leia` | `leia-finrobot-finance-normalizers` | `packages/finrobot/finance_normalizers` | `contracts/finance_normalizers_contract.json` |
| `live_packages/valuation_engine` | `live_packages/valuation_engine/main.leia` | `leia-finrobot-valuation-engine` | `packages/finrobot/valuation_engine` | `contracts/valuation_engine_contract.json` |
| `live_packages/report_renderer` | `live_packages/report_renderer/main.leia` | `leia-finrobot-report-renderer` | `packages/finrobot/report_renderer` | `contracts/report_renderer_contract.json` |
| `live_packages/product_workflow` | `live_packages/product_workflow/main.leia` | `leia-finrobot-web-product` | `packages/finrobot/web_product` | `contracts/web_product_contract.json` |
| `live_packages/optional_integrations` | `live_packages/optional_integrations/main.leia` | `leia-finrobot-optional-integrations` | `packages/finrobot/optional_integrations` | `contracts/optional_integration_capability_gates.json` |
| `live_packages/prompt_roles` | `live_packages/prompt_roles/main.leia` | `leia-finrobot-prompt-roles` | `packages/finrobot/prompt_roles` | `contracts/prompt_role_contract.json` |
| `live_packages/news_catalyst` | `live_packages/news_catalyst/main.leia` | `leia-finrobot-news-catalyst` | `packages/finrobot/news_catalyst` | `contracts/news_catalyst_contract.json` |
| `live_packages/finance_facade` | `live_packages/finance_facade/main.leia` | `leia-finrobot-finance-facade` | `packages/finrobot/finance_facade` | `contracts/finance_facade_contract.json` |
| `live_packages/document_pipeline` | `live_packages/document_pipeline/main.leia` | `leia-finrobot-document-pipeline` | `packages/finrobot/document_pipeline` | `contracts/document_pipeline_contract.json` |
| `live_packages/analyzer_report` | `live_packages/analyzer_report/main.leia` | `leia-finrobot-analyzer-report` | `packages/finrobot/analyzer_report` | `contracts/analyzer_report_contract.json` |
| `live_packages/coding_notebook` | `live_packages/coding_notebook/main.leia` | `leia-finrobot-coding-notebook` | `packages/finrobot/coding_notebook` | `contracts/coding_notebook_contract.json` |
| `live_packages/tutorial_demo_parity` | `live_packages/tutorial_demo_parity/main.leia` | `leia-finrobot-tutorial-demo-parity` | `packages/finrobot/tutorial_demo_parity` | `contracts/tutorial_demo_parity_contract.json` |
| `live_packages/chart_renderer` | `live_packages/chart_renderer/main.leia` | `leia-finrobot-chart-renderer` | `packages/finrobot/chart_renderer` | `contracts/chart_renderer_contract.json` |
| `live_packages/backtest_strategy` | `live_packages/backtest_strategy/main.leia` | `leia-finrobot-backtest-strategy` | `packages/finrobot/backtest_strategy` | `contracts/backtest_strategy_contract.json` |
| `live_packages/equity_analysis_pipeline` | `live_packages/equity_analysis_pipeline/main.leia` | `leia-finrobot-equity-analysis-pipeline` | `packages/finrobot/equity_analysis_pipeline` | `contracts/stage_dag_contract.json` |
| `live_packages/retail_sentiment` | `live_packages/retail_sentiment/main.leia` | `leia-finrobot-retail-sentiment` | `packages/finrobot/retail_sentiment` | `contracts/retail_sentiment_contract.json` |
| `live_packages/html_ui_snapshots` | `live_packages/html_ui_snapshots/main.leia` | `leia-finrobot-html-ui-snapshots` | `packages/finrobot/html_ui_snapshots` | `contracts/html_ui_snapshot_contract.json` |
| `live_packages/earnings_transcript` | `live_packages/earnings_transcript/main.leia` | `leia-finrobot-earnings-transcript` | `packages/finrobot/earnings_transcript` | `contracts/earnings_transcript_contract.json` |
| `live_packages/sec_filings` | `live_packages/sec_filings/main.leia` | `leia-finrobot-sec-filings` | `packages/finrobot/sec_filings` | `contracts/sec_filings_contract.json` |
| `live_packages/generic_model_registry` | `live_packages/generic_model_registry/main.leia` | `leia-generic-ai-model-registry` | `packages/generic_ai/model_registry` | `contracts/model_registry_contract.json` |
| `live_packages/generic_analytical_model_contracts` | `live_packages/generic_analytical_model_contracts/main.leia` | `leia-generic-ai-analytical-model-contracts` | `packages/generic_ai/analytical_model_contracts` | `contracts/generic_analytical_model_contracts_contract.json` |
| `live_packages/generic_optional_adapter_boundary` | `live_packages/generic_optional_adapter_boundary/main.leia` | `leia-generic-ai-optional-adapter-boundary` | `packages/generic_ai/optional_adapter_boundary` | `contracts/generic_optional_adapter_boundary_contract.json` |
| `live_packages/generic_model_io_envelope` | `live_packages/generic_model_io_envelope/main.leia` | `leia-generic-ai-model-io-envelope` | `packages/generic_ai/model_io_envelope` | `contracts/model_io_envelope_contract.json` |
| `live_packages/generic_turn_runner` | `live_packages/generic_turn_runner/main.leia` | `leia-generic-ai-turn-runner` | `packages/generic_ai/turn_runner` | `contracts/generic_turn_runner_contract.json` |
| `live_packages/generic_tool_contracts` | `live_packages/generic_tool_contracts/main.leia` | `leia-generic-ai-tool-contracts` | `packages/generic_ai/tool_contracts` | `contracts/generic_tool_contract.json`; `fixtures/registry_descriptor_to_tool_contract_projection_fixture.json` |
| `live_packages/generic_tool_registry` | `live_packages/generic_tool_registry/main.leia` | `leia-generic-ai-tool-registry` | `packages/generic_ai/tool_registry` | `contracts/tool_registry_contract.json` |
| `live_packages/generic_coding_workspace` | `live_packages/generic_coding_workspace/main.leia` | `leia-generic-ai-coding-workspace` | `packages/generic_ai/coding_workspace` | `contracts/generic_coding_workspace_contract.json` |
| `live_packages/generic_data_normalization_contracts` | `live_packages/generic_data_normalization_contracts/main.leia` | `leia-generic-ai-data-normalization-contracts` | `packages/generic_ai/data_normalization_contracts` | `contracts/generic_data_normalization_contracts_contract.json`; `fixtures/provider_response_projection_fixture.json` |
| `live_packages/generic_data_provider_boundary` | `live_packages/generic_data_provider_boundary/main.leia` | `leia-generic-ai-data-provider-boundary` | `packages/generic_ai/data_provider_boundary` | `contracts/generic_data_provider_boundary_contract.json` |
| `live_packages/generic_agent_runner` | `live_packages/generic_agent_runner/main.leia` | `leia-generic-ai-agent-runner` | `packages/generic_ai/agent_runner` | `contracts/agent_runner_contract.json` |
| `live_packages/generic_planning_graph` | `live_packages/generic_planning_graph/main.leia` | `leia-generic-ai-planning-graph` | `packages/generic_ai/planning_graph` | `contracts/planning_graph_contract.json` |
| `live_packages/generic_product_app_boundary` | `live_packages/generic_product_app_boundary/main.leia` | `leia-generic-ai-product-app-boundary` | `packages/generic_ai/product_app_boundary` | `contracts/generic_product_app_boundary_contract.json` |
| `live_packages/generic_prompt_role_catalog` | `live_packages/generic_prompt_role_catalog/main.leia` | `leia-generic-ai-prompt-role-catalog` | `packages/generic_ai/prompt_role_catalog` | `contracts/generic_prompt_role_catalog_contract.json` |
| `live_packages/generic_report_render_contracts` | `live_packages/generic_report_render_contracts/main.leia` | `leia-generic-ai-report-render-contracts` | `packages/generic_ai/report_render_contracts` | `contracts/generic_report_render_contracts_contract.json` |
| `live_packages/generic_evidence_report_artifacts` | `live_packages/generic_evidence_report_artifacts/main.leia` | `leia-generic-ai-evidence-report-artifacts` | `packages/generic_ai/evidence_report_artifacts` | `contracts/generic_evidence_report_artifacts_contract.json` |
| `live_packages/generic_evidence_verification` | `live_packages/generic_evidence_verification/main.leia` | `leia-generic-ai-evidence-verification` | `packages/generic_ai/evidence_verification` | `contracts/generic_evidence_verification_contract.json`; `fixtures/document_rag_evidence_projection_fixture.json` |
| `live_packages/generic_workflow_orchestrator` | `live_packages/generic_workflow_orchestrator/main.leia` | `leia-generic-ai-workflow-orchestrator` | `packages/generic_ai/workflow_orchestrator` | `contracts/workflow_graph_contract.json`; `fixtures/planning_graph_stage_projection_fixture.json` |
| `live_packages/generic_evaluation_harness` | `live_packages/generic_evaluation_harness/main.leia` | `leia-generic-ai-evaluation-harness` | `packages/generic_ai/evaluation_harness` | `contracts/evaluation_harness_contract.json` |
| `live_packages/generic_memory_store` | `live_packages/generic_memory_store/main.leia` | `leia-generic-ai-memory-store` | `packages/generic_ai/memory_store` | `contracts/generic_memory_store_contract.json` |
| `live_packages/generic_record_replay` | `live_packages/generic_record_replay/main.leia` | `leia-generic-ai-record-replay` | `packages/generic_ai/record_replay` | `contracts/record_replay_contract.json` |
| `live_packages/generic_trace_events` | `live_packages/generic_trace_events/main.leia` | `leia-generic-ai-trace-events` | `packages/generic_ai/trace_events` | `contracts/trace_events_contract.json` |
| `live_packages/generic_approval_policy` | `live_packages/generic_approval_policy/main.leia` | `leia-generic-ai-approval-policy` | `packages/generic_ai/approval_policy` | `contracts/generic_approval_policy_contract.json` |
| `live_packages/generic_chart_render_contracts` | `live_packages/generic_chart_render_contracts/main.leia` | `leia-generic-ai-chart-render-contracts` | `packages/generic_ai/chart_render_contracts` | `contracts/generic_chart_render_contracts_contract.json` |
| `live_packages/generic_document_rag_pipeline` | `live_packages/generic_document_rag_pipeline/main.leia` | `leia-generic-ai-document-rag-pipeline` | `packages/generic_ai/document_rag_pipeline` | `contracts/generic_document_rag_pipeline_contract.json` |
| `live_packages/generic_event_intelligence_boundary` | `live_packages/generic_event_intelligence_boundary/main.leia` | `leia-generic-ai-event-intelligence-boundary` | `packages/generic_ai/event_intelligence_boundary` | `contracts/generic_event_intelligence_boundary_contract.json` |
| `live_packages/generic_strategy_backtest_contracts` | `live_packages/generic_strategy_backtest_contracts/main.leia` | `leia-generic-ai-strategy-backtest-contracts` | `packages/generic_ai/strategy_backtest_contracts` | `contracts/generic_strategy_backtest_contracts_contract.json` |
| `live_packages/generic_transcript_pipeline` | `live_packages/generic_transcript_pipeline/main.leia` | `leia-generic-ai-transcript-pipeline` | `packages/generic_ai/transcript_pipeline` | `contracts/generic_transcript_pipeline_contract.json` |
| `live_packages/generic_ui_snapshot_evaluator` | `live_packages/generic_ui_snapshot_evaluator/main.leia` | `leia-generic-ai-ui-snapshot-evaluator` | `packages/generic_ai/ui_snapshot_evaluator` | `contracts/generic_ui_snapshot_evaluator_contract.json` |
| `live_packages/generic_package_boundary_auditor` | `live_packages/generic_package_boundary_auditor/main.leia` | `leia-generic-ai-package-boundary-auditor` | `packages/generic_ai/package_boundary_auditor` | `contracts/package_boundary_audit_contract.json` |

## Contract Rules

- Every target package manifest must name its package, target directory,
  contract file, capabilities, optional credentials, replay fixture keys, and
  test gates.
- Live network and real dependency imports default to disabled. Tests for this
  plan only validate manifests and replay contracts; they must not call real providers.
- Capabilities are the boundary between the replay slice and live packages.
  Provider adapters, normalizers, valuation logic, report rendering, and web
  product behavior must be granted explicitly before replacing a replay fixture.
- Optional provider credentials must be absent-safe and redacted. Missing live
  credentials produce clean skips, not failing imports or hidden fallback calls.
- Leia core guarantees generic composition, capability contracts, trace/replay
  metadata, approval policy, and provider-free gates only. It does not guarantee
  built-in finance vendors, valuation formulas, report renderers, or product UI.

## Test Gates

The machine-readable gate list lives in
`live_package_plan_manifest.json`. It is intentionally separate from live
package implementations so this repository can keep testing the migration plan
without shipping live clients or performing real network I/O.
