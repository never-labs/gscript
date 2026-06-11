# FinRobot SEC Filings Live Package Skeleton

This directory is a provider-free skeleton for SEC filing search, fetch,
artifact provenance, and section extraction workflows.

It defines offline 10-K and 10-Q fixtures for search and fetch behavior, HTML
and PDF artifact provenance, redirect/cache metadata, user-agent and rate-limit
policy metadata, terms metadata, and adapter boundaries. It does not import SEC
clients, PDF/HTML parser dependencies, require credentials, or make live
network calls.
