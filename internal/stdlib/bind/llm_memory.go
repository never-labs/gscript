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
		if source := doc.RawGetString("source").Str(); source != "" {
			b.WriteString("\nSource: ")
			b.WriteString(source)
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
		doc.RawGetString("text").Str(),
		llmMemoryValueText(doc.RawGetString("tags")),
		llmMemoryValueText(doc.RawGetString("metadata")),
	}, "\n"))
	score := 0
	for _, term := range terms {
		score += strings.Count(haystack, term)
	}
	return score
}
