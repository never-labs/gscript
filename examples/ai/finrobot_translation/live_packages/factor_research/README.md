# FinRobot Factor Research Live Package Skeleton

Provider-free skeleton for migrating FinRobot multi-factor research workflows into a live external package.

This package does not import provider SDKs, open network connections, or require credentials. It records the package boundary for:

- `experiments/multi_factor_agents.py` orchestration parity
- factor transform contracts
- portfolio factor exposure contracts
- market and factor data fixtures
- agent handoff metadata for future provider-backed implementations

