# FinRobot Document Pipeline Live Package Skeleton

This directory is a provider-free contract skeleton for the FinRobot document pipeline slice:

- `filings_src`: SEC filing search/fetch metadata and replay provenance.
- `marker_sec_src`: SEC HTML/PDF to markdown conversion and section extraction boundaries.
- `functional/rag.py`: deterministic chunking, citations, provenance, and vector adapter payloads.
- `functional/ragquery.py`: retriever query result shape, ranked chunks, and answer citations.

The package intentionally contains no SEC client, Marker, embedding model, vector database, retriever SDK, credentials, or live network behavior. Future live packages must satisfy these schemas and fixtures before replacing replay data.
