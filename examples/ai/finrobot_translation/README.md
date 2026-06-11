# FinRobot Translation Ledger

This directory tracks the Leia translation of FinRobot as an external
finance-agent workload. The checked-in slice is provider-free: examples,
fixtures, manifests, and tests exercise the generic Leia AI/runtime surfaces
without live finance vendors or committed credentials.

Source basis: a local `.external/FinRobot` checkout was used for the original
audit, including the top-level README, `finrobot`, `finrobot_equity`,
experiments, tutorials, deployment scripts, and tests. The current repository
evidence for the translated slice is in this directory and the `tests/llm`
FinRobot tests.

Coverage audit: see `COVERAGE.md` for the module matrix, registered-example
inventory, provider-free slice status, and live-package follow-up tasks.
Verification gates are listed in `VERIFICATION.md`. Production live-package
work is tracked separately in `live_package_plan.md` and
`live_package_plan_manifest.json`.

## Scope

FinRobot should translate into ordinary Leia packages that compose existing
general surfaces plus external package implementations. The current directory
now includes provider-free skeleton examples for the live-package boundary; the
remaining work is live implementation outside the core dialect.

- `model`, `turn`, `agent`, `tool`, `workflow`, `approve`, `test`, `evaluate`,
  record/replay, trace, and capability declarations;
- `api`, `web`, file, config, secret, JSON/CSV/table, document, chart, report,
  and database packages;
- finance libraries for vendor adapters, statement normalization, valuation,
  forecasting, portfolio methods, backtesting, prompts, report templates, and
  product workflows.

Non-goal: no `finrobot`, `finance`, `ticker`, `sec`, `trading`, valuation, or
report-section syntax should be added to Leia.

## Source Inventory Status Key

The module inventory below describes source-to-Leia translation targets and the
remaining productionization shape for a faithful live package. It is not the
provider-free CI gap ledger; the current provider-free slice has no open
language or AI-dialect gaps. See `GAPS.md` for the closed FR-GAP ledger and
`COVERAGE.md` for executable evidence.

- `Mapped`: reviewed and mapped to a Leia target, with no runtime edit in this
  branch.
- `Partial`: a generic Leia surface or provider-free skeleton exists, but live
  package APIs, broader parity, or production behavior are incomplete.
- `Open`: no direct provider-free skeleton exists yet, or the row requires
  package/product implementation before a faithful live translation can run.

Gap categories:

- `language`: parser or core language surface.
- `AI dialect`: model/turn/tool/agent/workflow/evaluate/replay contracts.
- `stdlib`: generic data, file, web, API, document, chart, report, database, or
  config libraries.
- `external library`: finance/vendor/model/reporting adapters outside core.
- `product layer`: deployment, auth, UI, compliance, operations, or package
  assembly.

## Module Translation Inventory

| FinRobot module | Leia translation target | Status | Main gap categories | Priority | Validation |
| --- | --- | --- | --- | --- | --- |
| `README.md`, architecture, Pro pipeline | Workload spec plus package examples that compose AI, data, workflow, report, and web surfaces | Mapped | AI dialect, stdlib, external library, product layer | P0 | Audit checklist links every README pipeline step to a module row and gap row |
| `finrobot/agents/agent_library.py` | Role-profile package data used by generic `agent` declarations | Partial | AI dialect, external library | P1 | Golden role registry snapshot; each role has name, system profile, tools, and termination policy |
| `finrobot/agents/workflow.py` | `agent` plus `workflow` flows for single assistant, RAG assistant, shadow assistant, group chat, and leader-directed delegation | Partial | AI dialect, stdlib | P0 | Provider-free replay fixtures for each workflow class, with parent/child trace assertions |
| `finrobot/agents/prompts.py`, `utils.py` | Prompt templates and helper package data | Partial | AI dialect, external library | P1 | Snapshot tests for rendered prompts and delegation trigger parsing |
| `finrobot/toolkits.py` | Typed `tool` registration with schemas, capabilities, structured errors, and string/table output adapters | Partial | AI dialect, stdlib | P0 | Tool inventory test validates schema, capability, replay key, and result envelope for every tool |
| `finrobot/utils.py`, package init | Config/model alias helpers and package bootstrap | Partial | AI dialect, stdlib | P1 | Config fixture resolves model aliases and secrets without checked-in credentials |
| `finrobot/data_source/yfinance_utils.py` | Yahoo Finance client package over `api`/`web` plus typed table outputs | Partial | stdlib, external library | P0 | Offline fixtures and vendor-adapter skeleton cover price history, company info, dividends, statements, cash flow, recommendations |
| `finrobot/data_source/finnhub_utils.py` | Finnhub client package with typed JSON decode, rate-limit metadata, and news/metrics schemas | Partial | stdlib, external library | P0 | Recorded API fixtures and vendor-adapter skeleton validate profile, news, historical metrics, and basic financials |
| `finrobot/data_source/fmp_utils.py` | Financial Modeling Prep client package for targets, SEC metadata, market cap, BVPS, metrics, competitors | Partial | stdlib, external library | P0 | Recorded fixtures and vendor-adapter skeleton cover pagination, errors, statement schemas, and competitor metrics |
| `finrobot/data_source/sec_utils.py` | SEC client/document package for 10-K metadata, downloads, PDF conversion, and section extraction | Partial | stdlib, external library | P0 | Fixture-backed download, document/RAG skeleton, and section-extraction tests over known 10-K filings |
| `finrobot/data_source/finnlp_utils.py`, `FinNLP/` | Optional FinNLP adapter package for news/social datasets | Partial | stdlib, external library | P2 | Optional integration skeleton records dataset gates and schema normalization expectations |
| `finrobot/data_source/reddit_utils.py` | Reddit sentiment/source adapter with explicit capability and terms metadata | Partial | stdlib, external library, product layer | P2 | Recorded fixture and vendor-adapter skeleton validate auth, pagination, post schema, terms metadata, and redaction |
| `finrobot/data_source/finance_data.py` | Generic finance data facade over provider adapters | Partial | stdlib, external library | P1 | Contract fixtures ensure facade-shaped typed tables with provenance; live dispatch remains package work |
| `finrobot/data_source/earnings_calls_src/*` | Earnings-call transcript package and RAG corpus builder | Partial | stdlib, external library | P1 | Fixture transcript retrieval, chunking, metadata, and replayable RAG lookup skeleton |
| `finrobot/data_source/filings_src/*` | SEC filing search/fetch/section parser package | Partial | stdlib, external library | P0 | Golden 10-K section fixtures, redirect handling, HTML/PDF parsing boundary, and traceable errors |
| `finrobot/data_source/marker_sec_src/*` | PDF-to-markdown and SEC filing conversion tools | Partial | stdlib, external library | P1 | Local PDF artifact skeleton verifies deterministic markdown chunk expectations and provenance |
| `finrobot/functional/rag.py`, `ragquery.py` | Generic RAG helpers over files, document chunks, vector stores, and retrieval providers | Partial | AI dialect, stdlib, external library | P0 | Replay tests for SEC and earnings-call RAG without live model or network calls |
| `finrobot/functional/analyzer.py` | Finance analysis prompt library and report-section analyzers | Open | AI dialect, external library | P1 | Section golden tests check required evidence, source citations, and output schemas |
| `finrobot/functional/charting.py` | Chart package examples for stock, share performance, P/E, EPS, and report charts | Partial | stdlib, external library | P1 | Report/chart skeleton validates artifact metadata and deterministic fixture data; image rendering remains package work |
| `finrobot/functional/coding.py` | Capability-gated file, Python execution, and image-display tools | Partial | AI dialect, stdlib, product layer | P1 | Approval/replay tests for file edits, code execution, denied commands, and image artifacts |
| `finrobot/functional/quantitative.py` | Backtrader/strategy package wrapper | Partial | stdlib, external library | P2 | Backtest fixture skeleton validates inputs, trades, metrics, and deterministic seed behavior |
| `finrobot/functional/reportlab.py` | Document/report package for annual-report PDF assembly | Partial | stdlib, external library | P1 | Generated report skeleton validates sections, assets, provenance, and disclosure markers |
| `finrobot/functional/text.py` | Generic text utility package | Mapped | stdlib | P2 | Unit tests for length checks and report text constraints |
| `experiments/investment_group.py` | Multi-agent investment workflow example | Partial | AI dialect, external library | P2 | Offline replay of group roles, tool calls, and final recommendation schema |
| `experiments/multi_factor_agents.py` | Multi-factor research workflow and data package example | Open | AI dialect, stdlib, external library | P2 | Fixture data validates factor transforms and agent outputs |
| `experiments/portfolio_optimization.py` | Portfolio optimization package example | Partial | stdlib, external library | P2 | Deterministic optimizer fixture validates constraints, weights, and risk metrics |
| `tutorials_beginner/*` | Leia example suite for annual reports, FinGPT forecast, RAG QA, Ollama function calling, stock charts | Open | AI dialect, stdlib, external library | P1 | Each tutorial gets a provider-free replay record and optional live-provider gate |
| `tutorials_advanced/*` | Advanced Leia examples for trade strategy, OpenBB, multimodal chart/document analysis, mplfinance, SMA crossover | Open | AI dialect, stdlib, external library | P2 | Notebook-to-Leia parity checklist plus recorded fixtures for external calls |
| `agent_builder_demo.py`, `test_module.py` | Smoke/demo examples and regression fixtures | Open | AI dialect, stdlib | P2 | Convert to small executable Leia examples with deterministic assertions |
| `configs/*`, `OAI_CONFIG_LIST`, `config_api_keys` | Config, model alias, and secret mapping examples | Partial | AI dialect, stdlib, product layer | P0 | Secret-free config fixture checks env lookup, missing-key diagnostics, and trace metadata |
| `requirements*.txt`, `setup.py`, `Dockerfile`, `deploy*.sh`, `run_web_app.py` | Package/deploy metadata and local service entry points | Partial | product layer, external library | P2 | Packaging smoke test and deployment metadata exist; release packaging and cloud smoke remain external package work |
| `finrobot_equity/README.md` | Equity research product workload spec | Mapped | AI dialect, stdlib, external library, product layer | P0 | Trace every product pipeline step to analysis, text, chart, report, and web rows |
| `finrobot_equity/core/src/generate_financial_analysis.py` | CLI workflow for fetching, processing, forecasting, agent text, and artifacts | Open | AI dialect, stdlib, external library | P0 | End-to-end fixture run with recorded data and model outputs |
| `finrobot_equity/core/src/create_equity_report.py`, `generate_pdf_report.py` | Report generation workflows over CSVs, charts, text sections, and PDF/HTML outputs | Partial | stdlib, external library, product layer | P0 | Golden HTML/PDF artifact skeletons with stale-date and required-section checks; real rendering remains package work |
| `finrobot_equity/core/src/modules/market_data_api.py` | FMP/YFinance market data package | Partial | stdlib, external library | P0 | Recorded fixtures and vendor-adapter skeleton cover statements, EV, ratios, target price, rating, profile, news, indicators |
| `finrobot_equity/core/src/modules/financial_data_processor.py` | Typed table normalization, historical metrics, and forecast transforms | Partial | stdlib, external library | P0 | Unit fixtures cover numeric cleaning, statement extraction, forecast formulas, and missing data |
| `finrobot_equity/core/src/modules/valuation_engine.py` | DCF, EV/EBITDA, peer comparison, synthesis, and football-field data package | Partial | stdlib, external library | P1 | Deterministic valuation skeleton covers fixtures with sensitivity to assumptions |
| `finrobot_equity/core/src/modules/sensitivity_analyzer.py` | Sensitivity and confidence interval package | Partial | stdlib, external library | P1 | Matrix/table fixtures for revenue and margin ranges |
| `finrobot_equity/core/src/modules/catalyst_analyzer.py`, `news_integrator.py` | News/catalyst classification and summary package with optional AI sections | Open | AI dialect, stdlib, external library | P1 | Fixture news set validates relevance, category, sentiment, impact, and generated summaries |
| `finrobot_equity/core/src/modules/retail_sentiment_client.py` | Adanos/retail sentiment adapter over Reddit, X.com, and Polymarket snapshots | Partial | stdlib, external library, product layer | P2 | Recorded snapshot fixtures and vendor-adapter skeleton validate source normalization and prompt formatting |
| `finrobot_equity/core/src/modules/text_generator_agents.py`, `enhanced_text_generator.py`, `equity_agents/*` | Section-specific AI agents for thesis, risks, valuation, news, company overview, and takeaways | Partial | AI dialect, external library | P0 | Output-schema tests and replay records for each section agent |
| `finrobot_equity/core/src/modules/chart_generator.py`, `enhanced_chart_generator.py` | Chart/report artifact package | Partial | stdlib, external library | P1 | Fixture chart skeletons validate dimensions, source metadata, and report references |
| `finrobot_equity/core/src/modules/html_renderer.py`, `html_template_professional.py` | HTML report templating package | Open | stdlib, product layer | P1 | Golden HTML snapshots with required sections, tables, charts, disclaimers, and accessibility checks |
| `finrobot_equity/core/src/modules/pdf_generator.py`, `professional_pdf_report.py` | PDF report package | Partial | stdlib, external library, product layer | P1 | PDF artifact skeleton records page-level contract expectations; rendered PDF output remains package work |
| `finrobot_equity/core/src/modules/report_structure.py`, `report_data_loader.py`, `common_utils.py` | Report schemas, source annotations, AI disclosure, data loading, and config utilities | Partial | stdlib, product layer | P0 | Schema tests for ordered sections, provenance, disclosure, and config resolution |
| `finrobot_equity/core/tests/*` | Existing Python tests as source for Leia fixtures | Mapped | AI dialect, stdlib | P1 | Port representative assertions to Leia `test`/`evaluate` fixtures |
| `finrobot_equity/web_app/main.py`, `admin_routes.py`, `auth.py` | Product web app over workflows, auth, history, logs, and report downloads | Open | stdlib, product layer | P2 | Web route smoke tests with mocked workflow process and auth sessions |
| `finrobot_equity/web_app/database/*` | SQLite/SQLAlchemy schema and CRUD product package | Open | stdlib, product layer | P2 | DB migration/CRUD fixtures for users, sessions, requests, and reports |
| `finrobot_equity/web_app/templates/*`, `static/*` | Product UI assets and templates | Open | product layer | P3 | Snapshot/accessibility checks; outside AI dialect translation critical path |

## Priority Summary

- P0: establish executable translation substrate for agent workflows, typed
  tools, provider adapters, config/secrets, RAG, financial data normalization,
  and the equity-report pipeline.
- P1: port section agents, valuation/report/chart/document capabilities, and
  replay-backed examples.
- P2: cover optional provider adapters, experiments, advanced tutorials,
  deployment, and web product flows.
- P3: product UI assets that are useful for parity but not needed to validate
  the language and package mapping.

See `GAPS.md` for the closed gap ledger and the remaining live-package
implementation boundary.
