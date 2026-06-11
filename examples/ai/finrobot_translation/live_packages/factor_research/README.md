# FinRobot Factor Research Live Package Skeleton

Provider-free skeleton for migrating FinRobot multi-factor research workflows into a live external package.

This package does not import provider SDKs, open network connections, or require credentials. It records the package boundary for:

- `experiments/multi_factor_agents.py` orchestration parity
- factor transform contracts
- factor transform registry ordering, provenance tags, and clean-skip failure modes
- portfolio factor exposure contracts
- market and factor data fixtures with replay provenance summaries
- exposure summaries and portfolio factor risk envelopes
- agent handoff metadata for future provider-backed implementations
- an optional optimizer gate that defaults disabled and falls back to fixtures
