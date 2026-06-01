package modules

import (
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
	llmrt "github.com/never-labs/gscript/internal/stdlibrt/llm"
)

type llmInstallRecorder struct {
	modules map[string]runtime.Value
	aliases map[string]runtime.Value
}

func newLLMInstallRecorder() *llmInstallRecorder {
	return &llmInstallRecorder{
		modules: make(map[string]runtime.Value),
		aliases: make(map[string]runtime.Value),
	}
}

func (r *llmInstallRecorder) RegisterModule(name string, module runtime.Value) {
	r.modules[name] = module
}

func (r *llmInstallRecorder) RegisterTable(name string, table *runtime.Table) {
	r.RegisterModule(name, runtime.TableValue(table))
}

func (r *llmInstallRecorder) RegisterAlias(name string, value runtime.Value) {
	r.aliases[name] = value
}

func TestInstallLLMRegistersPublicBindingsAndToolofAlias(t *testing.T) {
	rec := newLLMInstallRecorder()

	InstallLLM(rec, llmrt.Options{})

	for _, name := range []string{"llm", "msg", "history", "chat", "loop"} {
		val := rec.modules[name]
		if !val.IsTable() {
			t.Fatalf("%s module = %v, want table", name, val.Type())
		}
	}
	toolof := rec.aliases["toolof"]
	if !toolof.IsFunction() {
		t.Fatalf("toolof alias = %v, want function", toolof.Type())
	}
	llmToolof := rec.modules["llm"].Table().RawGetString("toolof")
	if toolof != llmToolof {
		t.Fatalf("toolof alias should reuse llm.toolof binding")
	}
}
