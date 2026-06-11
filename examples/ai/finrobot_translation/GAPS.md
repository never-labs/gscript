# FinRobot Translation Gaps

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
