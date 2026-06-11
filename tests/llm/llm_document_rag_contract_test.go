package leia_test

import (
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestLLMDocumentRAGCorpusContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := leia.New(llmScenarioOptions(nil, tc.opts...)...)
			if err := vm.Exec(`
filing := llm.document({
    id: "aapl-2025-10k"
    title: "Apple 2025 Form 10-K"
    source: "fixtures/sec/aapl-2025-10k.md"
    artifact_id: "artifact://sec/aapl-2025-10k.md"
    tags: {"sec", "10-k", "aapl"}
    sections: {
        item1: "Business section discusses products and services."
        item1a: "Risk factors include supply concentration and regulatory pressure."
        item7: "Management discussion highlights margin expansion and services growth."
    }
})

chunks := llm.chunks(filing)
seeded := llm.corpus(chunks, {id: "sec-local"})
seeded = llm.corpus_reset(seeded)
after_reset_count := seeded.count
corpus := llm.corpus_add(seeded, chunks)
retrieved := llm.retrieve(corpus, "supply risk regulatory pressure", {limit: 2 label: "SEC evidence"})
cite := llm.citation(retrieved.matches[1])

chunk_count := chunks.count
first_chunk_doc_id := chunks.docs[1].doc_id
risk_chunk_id := chunks.docs[2].chunk_id
risk_chunk_section := chunks.docs[2].section
risk_chunk_artifact := chunks.docs[2].artifact_id
risk_chunk_meta_section := chunks.docs[2].metadata.section
first_match_id := retrieved.matches[1].id
first_match_doc_id := retrieved.matches[1].citation.doc_id
first_match_section := retrieved.matches[1].citation.section
first_match_artifact := retrieved.matches[1].citation.artifact_id
first_match_score := retrieved.matches[1].score
retrieved_text := retrieved.text
manual_cite_chunk := cite.chunk_id
manual_cite_snippet := cite.snippet
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"chunk_count":             int64(3),
				"after_reset_count":       int64(0),
				"first_chunk_doc_id":      "aapl-2025-10k",
				"risk_chunk_id":           "aapl-2025-10k#item1a",
				"risk_chunk_section":      "item1a",
				"risk_chunk_artifact":     "artifact://sec/aapl-2025-10k.md",
				"risk_chunk_meta_section": "item1a",
				"first_match_id":          "aapl-2025-10k#item1a",
				"first_match_doc_id":      "aapl-2025-10k",
				"first_match_section":     "item1a",
				"first_match_artifact":    "artifact://sec/aapl-2025-10k.md",
				"manual_cite_chunk":       "aapl-2025-10k#item1a",
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
			score, err := vm.Get("first_match_score")
			if err != nil {
				t.Fatalf("Get first_match_score: %v", err)
			}
			if score.(int64) <= 0 {
				t.Fatalf("first_match_score = %#v, want positive", score)
			}
			text, err := vm.Get("retrieved_text")
			if err != nil {
				t.Fatalf("Get retrieved_text: %v", err)
			}
			if got := text.(string); !strings.Contains(got, "SEC evidence:") ||
				!strings.Contains(got, "[aapl-2025-10k#item1a] Apple 2025 Form 10-K section item1a") ||
				!strings.Contains(got, "Artifact: artifact://sec/aapl-2025-10k.md") {
				t.Fatalf("retrieved_text = %q", got)
			}
			snippet, err := vm.Get("manual_cite_snippet")
			if err != nil {
				t.Fatalf("Get manual_cite_snippet: %v", err)
			}
			if !strings.Contains(snippet.(string), "Risk factors include supply concentration") {
				t.Fatalf("manual_cite_snippet = %q", snippet)
			}
		})
	}
}
