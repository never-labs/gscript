# Generic AI Dialect Package Index Audit

This directory records a provider-free package index for the generic AI dialect
surface exercised by the FinRobot translation work. The index is intentionally
not a FinRobot syntax contract: each entry names a generic AI capability,
points to an example, a Go test, and a fixture, and records the production
package boundary that is still missing.

The audit is read-only. It does not import live packages, call model providers,
touch external networks, or depend on the q runtime package.
