# FinRobot Translation Gap Audit

## Goal

This audit uses FinRobot as a finance-agent workload test for Leia's general AI,
data, web, workflow, and reporting surfaces.

It should not produce a `finrobot` dialect. FinRobot-like systems should be
expressible as ordinary Leia packages that compose existing general dialects,
standard-library modules, and domain libraries.

Sources reviewed:

- FinRobot repository README and source layout:
  https://github.com/AI4Finance-Foundation/FinRobot
- FinRobot paper, arXiv 2405.14767:
  https://arxiv.org/abs/2405.14767
- Local audit source: `.external/FinRobot`, including `finrobot`,
  `finrobot_equity`, experiments, tutorials, deployment scripts, and tests.

Detailed working ledgers live in:

- `examples/ai/finrobot_translation/README.md`
- `examples/ai/finrobot_translation/GAPS.md`

## FinRobot Feature Inventory

Platform architecture:

- financial AI agents layer;
- financial LLM algorithms layer;
- LLMOps layer;
- DataOps layer;
- multi-source foundation-model layer;
- perception, brain, and action agent loop;
- Financial Chain-of-Thought prompting;
- smart scheduler for model and agent selection;
- plug-and-play general and finance-specialized LLMs.

Agent workflows:

- single assistant workflow;
- single assistant with RAG tool;
- shadow assistant for nested instruction refinement;
- multi-agent group chat;
- leader-directed multi-agent workflow;
- role library for software developer, data analyst, programmer, accountant,
  statistician, IT specialist, AI engineer, financial analyst, market analyst,
  and expert investor;
- tool registration from functions, dict configs, and classes;
- user proxy execution for tool calls and optional code execution;
- disk cache for repeat chat runs;
- termination-message based run control;
- human-input modes for interactive and non-interactive execution.

Financial data sources:

- Yahoo Finance stock price history;
- Yahoo Finance company profile, dividends, financial statements, cash flow,
  balance sheet, and analyst recommendations;
- Finnhub company profile, company news, and basic financial metrics;
- Financial Modeling Prep target prices, SEC report metadata, historical market
  capitalization, historical book value per share, financial metrics, and
  competitor metrics;
- SEC filing search, filing download, PDF generation, and 10-K section
  extraction;
- earnings-call transcript retrieval;
- SEC filing conversion from HTML/PDF to markdown;
- optional retail sentiment snapshots from Reddit, X.com, and Polymarket in the
  equity module.

Analysis and modeling:

- market forecaster agent for short-term stock-movement direction;
- annual-report and equity-research analysis;
- income statement analysis;
- balance sheet analysis;
- cash flow analysis;
- segment analysis;
- risk factor assessment;
- competitor analysis;
- business highlights and company-description analysis;
- key data extraction;
- financial metrics extraction from APIs and PDFs;
- multi-year growth and forecast generation;
- peer comparison;
- DCF-style valuation and valuation overview;
- P/E and EV/EBITDA multiples;
- sensitivity analysis;
- catalyst detection and impact assessment;
- news relevance, category, importance, sentiment, and summary analysis;
- technical indicators;
- portfolio optimization and multi-factor experiments;
- backtesting through Backtrader utilities;
- FinRL-style trading and portfolio allocation as a referenced algorithm layer;
- FinGPT-style financial language-model use as a referenced model layer;
- multimodal chart and document analysis demonstrations.

RAG and document handling:

- RAG function generation over configured retrieval stores;
- RAG over SEC filings;
- RAG over earnings-call transcripts;
- annual-report PDF construction;
- SEC filing section parsing;
- PDF-to-markdown conversion;
- HTML report rendering;
- professional PDF report generation;
- report structure validation, table of contents, data-source annotations, and
  AI-content disclosure.

Charts and presentation outputs:

- stock price charts;
- share performance charts;
- P/E and EPS performance charts;
- revenue and EBITDA charts;
- EV/EBITDA peer comparison charts;
- margin trend charts;
- revenue breakdown pie charts;
- financial radar charts;
- time series charts;
- sensitivity heatmaps;
- technical-indicator charts;
- valuation waterfall charts;
- quarterly comparison charts;
- cash flow charts;
- HTML report templates;
- PDF report templates.

Product and operations surface:

- one-command local web-app deployment;
- CLI pipeline for analysis and report creation;
- FastAPI web interface;
- local and GitHub OAuth authentication;
- admin endpoints;
- SQLAlchemy and SQLite persistence;
- request logging;
- Jinja2 templates and static assets;
- config-file and environment-variable API key management;
- unit tests for core equity modules.

Coding and notebook tools:

- list directory;
- inspect file;
- modify code;
- create file with code;
- execute Python cells in IPython;
- display generated images;
- beginner and advanced Jupyter tutorials.

## Full Module Ledger

The full module-by-module translation ledger is maintained in
`examples/ai/finrobot_translation/README.md`. The ledger records, for every
reviewed FinRobot module, its Leia translation target, current completion
status, gap categories, priority, and validation method.

Audit coverage:

| Area | Reviewed modules | Leia target | Current status | Top gap classes |
| --- | --- | --- | --- | --- |
| Core agent roles and workflows | `finrobot/agents/agent_library.py`, `workflow.py`, `prompts.py`, `utils.py` | Role data, generic `agent`, agent-as-tool composition, `workflow`, trace, replay, approval gates | Partial | AI dialect, stdlib |
| Tool registration | `finrobot/toolkits.py` plus function/class toolkit configs | Typed `tool` declarations with schemas, capabilities, structured errors, output adapters, replay keys | Partial | AI dialect, stdlib |
| Market and filing data sources | `yfinance_utils.py`, `finnhub_utils.py`, `fmp_utils.py`, `sec_utils.py`, `finance_data.py`, `filings_src/*` | External provider packages over `api`, `web`, JSON/CSV/table, file, document, and provenance APIs | Open | stdlib, external library |
| Text, RAG, coding, charting, quantitative, and report helpers | `finrobot/functional/*` | Generic RAG/document/chart/report/code-exec packages plus finance prompt/report assets | Partial/Open | AI dialect, stdlib, external library, product layer |
| Experiments and tutorials | `experiments/*`, `tutorials_beginner/*`, `tutorials_advanced/*`, demos/tests | Replay-backed Leia examples with optional live-provider gates | Open | AI dialect, stdlib, external library |
| Config, packaging, and deployment | `configs/*`, `OAI_CONFIG_LIST`, `config_api_keys`, `requirements*.txt`, `setup.py`, `Dockerfile`, `deploy*.sh`, `run_web_app.py` | Config/env/secret examples, package metadata, optional deployment/product docs | Partial/Open | AI dialect, stdlib, product layer |
| Equity product pipeline | `finrobot_equity/README.md`, `core/src/generate_financial_analysis.py`, `create_equity_report.py`, `generate_pdf_report.py` | Named workflow stages for fetch, normalize, forecast, AI section generation, chart/report artifacts, evaluate/replay | Open | AI dialect, stdlib, external library, product layer |
| Equity analytics modules | `market_data_api.py`, `financial_data_processor.py`, `valuation_engine.py`, `sensitivity_analyzer.py`, `catalyst_analyzer.py`, `news_integrator.py`, `retail_sentiment_client.py` | Finance data, valuation, sensitivity, catalyst, sentiment, news, and vendor adapter packages | Open | stdlib, external library |
| Equity AI section agents | `text_generator_agents.py`, `enhanced_text_generator.py`, `equity_agents/*` | Section-specific `agent` package data with output schemas and replay records | Partial | AI dialect, external library |
| Equity charts and reports | `chart_generator.py`, `enhanced_chart_generator.py`, `html_renderer.py`, `html_template_professional.py`, `pdf_generator.py`, `professional_pdf_report.py`, `report_structure.py`, `report_data_loader.py`, `common_utils.py` | Chart/report/document packages with provenance, required sections, stale-data checks, and AI disclosure markers | Open/Partial | stdlib, external library, product layer |
| Equity web app | `finrobot_equity/web_app/*` | Product package for FastAPI routes, auth, admin, logs, SQLite CRUD, report history/downloads, templates/static assets | Open | stdlib, product layer |

Current completion status means translation audit status, not executable runtime
support. No runtime support is added by this audit.

## Gap Ledger

The actionable gap backlog is maintained in
`examples/ai/finrobot_translation/GAPS.md`.

Priority summary:

- P0: typed tools, trace/replay, config/secrets, API/web fixtures, typed data
  tables, SEC/document parsing, RAG, report artifacts, equity pipeline
  workflow, safety approvals, and provider-free evaluation.
- P1: model routing policy, valuation/sensitivity/chart/report packages,
  section agents, generated-code tooling, and substantial tutorial parity.
- P2: optional providers, advanced experiments, Backtrader/mplfinance/OpenBB,
  deployment, web app product flows, and packaging.
- P3: UI/static asset parity after product workflow parity exists.

Validation strategy:

- every P0 row must have a provider-free replay fixture;
- every external provider adapter must have recorded success, failure,
  rate-limit/auth, and schema-mismatch fixtures;
- every report/chart artifact must carry source, timestamp, provider,
  transformation history, artifact ID, AI-generated-section marker where
  applicable, and required-section validation;
- every high-risk operation, including trading, portfolio changes, generated
  code execution, network calls, credentials, and report publication, must be
  declared through capabilities and approval policy;
- live-provider tests remain optional and capability-gated.

## Leia Mapping

AI orchestration maps to general AI dialects:

- FinRobot LLM provider configs map to `model`.
- Single assistant runs map to callable `agent` values.
- Direct provider calls map to `turn`.
- Registered FinRobot toolkits map to `tool` values with parameter schemas,
  capability declarations, and structured errors.
- Multi-agent group chat maps to agents passed as tools or to explicit custom
  agent flows.
- Leader-directed workflows map to `agent` plus `workflow`, not to a finance
  dialect.
- Cache-backed repeat runs map to record/replay support.
- Human-input modes map to `approve` and workflow gates.
- Prompt profiles and role descriptions are package data, not syntax.

Data acquisition maps to general web/API/database/std packages:

- FMP, Finnhub, SEC, Yahoo Finance, and retail-sentiment integrations map to
  package-provided clients built on `api`, `web`, `json`, `csv`, and `table`
  support.
- SEC filing downloads map to `web` fetch/download plus file and document
  parsing libraries.
- Provider keys map to environment/config support and capability-scoped secrets.
- Cached source data maps to file/database stdlib APIs.

Financial analytics maps to data and package libraries:

- Statements, metrics, ratios, forecasts, and peer tables map to `data` tables,
  columnar transforms, matrix/vector operations, and chart libraries.
- DCF, multiples, sensitivity, catalyst, technical-indicator, portfolio, and
  backtesting logic should live in finance libraries.
- FinGPT, FinRL, FinML, and multimodal financial models are model/package
  choices, not language features.

RAG and document workflows map to general AI/document capabilities:

- SEC and earnings-call retrieval map to RAG libraries over generic vector,
  text, document, and file APIs.
- Filing section extraction maps to document parsers plus finance-specific SEC
  helpers.
- Report structure validation maps to ordinary schemas/tests.
- Data-source annotations and AI disclosure map to reporting libraries and
  metadata conventions.

Output generation maps to general document/chart/web capabilities:

- HTML reports map to `html` or template libraries.
- PDF reports map to document/export libraries.
- Charts map to a charting package fed by `data` tables.
- CLI and web-app flows map to `workflow`, `test`, `web`, and package metadata.

## Landing Checklist

The next FinRobot translation step should treat recent AI dialect polish as the
baseline and stop listing covered platform work as open gaps.

Covered by the current general AI/workflow contracts:

- Workflow coverage: FinRobot single-assistant runs, RAG-assistant runs,
  leader-directed runs, report pipelines, deterministic test runs, and human
  input gates map to `agent`, `workflow`, `approve`, `test`, and `evaluate`
  composition. The open work is not another orchestration dialect.
- Memory coverage: explicit history, RAG state, reset behavior, bounded context,
  and retrieval events are covered by `agent` memory contracts, message/history
  helpers, trace events, and record/replay. Finance packages still choose the
  backing store and retrieval strategy.
- Schema coverage: model-facing tool parameters, structured tool errors,
  expected agent outputs, report-section validation, and provider-free replay
  fixtures map to `tool` schemas, `output` validation, ordinary schemas/tests,
  and replay records.
- Sections coverage: annual-report sections, equity-research sections,
  disclosure markers, chart/report artifacts, and stale-data checks belong to
  report/document package metadata consumed by workflows. They are package data,
  not parser syntax.
- Handoff coverage: group chat, shadow assistants, planner/executor flows, and
  nested specialists map to agent-as-tool composition or explicit custom
  `flow` functions with parent/child traces, cancellation, budget, approval, and
  structured error propagation.
- Capability coverage: data fetches, filing downloads, generated code, trading
  actions, credentials, and high-risk tools are represented as declared
  capabilities and approval policies visible to host review tooling.

Remaining landing work, sorted by ownership:

1. General dialects and contracts
   - Finish `api` requirements for auth headers, retry, pagination, typed JSON
     decode, rate-limit metadata, and traceable errors.
   - Finish `web` requirements for robust download, scrape, generated-asset
     retrieval, redirect handling, and deterministic replay.
   - Finish `workflow` requirements for named stages, dependencies, artifacts,
     approvals, retry policy, report outputs, and failure-on-stale-section
     hooks.
   - Finish `evaluate` and `test` fixture conventions for provider-free agent
     regression tests, recorded tool calls, streaming turns, and nested-agent
     replay.
   - Define model-routing policy boundaries for smart-scheduler-like fallback,
     regional model selection, scoring, and trace-visible provider choice.
   - Define package metadata for external services, required capabilities,
     credential scopes, rate-limit declarations, and deployment-specific model
     aliases.

2. Standard library and shared packages
   - Add table joins, group-by, time-series windows, rolling metrics,
     missing-data handling, typed CSV/JSON IO, and reliable nested-data to typed
     table conversion.
   - Stabilize vector and matrix operations needed by forecasts,
     optimization, sensitivity analysis, portfolio experiments, and backtesting
     packages.
   - Provide provenance metadata conventions for source, timestamp, provider,
     retrieval URL, trace ID, transformation history, generated section, and
     chart/report artifact.
   - Provide generic document interfaces for PDF, HTML table, markdown,
     section extraction, file download, chunking, and report export.
   - Provide a generic RAG interface over local files, document chunks, vector
     stores, and provider-backed retrieval APIs.
   - Provide chart/report contracts over `data` tables so workflows can validate
     required sections, artifacts, source annotations, and AI disclosure
     markers.
   - Provide config/env/secret conventions that work consistently for local
     examples, CI, and deployed package hosts.

3. External and domain libraries
   - Implement Yahoo Finance, Finnhub, Financial Modeling Prep, SEC, earnings
     call, Reddit, X.com, Polymarket, OpenBB, and other provider adapters as
     packages with schemas, capabilities, rate limits, cache policy, and terms
     metadata.
   - Implement finance-specific normalizers for statements, metrics, ratios,
     peer tables, SEC filing sections, analyst recommendations, and market data.
   - Implement DCF, multiples, sensitivity, catalyst, sentiment, technical
     indicator, portfolio optimization, and backtesting libraries outside the
     core dialect layer.
   - Implement chart themes, report templates, section taxonomies, disclosure
     wording, and house-style prompts as package assets.
   - Implement FinRobot role profiles, prompt libraries, and workflow presets as
     package data over general `agent` and `workflow` contracts.
   - Implement FinGPT, FinRL, FinML, Backtrader, mplfinance, multimodal model,
     notebook, and image-display integrations as optional packages.
   - Implement web-app runtime pieces such as OAuth setup, admin routes, request
     logging, SQLite schemas, local-service commands, deployment scripts, and
     compliance review packages outside the AI dialect core.

## Library, Not Dialect

These must be ordinary packages or stdlib modules, not built into a finance
dialect:

- FMP, Finnhub, Yahoo Finance, SEC API, Reddit, X.com, Polymarket, OpenBB, or
  any other provider adapter;
- ticker/company profile helpers;
- SEC section names and filing heuristics;
- earnings-call transcript fetchers;
- DCF, EV/EBITDA, P/E, BVPS, market-cap, analyst-rating, and target-price
  calculations;
- forecast assumptions and peer-comparison formulas;
- risk, catalyst, sentiment, and investment-thesis prompt templates;
- FinRobot role profiles;
- chart themes and report templates;
- FinGPT, FinRL, FinML, Backtrader, mplfinance, and multimodal-model adapters;
- SQLite schemas for a specific web app;
- OAuth provider setup;
- deploy scripts;
- generated-code tools for notebooks or Python files;
- disclaimers and house-style report wording.

The language may provide the composition substrate: AI turns, agents, tools,
data tables, APIs, files, documents, charts, reports, workflows, tests,
capabilities, approvals, and replay. Finance packages should provide the domain
knowledge and provider integrations.

## Non-Goals

- Do not add `finrobot`, `finance`, `equity`, `ticker`, `sec`, or `trading` as
  parser keywords.
- Do not make stock tickers or financial statement fields special syntax.
- Do not bake any data vendor into Leia.
- Do not hardcode analyst roles, report sections, valuation methods, or prompt
  templates into the language.
- Do not make live trading or portfolio adjustment possible without explicit
  capabilities and approval.
