# FinRobot Translation Gap Register

This register tracks the remaining gaps after translating FinRobot into generic
Leia AI dialect examples. It intentionally avoids FinRobot-specific language
features: domain behavior belongs in packages, package data, replay fixtures, or
external adapters.

## Gap Categories

| Category | Meaning for FinRobot translation |
| --- | --- |
| `language` | Missing syntax or core language feature. Current audit finds no FinRobot-specific language gap. |
| `AI dialect` | Missing or incomplete `model`, `turn`, `tool`, `agent`, `workflow`, approval, trace, replay, or capability contract. |
| `stdlib` | Missing generic API/web/file/config/data/table/document/chart/report/db support. |
| `external library` | Missing finance, vendor, model, chart, report, notebook, or backtesting package. |
| `product layer` | Missing deployment, auth, UI, compliance, logging, operational, or packaging work. |

## Open Gaps

| ID | Gap | Category | Affected FinRobot modules | Priority | Validation |
| --- | --- | --- | --- | --- | --- |
| FR-GAP-001 | Typed tool contracts need stable schemas, capability declarations, result envelopes, structured errors, and replay keys. | AI dialect | `finrobot/toolkits.py`, all `data_source/*`, `functional/*`, equity modules | P0 | Tool inventory fixture fails if any tool lacks schema, capability, error type, or replay identity |
| FR-GAP-002 | Agent-as-tool and nested-agent workflows need parent/child traces, cancellation, budget, max-turn, approval, and structured error propagation. | AI dialect | `finrobot/agents/workflow.py`, `experiments/*`, `equity_agents/*` | P0 | Replay fixtures for single, RAG, group-chat, leader, and handoff workflows assert trace hierarchy |
| FR-GAP-003 | Provider-free replay must cover `turn`, tool, API/web, file, process, clock, chart/report artifacts, and nested agents. | AI dialect, stdlib | tutorials, experiments, equity pipeline, web process runner | P0 | CI replay run executes no network/model calls and fails on missing or ambiguous events |
| FR-GAP-004 | Model aliases and scheduler-style routing need trace-visible provider choice and deterministic replay behavior. | AI dialect | `configs/*`, `OAI_CONFIG_LIST`, `agent_library.py`, workflow classes | P1 | Config fixture records selected alias/provider/model per turn and replays the same routing decision |
| FR-GAP-005 | Config/env/secret handling needs consistent host secrets, missing-key diagnostics, redaction, and local defaults. | stdlib, product layer | `configs/*`, `config_api_keys`, equity config, deploy scripts | P0 | Secret-free config tests resolve env values, redact traces, and fail clearly on missing keys |
| FR-GAP-006 | API substrate needs auth headers, typed JSON decode, retry, pagination, rate-limit metadata, cache policy, and traceable errors. | stdlib | FMP, Finnhub, SEC, Reddit, Adanos, OpenBB, Yahoo/FMP modules | P0 | Recorded fixtures cover success, empty response, rate-limit, auth failure, pagination, and schema mismatch |
| FR-GAP-007 | Web/download substrate needs redirects, streaming downloads, generated asset retrieval, scrape parsing, replay, and terms/capability metadata. | stdlib, product layer | SEC downloads, PDF conversion, FinNLP downloads, web app downloads | P0 | Golden SEC filing and report-asset fixtures replay without live network |
| FR-GAP-008 | Typed table/data support needs joins, group-by, windows, rolling metrics, missing-data handling, CSV/JSON IO, nested JSON flattening, and provenance columns. | stdlib | financial processors, market data, valuation, sensitivity, portfolio experiments | P0 | Unit fixtures validate statement normalization, forecasts, peer joins, and provenance |
| FR-GAP-009 | Vector/matrix/math support needs deterministic optimization and sensitivity primitives. | stdlib, external library | sensitivity, valuation, portfolio optimization, Backtrader wrapper | P1 | Numeric fixtures with tolerance checks and fixed seeds |
| FR-GAP-010 | Document parsing needs generic PDF, HTML table, markdown, section extraction, chunking, file artifact, and conversion APIs. | stdlib, external library | SEC helpers, filing parsers, marker conversion, RAG tutorials, report generation | P0 | Known 10-K and annual-report fixtures verify sections, chunks, artifact IDs, and parse errors |
| FR-GAP-011 | RAG needs generic corpus/index/retrieval contracts over local files, SEC filings, earnings calls, and provider-backed retrieval APIs. | AI dialect, stdlib, external library | `functional/rag.py`, `ragquery.py`, `earnings_calls_src/*`, RAG tutorials | P0 | Replay tests assert retrieval inputs, chunks, citations, reset behavior, and no live provider calls |
| FR-GAP-012 | Chart/report contracts need declared inputs, source annotations, artifact IDs, dimensions, required sections, stale-data checks, and AI disclosure markers. | stdlib, product layer | chart generators, HTML renderers, PDF generators, `report_structure.py` | P0 | Golden HTML/PDF/chart fixtures check required sections, artifact references, source metadata, and disclosures |
| FR-GAP-013 | Finance vendor adapters are needed for Yahoo Finance, Finnhub, FMP, SEC, earnings calls, Reddit, X.com, Polymarket, OpenBB, and FinNLP datasets. | external library | all `data_source/*`, `market_data_api.py`, `retail_sentiment_client.py`, advanced tutorials | P0/P2 | Adapter tests per provider with recorded payloads, schemas, rate limits, and terms metadata |
| FR-GAP-014 | Finance normalizers need schemas for statements, ratios, market data, analyst recommendations, SEC sections, peer metrics, and news/sentiment records. | external library, stdlib | data source modules, equity market data and processors | P0 | Typed schema fixtures reject stale/missing fields and preserve provider provenance |
| FR-GAP-015 | Valuation and analytics packages need DCF, EV/EBITDA, P/E, BVPS, market cap, target price, sensitivity, catalysts, technical indicators, and thesis/risk helpers. | external library, stdlib | valuation, sensitivity, catalyst, news, analyzer modules | P1 | Deterministic fixtures compare calculations, assumptions, and explanation schemas |
| FR-GAP-016 | Prompt/role/report-section packages need role profiles, financial prompts, equity section agents, report taxonomy, and templates as data. | AI dialect, external library | `agent_library.py`, `prompts.py`, `equity_agents/*`, text generators, templates | P1 | Snapshot tests for rendered prompts plus output-schema tests for each section agent |
| FR-GAP-017 | Generated-code and notebook tooling needs capability-gated file operations, Python execution, image display, command approval, and replayable outputs. | AI dialect, stdlib, product layer | `functional/coding.py`, coding tutorials, notebooks | P1 | Approval/replay fixtures for read/write/execute/display, including denied command cases |
| FR-GAP-018 | Optional integrations are missing for FinGPT, FinRL, FinML, Backtrader, mplfinance, multimodal chart/document models, Ollama, and OpenBB. | external library | tutorials, experiments, `functional/quantitative.py`, charting modules | P2 | Optional-package examples are capability-gated and skip cleanly when dependencies are absent |
| FR-GAP-019 | Equity report CLI workflow needs named stages, dependencies, artifacts, retries, stale-section failure hooks, and final report outputs. | AI dialect, stdlib, product layer | `generate_financial_analysis.py`, `create_equity_report.py`, `generate_pdf_report.py` | P0 | End-to-end fixture builds CSVs, text sections, charts, HTML, and PDF from recorded inputs |
| FR-GAP-020 | Web app product layer needs route parity, auth/session handling, OAuth, admin endpoints, background logs, history, downloads, and SQLite CRUD. | product layer, stdlib | `finrobot_equity/web_app/*` | P2 | Route smoke tests with mocked workflows, DB fixtures, auth sessions, and log/report artifacts |
| FR-GAP-021 | Deployment/package metadata needs local service commands, Docker/gcloud docs, dependency extras, and install checks outside core dialects. | product layer, external library | `requirements*.txt`, `setup.py`, `Dockerfile`, `deploy*.sh`, `run_web_app.py` | P2 | Packaging smoke test and documentation check for optional extras and environment setup |
| FR-GAP-022 | Compliance and safety gates need explicit approvals for trading, portfolio changes, generated code execution, external network calls, credentials, and report publication. | AI dialect, product layer | trading tutorials, portfolio experiments, coding tools, report generators | P0 | Capability-policy tests deny high-risk actions by default and record approvals in replay traces |
| FR-GAP-023 | Evaluation harness needs offline records, golden outputs, metrics, fixture versioning, and optional live-provider runs. | AI dialect, stdlib | all examples and tutorials | P0 | `evaluate` suite runs all P0 examples provider-free and emits a CI-friendly report |
| FR-GAP-024 | Product UI snapshots are needed for template/static parity but are not on the core translation path. | product layer | `web_app/templates/*`, `web_app/static/*` | P3 | Snapshot and accessibility checks after workflow/web parity exists |

## Translation Slice Gaps

### Core Agents

- AutoGen `GroupChat` speaker selection is translated as explicit coordinator history and assertions. Leia has agents, tools, turns, and history, but no drop-in mutable `GroupChatManager` object.
- FinRobot `UserProxyAgent` code execution is represented as ordinary tools or tool-result history, not exact `human_input_mode`, `max_consecutive_auto_reply`, or `code_execution_config` lifecycle knobs.
- FinRobot RAG wiring through `get_rag_function(...)` is expressible as a normal Leia tool, but vector-store setup and retrieval backends remain package resources.
- Nested chat summary modes such as `summary_method="reflection_with_llm"` are modeled with structured specialist output plus a follow-up leader turn.
- `TERMINATE` remains a prompt-level convention; Leia validates completion through structured outputs and assertions instead of reinterpreting it as a control signal.

### Data Tools

- Real provider clients are intentionally not translated: yfinance, Finnhub, SEC API, FMP, Reddit/PRAW, FinNLP downloaders, and earnings-call HTTP calls are represented by replay documents and `llm.tool`.
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

- Chart rendering is represented as chart specs only. A real translation still needs a chart package for stock price, share-performance, PE/EPS, revenue/EBITDA, EV/EBITDA, margin, sensitivity, technical-indicator, waterfall, radar, and comparison charts.
- HTML rendering is modeled as a deterministic artifact boundary. The full template system still needs reusable template assets, table renderers, markdown-to-HTML conversion, fallback formatting, disclosure components, and source-provenance markup.
- PDF generation is planned but not implemented. A document/export package must provide styled A4 layouts, frames/columns, cover pages, table pagination, image fitting, page headers/footers, font registration, and HTML-to-PDF or report-object-to-PDF conversion.
- Web orchestration is captured as staged task metadata only; routes, workers, logs, persistence, downloads, auth, admin views, static assets, and status recovery remain product-layer work.
- Artifact contracts need formal schemas for report sections, chart specs, source annotations, AI markers, stale-data checks, HTML/PDF output manifests, and user-visible warnings.

## Closed Or Non-Gaps

| Item | Decision | Validation |
| --- | --- | --- |
| FinRobot-specific parser syntax | Not needed. FinRobot maps to general AI, data, web/API, document, chart, report, workflow, and package capabilities. | Translation review rejects new parser keywords for `finrobot`, `ticker`, `sec`, `trading`, valuation, or report-section concepts. |
| Finance vendors as built-ins | Not needed. Vendors belong in external packages with capabilities, credentials, schemas, rate-limit metadata, and terms metadata. | Vendor package manifests declare capabilities and replay fixtures. |
| Role profiles as language concepts | Not needed. Roles are package data consumed by generic `agent` declarations. | Role registry snapshot is data-only and uses no parser changes. |

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
