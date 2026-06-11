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

## Current Gaps

Core AI gaps:

- `agent` needs first-class custom flows for multi-agent coordination.
- `agent` needs explicit history, memory, and reset semantics.
- `tool` needs parameter-schema reflection from ordinary functions.
- `tool` needs capability declarations that are visible to review tooling.
- Record/replay needs to cover tool calls, LLM turns, and streaming output.
- Agent-as-tool composition needs clear error and cancellation behavior.
- Human approval needs to compose with agents and workflows.
- Provider/model routing is not yet rich enough to express smart-scheduler-like
  fallback, scoring, and regional model selection.

Data and analytics gaps:

- `data` needs table joins, group-by, time-series windows, rolling metrics, and
  missing-data handling.
- Numeric support needs stable vector/matrix operations for forecasting,
  optimization, sensitivity analysis, and backtesting libraries.
- Finance data usually arrives as nested JSON, CSV, HTML tables, PDFs, and
  provider-specific schemas; Leia needs reliable conversion into typed tables.
- There is no standard provenance model for source, timestamp, provider, and
  transformation history.
- There is no built-in charting/report data contract for generated assets.

Web/API/document gaps:

- `api` needs auth headers, retry, pagination, rate-limit handling, and typed
  decode paths.
- `web` needs robust download and scrape behavior for filings, articles, and
  generated assets.
- Document parsing needs PDF, HTML table, markdown, and section-extraction
  libraries.
- RAG needs a generic retrieval interface that can be backed by local files,
  vector stores, and provider APIs.
- Secret management needs a consistent config/env story for local examples,
  CI, and deployed apps.

Workflow, app, and ops gaps:

- `workflow` needs named stages, artifacts, dependencies, approvals, retry, and
  report outputs.
- `test` and `evaluate` need fixtures for provider-free regression tests.
- Web serving, auth, admin endpoints, request logging, and local persistence are
  app-runtime features, not currently covered by the minimal dialect drafts.
- Package metadata needs a way to declare external services and required
  capabilities.
- Generated reports need validation hooks so missing sections or stale data can
  fail a workflow.

Safety and compliance gaps:

- Financial analysis needs source provenance, timestamps, disclaimers, and
  AI-content markers as report-library conventions.
- Trading, portfolio adjustment, alerting, and code-writing tools need explicit
  high-risk capabilities and approval gates.
- Generated code tools must be sandboxed and should not be ordinary finance
  helpers.
- Data-provider terms, rate limits, and credential scopes need package-level
  declarations.

## Priority

P0, required before a FinRobot-like package is credible:

- `model`, `turn`, `tool`, `agent`, and `evaluate` contracts;
- agent custom flows and agent-as-tool composition;
- tool schemas, structured errors, capabilities, and approval gates;
- provider-independent record/replay for turns and tools;
- `api` with auth, retry, pagination, JSON decode, and rate-limit metadata;
- `data` tables with joins, group-by, time-series windows, typed CSV/JSON IO,
  and missing-data handling;
- package secrets/config conventions;
- workflow artifacts and deterministic tests.

P1, needed for high-quality equity research automation:

- generic RAG interface;
- PDF/HTML/markdown document parsing libraries;
- provenance metadata for source data and generated sections;
- charting package over `data` tables;
- report generation to HTML and PDF;
- schema validation for report sections and generated agent outputs;
- model routing policies for fallback and task-specific model selection.

P2, useful for production finance apps:

- web app runtime with auth, admin routes, request logging, and persistence;
- deployment metadata and local-service commands;
- benchmark/evaluation suites for finance agents;
- portfolio optimization, backtesting, technical indicators, and multimodal
  model adapters as installable packages;
- notebook interop and image display helpers.

P3, research or ecosystem extensions:

- financial model fine-tuning workflows;
- smart-scheduler scoring research;
- multilingual/regional finance model packs;
- trading execution adapters;
- compliance review packages.

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
