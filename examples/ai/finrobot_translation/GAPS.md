# FinRobot Translation Gaps

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
