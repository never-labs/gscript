package bind

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const (
	llmMemoryDocMarker        = "__llm_doc"
	llmMemoryCollectionMarker = "__llm_collection"
	llmMemoryContextMarker    = "__llm_context"
	llmMemoryCorpusMarker     = "__llm_corpus"
)

func registerLLMMemoryHelpers(t *Table) {
	docFn := func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'llm.doc'")
		}
		opts := NilValue()
		if len(args) >= 2 {
			opts = args[1]
		}
		return []Value{TableValue(llmMemoryDoc(args[0], opts))}, nil
	}
	setLLMFunction(t, "llm", "doc", docFn)
	setLLMFunction(t, "llm", "document", docFn)

	collectionFn := func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'llm.collection'")
		}
		return []Value{TableValue(llmMemoryCollection(args[0]))}, nil
	}
	setLLMFunction(t, "llm", "collection", collectionFn)
	setLLMFunction(t, "llm", "docs", collectionFn)

	chunksFn := func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'llm.chunks'")
		}
		opts := NilValue()
		if len(args) >= 2 {
			opts = args[1]
		}
		return []Value{TableValue(llmMemoryChunks(args[0], opts))}, nil
	}
	setLLMFunction(t, "llm", "chunks", chunksFn)
	setLLMFunction(t, "llm", "chunk", chunksFn)

	citationFn := func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'llm.citation'")
		}
		return []Value{TableValue(llmMemoryCitation(args[0]))}, nil
	}
	setLLMFunction(t, "llm", "citation", citationFn)
	setLLMFunction(t, "llm", "cite", citationFn)

	corpusFn := func(args []Value) ([]Value, error) {
		input := NilValue()
		if len(args) >= 1 {
			input = args[0]
		}
		opts := NilValue()
		if len(args) >= 2 {
			opts = args[1]
		}
		return []Value{TableValue(llmMemoryCorpus(input, opts))}, nil
	}
	setLLMFunction(t, "llm", "corpus", corpusFn)

	corpusAddFn := func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.corpus_add' (corpus, docs expected)")
		}
		return []Value{TableValue(llmMemoryCorpusAdd(args[0], args[1]))}, nil
	}
	setLLMFunction(t, "llm", "corpus_add", corpusAddFn)

	corpusResetFn := func(args []Value) ([]Value, error) {
		opts := NewTable()
		if len(args) >= 1 {
			if args[0].IsTable() {
				llmCopyTable(opts, args[0].Table(), true)
			}
		}
		opts.RawSetString("reset", BoolValue(true))
		return []Value{TableValue(llmMemoryCorpus(NilValue(), TableValue(opts)))}, nil
	}
	setLLMFunction(t, "llm", "corpus_reset", corpusResetFn)

	contextFn := func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'llm.context'")
		}
		opts := NilValue()
		if len(args) >= 2 {
			opts = args[1]
		}
		return []Value{TableValue(llmMemoryContext(args[0], opts, "Context"))}, nil
	}
	setLLMFunction(t, "llm", "context", contextFn)

	evidenceFn := func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'llm.evidence'")
		}
		opts := NilValue()
		if len(args) >= 2 {
			opts = args[1]
		}
		return []Value{TableValue(llmMemoryContext(args[0], opts, "Evidence"))}, nil
	}
	setLLMFunction(t, "llm", "evidence", evidenceFn)

	retrieveFn := func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'llm.retrieve' (collection, query expected)")
		}
		opts := NilValue()
		if len(args) >= 3 {
			opts = args[2]
		}
		return []Value{TableValue(llmMemoryRetrieve(args[0], args[1].Str(), opts))}, nil
	}
	setLLMFunction(t, "llm", "retrieve", retrieveFn)
	setLLMFunction(t, "llm", "search", retrieveFn)
}

func llmApplyMemoryContext(opts *Table) {
	if opts == nil {
		return
	}
	var additions []Value
	for _, key := range []string{"context", "evidence"} {
		additions = append(additions, llmMemoryMessages(opts.RawGetString(key))...)
	}
	if len(additions) == 0 {
		return
	}
	base := llmMessageValuesFromTable(opts.RawGetString("messages").Table())
	all := append(base, additions...)
	opts.RawSetString("messages", llmTableFromValues(all))
}

func llmMemoryDoc(input, opts Value) *Table {
	doc := NewTable()
	doc.RawSetString(llmMemoryDocMarker, BoolValue(true))
	if input.IsTable() {
		llmCopyTable(doc, input.Table(), true)
	} else {
		doc.RawSetString("text", StringValue(input.Str()))
	}
	if opts.IsTable() {
		llmCopyTable(doc, opts.Table(), true)
	}
	if doc.RawGetString("text").IsNil() {
		doc.RawSetString("text", StringValue(llmMemoryValueText(input)))
	}
	if doc.RawGetString("artifact_id").IsNil() {
		doc.RawSetString("artifact_id", doc.RawGetString("id"))
	}
	if doc.RawGetString("metadata").IsNil() {
		doc.RawSetString("metadata", TableValue(llmMemoryDocMetadata(doc)))
	}
	if doc.RawGetString("citation").IsNil() {
		doc.RawSetString("citation", TableValue(llmMemoryCitation(TableValue(doc))))
	}
	return doc
}

func llmMemoryCollection(input Value) *Table {
	coll := NewTable()
	coll.RawSetString(llmMemoryCollectionMarker, BoolValue(true))
	docs := llmMemoryDocs(input)
	list := NewSequentialArrayTable(len(docs))
	for i, doc := range docs {
		list.RawSet(IntValue(int64(i+1)), TableValue(doc))
	}
	coll.RawSetString("docs", TableValue(list))
	coll.RawSetString("count", IntValue(int64(len(docs))))
	return coll
}

func llmMemoryChunks(input, opts Value) *Table {
	chunks := make([]*Table, 0)
	for _, doc := range llmMemoryDocs(input) {
		chunks = append(chunks, llmMemoryDocChunks(doc, opts)...)
	}
	list := NewSequentialArrayTable(len(chunks))
	for i, chunk := range chunks {
		list.RawSet(IntValue(int64(i+1)), TableValue(chunk))
	}
	coll := llmMemoryCollection(TableValue(list))
	coll.RawSetString("kind", StringValue("chunks"))
	return coll
}

func llmMemoryDocChunks(doc *Table, opts Value) []*Table {
	if doc == nil {
		return nil
	}
	if sections := doc.RawGetString("sections").Table(); sections != nil {
		keys := llmMemorySectionKeys(sections)
		out := make([]*Table, 0, len(keys))
		for _, key := range keys {
			text := sections.RawGetString(key).Str()
			if text == "" {
				text = llmMemoryValueText(sections.RawGetString(key))
			}
			out = append(out, llmMemoryChunkDoc(doc, key, text, len(out)+1, opts))
		}
		return out
	}
	return []*Table{llmMemoryChunkDoc(doc, "", doc.RawGetString("text").Str(), 1, opts)}
}

func llmMemoryChunkDoc(doc *Table, section, text string, index int, opts Value) *Table {
	chunk := NewTable()
	llmCopyTable(chunk, doc, true)
	docID := doc.RawGetString("id").Str()
	if docID == "" {
		docID = fmt.Sprintf("doc%d", index)
	}
	chunkID := doc.RawGetString("chunk_id").Str()
	if chunkID == "" {
		if section != "" {
			chunkID = docID + "#" + section
		} else {
			chunkID = fmt.Sprintf("%s#chunk%d", docID, index)
		}
	}
	if opts.IsTable() {
		llmCopyTable(chunk, opts.Table(), true)
	}
	chunk.RawSetString(llmMemoryDocMarker, BoolValue(true))
	chunk.RawSetString("doc_id", StringValue(docID))
	chunk.RawSetString("chunk_id", StringValue(chunkID))
	chunk.RawSetString("id", StringValue(chunkID))
	chunk.RawSetString("chunk_index", IntValue(int64(index)))
	chunk.RawSetString("text", StringValue(text))
	if section != "" {
		chunk.RawSetString("section", StringValue(section))
	}
	if chunk.RawGetString("artifact_id").IsNil() {
		chunk.RawSetString("artifact_id", doc.RawGetString("artifact_id"))
	}
	chunk.RawSetString("metadata", TableValue(llmMemoryDocMetadata(chunk)))
	chunk.RawSetString("citation", TableValue(llmMemoryCitation(TableValue(chunk))))
	return chunk
}

func llmMemoryCorpus(input, opts Value) *Table {
	corpus := llmMemoryCollection(input)
	corpus.RawSetString(llmMemoryCorpusMarker, BoolValue(true))
	corpus.RawSetString("kind", StringValue("local_corpus"))
	corpus.RawSetString("reset", BoolValue(false))
	if opts.IsTable() {
		llmCopyTable(corpus, opts.Table(), true)
		if opts.Table().RawGetString("reset").Truthy() {
			empty := NewSequentialArrayTable(0)
			corpus.RawSetString("docs", TableValue(empty))
			corpus.RawSetString("count", IntValue(0))
			corpus.RawSetString("reset", BoolValue(true))
		}
	}
	return corpus
}

func llmMemoryCorpusAdd(corpus, input Value) *Table {
	existing := llmMemoryDocs(corpus)
	additions := llmMemoryDocs(input)
	list := NewSequentialArrayTable(len(existing) + len(additions))
	for i, doc := range existing {
		list.RawSet(IntValue(int64(i+1)), TableValue(doc))
	}
	for i, doc := range additions {
		list.RawSet(IntValue(int64(len(existing)+i+1)), TableValue(doc))
	}
	out := llmMemoryCorpus(TableValue(list), NilValue())
	if corpus.IsTable() {
		for _, key := range corpus.Table().PairsKeysSnapshot() {
			if key.IsString() {
				switch key.Str() {
				case "docs", "count", "reset":
					continue
				}
			}
			out.RawSet(key, corpus.Table().RawGet(key))
		}
	}
	out.RawSetString("reset", BoolValue(false))
	return out
}

func llmMemoryContext(input, opts Value, defaultLabel string) *Table {
	ctx := NewTable()
	ctx.RawSetString(llmMemoryContextMarker, BoolValue(true))
	docs := llmMemoryDocs(input)
	if len(docs) == 0 && !input.IsNil() {
		docs = []*Table{llmMemoryDoc(input, NilValue())}
	}
	label := defaultLabel
	role := "user"
	if opts.IsTable() {
		if v := opts.Table().RawGetString("label"); v.IsString() && v.Str() != "" {
			label = v.Str()
		}
		if v := opts.Table().RawGetString("role"); v.IsString() && v.Str() != "" {
			role = v.Str()
		}
		llmCopyTable(ctx, opts.Table(), true)
	}
	text := llmMemoryContextText(label, docs)
	if override := ctx.RawGetString("text"); !override.IsNil() {
		text = override.Str()
	}
	docList := NewSequentialArrayTable(len(docs))
	for i, doc := range docs {
		docList.RawSet(IntValue(int64(i+1)), TableValue(doc))
	}
	msg := TableValue(llmMessageTable(role, text))
	ctx.RawSetString("docs", TableValue(docList))
	ctx.RawSetString("text", StringValue(text))
	ctx.RawSetString("message", msg)
	ctx.RawSet(IntValue(1), msg)
	return ctx
}

func llmMemoryRetrieve(collection Value, query string, opts Value) *Table {
	docs := llmMemoryDocs(collection)
	limit := len(docs)
	label := "Context"
	if opts.IsTable() {
		if n := toInt(opts.Table().RawGetString("limit")); n > 0 && int(n) < limit {
			limit = int(n)
		}
		if v := opts.Table().RawGetString("label"); v.IsString() && v.Str() != "" {
			label = v.Str()
		}
	}
	terms := llmMemoryTerms(query)
	sort.SliceStable(docs, func(i, j int) bool {
		left := llmMemoryScore(docs[i], terms)
		right := llmMemoryScore(docs[j], terms)
		if left == right {
			return docs[i].RawGetString("id").Str() < docs[j].RawGetString("id").Str()
		}
		return left > right
	})
	if limit < len(docs) {
		docs = docs[:limit]
	}
	list := NewSequentialArrayTable(len(docs))
	for i, doc := range docs {
		copyDoc := llmCloneTable(doc)
		copyDoc.RawSetString("score", IntValue(int64(llmMemoryScore(doc, terms))))
		copyDoc.RawSetString("citation", TableValue(llmMemoryCitation(TableValue(copyDoc))))
		list.RawSet(IntValue(int64(i+1)), TableValue(copyDoc))
	}
	ctx := llmMemoryContext(TableValue(list), opts, label)
	ctx.RawSetString("query", StringValue(query))
	ctx.RawSetString("matches", TableValue(list))
	return ctx
}

func llmMemoryMessages(v Value) []Value {
	if v.IsNil() {
		return nil
	}
	if t := v.Table(); t != nil {
		if msg := t.RawGetString("message"); msg.IsTable() {
			return []Value{msg}
		}
		if t.RawGetString("role").IsString() && t.RawGetString("text").IsString() {
			return []Value{v}
		}
		if t.RawGetString(llmMemoryContextMarker).Truthy() {
			return []Value{TableValue(llmMessageTable("user", t.RawGetString("text").Str()))}
		}
		if t.RawGetString(llmMemoryDocMarker).Truthy() || t.RawGetString(llmMemoryCollectionMarker).Truthy() || t.RawGetString("docs").IsTable() {
			return llmMemoryMessages(TableValue(llmMemoryContext(v, NilValue(), "Context")))
		}
		messages := llmMessageValuesFromTable(t)
		if len(messages) > 0 {
			return messages
		}
		return []Value{TableValue(llmMessageTable("user", llmMemoryValueText(v)))}
	}
	return []Value{TableValue(llmMessageTable("user", v.Str()))}
}

func llmMemoryDocs(v Value) []*Table {
	if v.IsNil() {
		return nil
	}
	t := v.Table()
	if t == nil {
		return []*Table{llmMemoryDoc(v, NilValue())}
	}
	if t.RawGetString(llmMemoryDocMarker).Truthy() {
		return []*Table{t}
	}
	if docs := t.RawGetString("docs"); docs.IsTable() {
		return llmMemoryDocs(docs)
	}
	out := make([]*Table, 0, t.Length())
	for i := 1; i <= t.Length(); i++ {
		item := t.RawGet(IntValue(int64(i)))
		if item.IsNil() {
			continue
		}
		if item.Table() != nil && item.Table().RawGetString(llmMemoryDocMarker).Truthy() {
			out = append(out, item.Table())
			continue
		}
		out = append(out, llmMemoryDoc(item, NilValue()))
	}
	return out
}

func llmMemoryContextText(label string, docs []*Table) string {
	var b strings.Builder
	if label == "" {
		label = "Context"
	}
	b.WriteString(label)
	b.WriteString(":")
	for i, doc := range docs {
		b.WriteString("\n[")
		id := doc.RawGetString("id").Str()
		if id == "" {
			id = fmt.Sprintf("doc%d", i+1)
		}
		b.WriteString(id)
		b.WriteString("]")
		if title := doc.RawGetString("title").Str(); title != "" {
			b.WriteString(" ")
			b.WriteString(title)
		}
		if section := doc.RawGetString("section").Str(); section != "" {
			b.WriteString(" section ")
			b.WriteString(section)
		}
		if source := doc.RawGetString("source").Str(); source != "" {
			b.WriteString("\nSource: ")
			b.WriteString(source)
		}
		if artifact := doc.RawGetString("artifact_id").Str(); artifact != "" {
			b.WriteString("\nArtifact: ")
			b.WriteString(artifact)
		}
		text := doc.RawGetString("text").Str()
		if text != "" {
			b.WriteString("\n")
			b.WriteString(text)
		}
	}
	return b.String()
}

func llmMemoryValueText(v Value) string {
	if v.IsString() {
		return v.Str()
	}
	data, err := json.Marshal(llmAnyFromValue(v))
	if err != nil {
		return v.Str()
	}
	return string(data)
}

func llmMemoryDocMetadata(doc *Table) *Table {
	meta := NewTable()
	if doc == nil {
		return meta
	}
	for _, key := range []string{"id", "doc_id", "chunk_id", "title", "source", "artifact_id", "section", "chunk_index"} {
		if v := doc.RawGetString(key); !v.IsNil() {
			meta.RawSetString(key, v)
		}
	}
	if tags := doc.RawGetString("tags"); !tags.IsNil() {
		meta.RawSetString("tags", tags)
	}
	return meta
}

func llmMemoryCitation(input Value) *Table {
	doc := input.Table()
	if doc == nil {
		docs := llmMemoryDocs(input)
		if len(docs) > 0 {
			doc = docs[0]
		}
	}
	citation := NewTable()
	if doc == nil {
		citation.RawSetString("text", StringValue(input.Str()))
		return citation
	}
	docID := doc.RawGetString("doc_id")
	if docID.IsNil() {
		docID = doc.RawGetString("id")
	}
	for _, key := range []string{"id", "chunk_id", "title", "source", "artifact_id", "section", "score"} {
		if v := doc.RawGetString(key); !v.IsNil() {
			citation.RawSetString(key, v)
		}
	}
	if !docID.IsNil() {
		citation.RawSetString("doc_id", docID)
	}
	if doc.RawGetString("chunk_id").IsNil() && !doc.RawGetString("id").IsNil() {
		citation.RawSetString("chunk_id", doc.RawGetString("id"))
	}
	if snippet := doc.RawGetString("text").Str(); snippet != "" {
		citation.RawSetString("snippet", StringValue(llmMemorySnippet(snippet, 180)))
	}
	return citation
}

func llmMemorySnippet(s string, limit int) string {
	s = strings.TrimSpace(s)
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return strings.TrimSpace(s[:limit]) + "..."
}

func llmMemorySectionKeys(sections *Table) []string {
	if sections == nil {
		return nil
	}
	if sections.Length() > 0 {
		keys := make([]string, 0, sections.Length())
		for i := 1; i <= sections.Length(); i++ {
			if section := sections.RawGet(IntValue(int64(i))).Table(); section != nil {
				name := section.RawGetString("name").Str()
				if name == "" {
					name = fmt.Sprintf("section%d", i)
				}
				sections.RawSetString(name, section.RawGetString("text"))
				keys = append(keys, name)
			}
		}
		if len(keys) > 0 {
			return keys
		}
	}
	keys := make([]string, 0)
	for _, key := range sections.PairsKeysSnapshot() {
		if key.IsString() {
			keys = append(keys, key.Str())
		}
	}
	sort.Strings(keys)
	return keys
}

func llmMemoryTerms(s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, term := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if term == "" || seen[term] {
			continue
		}
		seen[term] = true
		out = append(out, term)
	}
	return out
}

func llmMemoryScore(doc *Table, terms []string) int {
	if doc == nil || len(terms) == 0 {
		return 0
	}
	haystack := strings.ToLower(strings.Join([]string{
		doc.RawGetString("id").Str(),
		doc.RawGetString("title").Str(),
		doc.RawGetString("source").Str(),
		doc.RawGetString("artifact_id").Str(),
		doc.RawGetString("doc_id").Str(),
		doc.RawGetString("chunk_id").Str(),
		doc.RawGetString("section").Str(),
		doc.RawGetString("text").Str(),
		llmMemoryValueText(doc.RawGetString("tags")),
		llmMemoryValueText(doc.RawGetString("metadata")),
		llmMemoryValueText(doc.RawGetString("citation")),
	}, "\n"))
	score := 0
	for _, term := range terms {
		score += strings.Count(haystack, term)
	}
	return score
}
