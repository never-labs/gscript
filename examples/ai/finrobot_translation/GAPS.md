# FinRobot Translation Gaps

## Data Tools

- Real provider clients are intentionally not translated: yfinance, Finnhub, SEC API, FMP, Reddit/PRAW, FinNLP downloaders, and earnings-call HTTP calls are represented by local replay documents and `llm.tool` dispatch only.
- DataFrame-specific behavior is approximated as Leia tables with field projection and row limits; CSV/file save paths, pandas index handling, and provider pagination are not modeled.
- SEC filing download/render/PDF conversion/cache behavior is reduced to replay metadata plus section text evidence; no HTML parsing, PDF generation, marker conversion, or SEC section classifier is included.
- Earnings-call transcript parsing is represented as pre-split speaker segments; retry behavior, date correction, and LangChain `Document` interoperability remain out of scope.
- Toolkit registration is translated as a `register_data_toolkit()` list of `llm.tool` values, not AutoGen caller/executor registration or class-method decoration.
