# FinRobot Translation Gaps

## Quant Experiments

- Live AutoGen nested-chat behavior is represented as declarative Leia agents and local tools; there is no caller/executor registration, cache disk state, Docker execution, or human-input mode.
- External market and filing sources are not invoked. Dow 30 factor data, PDD/peer committee inputs, SEC 20-F evidence, and news/sentiment inputs are fixed replay fixtures.
- BackTrader integration is reduced to a deterministic portfolio-stat tool. Strategy class loading, pandas/yfinance feeds, analyzers, chart rendering, and matplotlib output are not translated.
- `ReportAnalysisUtils`, `CodingUtils`, and `IPythonUtils` are represented by replayable prompt/code tools only; no filesystem prompt persistence, IPython notebook display, or generated Python modification loop is included.
- The q coverage is limited to table ranking, aggregation, weighting, caps, and order skeletons needed for the experiment slice. It deliberately does not add or change q runtime behavior.

