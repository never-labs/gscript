# FinRobot Quant Experiments Translation

This directory translates the FinRobot quantitative experiment slice into
offline Leia examples. The examples map the Python AutoGen/FinRobot workflows to
three Leia dialect layers:

- Leia orchestration for deterministic control flow and assertions.
- q table operations for factor, risk, and portfolio calculations.
- AI dialect agents/tools with `mock-fast` so runs are replay-friendly and do
  not call model providers, market data APIs, BackTrader, IPython, or the q
  runtime internals.

Source coverage:

- `.external/FinRobot/experiments/multi_factor_agents.py`
- `.external/FinRobot/experiments/portfolio_optimization.py`
- `.external/FinRobot/experiments/investment_group.py`
- `.external/FinRobot/finrobot/functional/quantitative.py`
- `.external/FinRobot/finrobot/functional/analyzer.py`
- `.external/FinRobot/finrobot/functional/coding.py`

Run smoke checks from the repository root. Each command loads the sibling replay
fixture and consumes the AI turn offline:

```sh
go run ./cmd/leia evaluate --replay examples/ai/finrobot_translation/quant_experiments/multi_factor_agents.records.json examples/ai/finrobot_translation/quant_experiments/multi_factor_agents.leia
go run ./cmd/leia evaluate --replay examples/ai/finrobot_translation/quant_experiments/portfolio_optimization.records.json examples/ai/finrobot_translation/quant_experiments/portfolio_optimization.leia
go run ./cmd/leia evaluate --replay examples/ai/finrobot_translation/quant_experiments/investment_group.records.json examples/ai/finrobot_translation/quant_experiments/investment_group.leia
```
