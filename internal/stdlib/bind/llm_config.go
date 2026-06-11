package bind

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

type llmConfigOptions struct {
	envRead func() bool
}

func BuildLLMConfigLib(options ...llmConfigOptions) *Table {
	opts := llmConfigOptions{}
	if len(options) > 0 {
		opts = options[0]
	}
	if opts.envRead == nil {
		opts.envRead = func() bool { return true }
	}

	lib := NewTable()
	set := func(name string, fn func([]Value) ([]Value, error)) {
		lib.RawSetString(name, FunctionValue(&GoFunction{Name: "config." + name, Fn: fn}))
	}

	set("table", func(args []Value) ([]Value, error) {
		if len(args) == 0 || args[0].IsNil() {
			return []Value{TableValue(NewTable())}, nil
		}
		if !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'config.table' (table expected)")
		}
		return []Value{TableValue(llmConfigCopy(args[0].Table()))}, nil
	})

	set("secret", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() || args[0].Str() == "" {
			return nil, fmt.Errorf("bad argument #1 to 'config.secret' (environment variable name expected)")
		}
		required := true
		if len(args) >= 2 && args[1].IsTable() {
			if v := args[1].Table().RawGetString("required"); !v.IsNil() {
				required = v.Truthy()
			}
		}
		return []Value{TableValue(llmConfigSecret(args[0].Str(), required))}, nil
	})

	set("env", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() || args[0].Str() == "" {
			return nil, fmt.Errorf("bad argument #1 to 'config.env' (environment variable name expected)")
		}
		defaultValue := NilValue()
		required := false
		if len(args) >= 2 && args[1].IsTable() {
			cfg := args[1].Table()
			defaultValue = cfg.RawGetString("default")
			if v := cfg.RawGetString("required"); !v.IsNil() {
				required = v.Truthy()
			}
		}
		value, found, errValue := llmConfigLookupEnv(args[0].Str(), opts.envRead)
		if !errValue.IsNil() {
			return []Value{NilValue(), errValue}, nil
		}
		if found {
			return []Value{StringValue(value), NilValue()}, nil
		}
		if !defaultValue.IsNil() {
			return []Value{defaultValue, NilValue()}, nil
		}
		if required {
			return []Value{NilValue(), llmConfigMissing("env", args[0].Str(), "")}, nil
		}
		return []Value{NilValue(), NilValue()}, nil
	})

	set("resolve", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'config.resolve' (table expected)")
		}
		out, errValue := llmConfigResolve(args[0].Table(), opts.envRead)
		if !errValue.IsNil() {
			return []Value{NilValue(), errValue}, nil
		}
		return []Value{TableValue(out), NilValue()}, nil
	})

	set("redact", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return []Value{NilValue()}, nil
		}
		return []Value{llmConfigRedactValue(args[0], "")}, nil
	})

	set("display", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return []Value{StringValue("nil")}, nil
		}
		return []Value{StringValue(llmConfigDisplay(args[0], ""))}, nil
	})

	return lib
}

func llmConfigSecret(envName string, required bool) *Table {
	t := NewTable()
	t.RawSetString("__config_secret", BoolValue(true))
	t.RawSetString("source", StringValue("env"))
	t.RawSetString("key", StringValue(envName))
	t.RawSetString("required", BoolValue(required))
	t.RawSetString("redacted", BoolValue(true))
	t.RawSetString("display", StringValue("<redacted:"+envName+">"))
	return t
}

func llmConfigResolve(src *Table, envRead func() bool) (*Table, Value) {
	out := NewTable()
	for _, key := range llmConfigSortedKeys(src) {
		value := src.RawGetString(key)
		if value.IsTable() && llmConfigIsSecret(value.Table()) {
			spec := value.Table()
			envName := spec.RawGetString("key").Str()
			raw, found, errValue := llmConfigLookupEnv(envName, envRead)
			if !errValue.IsNil() {
				return nil, errValue
			}
			if found {
				out.RawSetString(key, StringValue(raw))
				continue
			}
			if fallback := spec.RawGetString("default"); !fallback.IsNil() {
				out.RawSetString(key, fallback)
				continue
			}
			if spec.RawGetString("required").Truthy() {
				return nil, llmConfigMissing("env", envName, key)
			}
			out.RawSetString(key, NilValue())
			continue
		}
		out.RawSetString(key, llmConfigCopyValue(value))
	}
	return out, NilValue()
}

func llmConfigLookupEnv(name string, envRead func() bool) (string, bool, Value) {
	if envRead != nil && !envRead() {
		return "", false, llmErrorValue("config", "environment read access disabled")
	}
	value, found := os.LookupEnv(name)
	return value, found, NilValue()
}

func llmConfigMissing(source, name, field string) Value {
	message := "missing required config key"
	if field != "" {
		message += " '" + field + "'"
	}
	if source != "" && name != "" {
		message += " from " + source + " '" + name + "'"
	}
	err := llmErrorValue("config", message).Table()
	err.RawSetString("missing", StringValue(name))
	if field != "" {
		err.RawSetString("field", StringValue(field))
	}
	if source != "" {
		err.RawSetString("source", StringValue(source))
	}
	return TableValue(err)
}

func llmConfigIsSecret(t *Table) bool {
	return t != nil && t.RawGetString("__config_secret").Truthy()
}

func llmConfigCopy(src *Table) *Table {
	out := NewTable()
	for _, key := range llmConfigSortedKeys(src) {
		out.RawSetString(key, llmConfigCopyValue(src.RawGetString(key)))
	}
	return out
}

func llmConfigCopyValue(value Value) Value {
	if value.IsTable() {
		return TableValue(llmConfigCopy(value.Table()))
	}
	return value
}

func llmConfigRedactValue(value Value, key string) Value {
	if value.IsTable() {
		t := value.Table()
		if llmConfigIsSecret(t) {
			return StringValue(t.RawGetString("display").Str())
		}
		out := NewTable()
		for _, childKey := range llmConfigSortedKeys(t) {
			out.RawSetString(childKey, llmConfigRedactValue(t.RawGetString(childKey), childKey))
		}
		return TableValue(out)
	}
	if llmConfigSecretLikeKey(key) && !value.IsNil() {
		return StringValue("<redacted>")
	}
	return value
}

func llmConfigDisplay(value Value, key string) string {
	if value.IsTable() {
		t := value.Table()
		if llmConfigIsSecret(t) {
			if display := t.RawGetString("display"); display.IsString() && display.Str() != "" {
				return display.Str()
			}
			return "<redacted>"
		}
		parts := make([]string, 0, len(llmConfigSortedKeys(t)))
		for _, childKey := range llmConfigSortedKeys(t) {
			parts = append(parts, childKey+": "+llmConfigDisplay(t.RawGetString(childKey), childKey))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	if llmConfigSecretLikeKey(key) && !value.IsNil() {
		return "<redacted>"
	}
	if value.IsString() {
		return value.Str()
	}
	return value.String()
}

func llmConfigSecretLikeKey(key string) bool {
	name := strings.ToLower(key)
	for _, marker := range []string{"secret", "token", "password", "api_key", "apikey", "access_key"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func llmConfigSortedKeys(t *Table) []string {
	if t == nil {
		return nil
	}
	keys := make([]string, 0)
	k, _, ok := t.Next(NilValue())
	for ok {
		if k.IsString() {
			keys = append(keys, k.Str())
		}
		k, _, ok = t.Next(k)
	}
	sort.Strings(keys)
	return keys
}
