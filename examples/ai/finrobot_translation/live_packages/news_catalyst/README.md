# Provider-Free News/Event/Catalyst AI Dialect Package

This directory is a provider-free live package for the FinRobot equity modules:

- `catalyst_analyzer.py`
- `news_integrator.py`
- `retail_sentiment_client.py`

It also abstracts those module seams into a reusable AI dialect for financial
news, event extraction, and catalyst classification. The package defines
fixture-backed contracts for source ingestion normalization, event extraction,
catalyst taxonomy, freshness/staleness decay, dedupe and source confidence
envelopes, prompt obligations, news relevance, sentiment, impact scoring,
source ranking, catalyst evidence links, retail sentiment snapshots, and
Polymarket/X/Reddit adapter boundaries.

The generic dialect is intended to be reused beyond equity news for macro,
credit, and commodity event workflows that follow the same raw-source to
canonical-event to catalyst lifecycle.

The contract of record is provider-free: it does not import provider SDKs,
require credentials, or make live network calls. Provider-specific packages can
plug in later only by satisfying the fixture replay contracts first.
