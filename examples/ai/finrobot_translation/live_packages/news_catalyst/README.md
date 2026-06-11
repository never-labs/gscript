# FinRobot News Catalyst Live Package Skeleton

This directory is a provider-free skeleton for the FinRobot equity modules:

- `catalyst_analyzer.py`
- `news_integrator.py`
- `retail_sentiment_client.py`

It defines fixture-backed schemas and contracts for news relevance, category,
sentiment, impact scoring, source ranking, catalyst evidence links, retail
sentiment snapshots, and Polymarket/X/Reddit adapter boundaries. It does not
import provider SDKs, require credentials, or make live network calls.
