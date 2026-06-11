# FinRobot Valuation Engine Live Package

Provider-free valuation scenario coverage for the FinRobot translation slice.
This package is fixture-driven and does not import provider SDKs, finance
APIs, network clients, or credentials.

The skeleton expresses generic valuation dialect behavior:

- Bear/base/bull DCF sensitivity with explicit WACC, terminal growth, and FCF
  multipliers.
- WACC by terminal-growth grid coverage.
- Peer multiple outlier rejection before EV/EBITDA and P/E synthesis.
- Analyst target conflict detection and trimmed consensus handling.
- Scenario audit trail events that explain inputs, filters, conflicts, and
  target synthesis.

Run `main.leia` to produce the deterministic summary used by CI.
