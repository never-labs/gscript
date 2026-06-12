# FinRobot Policy Parity

This directory records provider-free parity coverage for approval and capability
policy behavior used by the FinRobot translation examples.

`ledger.json` is the canonical audit artifact. It maps high-risk FinRobot-style
workflow actions to Leia policy dimensions and verifies that each dimension has
deny, approve, and clean-skip cases without calling live providers, reading real
credentials, executing local code, writing files, publishing reports, or placing
trades.

The ledger intentionally references `../compliance_policy.leia` as the executable
policy shape, but keeps these parity fixtures as inert JSON metadata.
