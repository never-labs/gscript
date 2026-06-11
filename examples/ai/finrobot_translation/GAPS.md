# FinRobot Translation Gaps

## Core Agents

- AutoGen `GroupChat` speaker selection is translated as explicit coordinator history and assertions. Leia currently has primitives for agents, tools, turns, and history, but no direct drop-in object equivalent for mutable `GroupChatManager` speaker policy.
- FinRobot `UserProxyAgent` code execution is represented as ordinary tools or tool-result history. This avoids adding financial or execution-specific runtime APIs, but it does not preserve AutoGen's exact proxy lifecycle knobs such as `human_input_mode`, `max_consecutive_auto_reply`, or `code_execution_config`.
- FinRobot RAG wiring through `get_rag_function(...)` can be expressed as a normal Leia tool, but this skeleton does not translate vector-store setup or retrieval backends because those are application-specific resources rather than core agent/workflow semantics.
- Nested chat summary modes such as `summary_method="reflection_with_llm"` are modeled with structured specialist output plus a follow-up leader turn. There is not yet a named Leia dialect field that exactly mirrors AutoGen summary methods.
- `TERMINATE` remains a prompt-level convention in the translated examples. Leia structured outputs and assertions can validate completion state, but they do not reinterpret `TERMINATE` as a built-in control signal.

## Reporting/Web

- Chart rendering is represented as chart specs only. A real translation still
  needs a chart package that can render mplfinance/matplotlib-equivalent stock
  price, share-performance, PE/EPS, revenue/EBITDA, EV/EBITDA, margin,
  sensitivity, technical-indicator, waterfall, radar, and comparison charts.
- HTML rendering is modeled as a deterministic artifact boundary. The full
  FinRobot professional template system still needs reusable template assets,
  table renderers, markdown-to-HTML conversion, fallback formatting, disclosure
  components, and source-provenance markup.
- PDF generation is planned but not implemented. A document/export package must
  provide styled A4 layouts, frames/columns, cover pages, table pagination,
  image fitting, page headers/footers, font registration, and HTML-to-PDF or
  report-object-to-PDF conversion.
- Web orchestration is captured as staged task metadata only. A real app still
  needs serve routes, background workers, request logs, task persistence,
  downloadable artifacts, auth/session handling, admin views, static assets, and
  restart-safe status recovery.
- The example does not execute provider-backed financial analysis, text-agent
  regeneration, enhanced news, catalyst, sensitivity, valuation, or retail
  sentiment steps. Those remain finance package workflows feeding the reporting
  boundary.
- Artifact contracts need formal schemas for report sections, chart specs,
  source annotations, AI-generated markers, stale-data checks, HTML/PDF output
  manifests, and user-visible warnings.

## Data Tools

- Real provider clients are intentionally not translated: yfinance, Finnhub, SEC API, FMP, Reddit/PRAW, FinNLP downloaders, and earnings-call HTTP calls are represented by local replay documents and `llm.tool` dispatch only.
- DataFrame-specific behavior is approximated as Leia tables with field projection and row limits; CSV/file save paths, pandas index handling, and provider pagination are not modeled.
- SEC filing download/render/PDF conversion/cache behavior is reduced to replay metadata plus section text evidence; no HTML parsing, PDF generation, marker conversion, or SEC section classifier is included.
- Earnings-call transcript parsing is represented as pre-split speaker segments; retry behavior, date correction, and LangChain `Document` interoperability remain out of scope.
- Toolkit registration is translated as a `register_data_toolkit()` list of `llm.tool` values, not AutoGen caller/executor registration or class-method decoration.

## Equity Report

- Live FMP/market/news/retail-sentiment API access is represented by local
  fixtures so the Leia workflow can replay offline.
- Pandas dataframe transformations, CSV persistence, and chart generation are
  summarized as normalized fixture tools rather than translated as runtime data
  processing.
- Professional HTML, legacy page rendering, and PDF generation are outside this
  workflow because the task scope is the replayable AI report pipeline, not the
  renderer stack.
- FinRobot's optional enhanced modules such as sensitivity heatmaps, technical
  indicators, valuation waterfall charts, and retail sentiment details are
  represented as structured summaries only.
- The source agents reference web search for leadership, competitors, and
  valuation context; the Leia translation uses replay fixtures instead to avoid
  network dependence.
