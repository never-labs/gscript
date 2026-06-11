package leia_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type agentBuilderConfigFixture struct {
	SchemaVersion         int    `json:"schema_version"`
	ID                    string `json:"id"`
	Source                string `json:"source"`
	ProviderFree          bool   `json:"provider_free"`
	LiveNetwork           bool   `json:"live_network"`
	LiveModel             bool   `json:"live_model"`
	CredentialsRequired   bool   `json:"credentials_required"`
	RealDependencyImports bool   `json:"real_dependency_imports"`
	Builder               struct {
		ID                string `json:"id"`
		SaveLoadRoundtrip bool   `json:"save_load_roundtrip"`
		GroupChat         struct {
			MaxRound int `json:"max_round"`
		} `json:"groupchat"`
	} `json:"builder"`
	Agents []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Metadata    struct {
			Seniority      string `json:"seniority"`
			Responsibility string `json:"responsibility"`
		} `json:"metadata"`
		Tools []string `json:"tools"`
	} `json:"agents"`
}

type agentBuilderTraceFixture struct {
	SchemaVersion       int    `json:"schema_version"`
	ID                  string `json:"id"`
	Source              string `json:"source"`
	ProviderFree        bool   `json:"provider_free"`
	LiveNetwork         bool   `json:"live_network"`
	LiveModel           bool   `json:"live_model"`
	CredentialsRequired bool   `json:"credentials_required"`
	GroupChat           struct {
		MaxRound int `json:"max_round"`
		Rounds   []struct {
			Round        int    `json:"round"`
			AgentID      string `json:"agent_id"`
			Role         string `json:"role"`
			Tool         string `json:"tool"`
			EvidenceID   string `json:"evidence_id"`
			ProviderFree bool   `json:"provider_free"`
		} `json:"rounds"`
	} `json:"groupchat"`
}

type agentBuilderReplayRecordsFixture struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Source        string `json:"source"`
	ProviderFree  bool   `json:"provider_free"`
	Records       []struct {
		ReplayKey       string   `json:"replay_key"`
		Mode            string   `json:"mode"`
		Models          []string `json:"models"`
		NetworkRequests []string `json:"network_requests"`
		Credentials     []string `json:"credentials"`
		LoadedTurns     int      `json:"loaded_turns"`
		ReplayedTurns   int      `json:"replayed_turns"`
		ToolCalls       []string `json:"tool_calls"`
	} `json:"records"`
}

func TestFinRobotAgentBuilderReplayExampleExecutesProviderFree(t *testing.T) {
	want := "agent_builder_replay roster=4 leaders=1 members=3 max_round=4 trace=4 first_tool=assign_sections saved_bytes=1187 provider_free=true live_network=false"
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var prints []string
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString | leia.LibJSON | leia.LibLLM),
				leia.WithPrint(func(args ...any) {
					var parts []string
					for _, arg := range args {
						parts = append(parts, fmt.Sprint(arg))
					}
					prints = append(prints, strings.Join(parts, " "))
				}),
			}, tc.opts...)...)

			if err := vm.ExecFile(agentBuilderReplayExamplePath(t)); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}
			got, err := vm.Get("agent_builder_replay_summary")
			if err != nil {
				t.Fatalf("Get agent_builder_replay_summary: %v", err)
			}
			if got != want {
				t.Fatalf("agent_builder_replay_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func TestFinRobotAgentBuilderReplayFixturesCoverBuilderParity(t *testing.T) {
	base := agentBuilderReplayFixtureDir(t)
	var config agentBuilderConfigFixture
	decodeAgentBuilderFixture(t, filepath.Join(base, "builder_config_fixture.json"), &config)
	if config.SchemaVersion != 1 || config.ID != "finrobot-agent-builder-demo-config-fixture" || config.Source != "agent_builder_demo.py" {
		t.Fatalf("config header = %#v", config)
	}
	if !config.ProviderFree || config.LiveNetwork || config.LiveModel || config.CredentialsRequired || config.RealDependencyImports {
		t.Fatalf("config must stay provider-free with no live model/network/credentials/imports: %#v", config)
	}
	if !config.Builder.SaveLoadRoundtrip || config.Builder.GroupChat.MaxRound != 4 {
		t.Fatalf("builder save/load or max_round mismatch: %#v", config.Builder)
	}
	if len(config.Agents) != 4 {
		t.Fatalf("agents = %d, want 4", len(config.Agents))
	}

	roles := map[string]int{}
	toolsByAgent := map[string][]string{}
	for _, agent := range config.Agents {
		if agent.ID == "" || agent.DisplayName == "" || agent.Metadata.Seniority == "" || agent.Metadata.Responsibility == "" {
			t.Fatalf("agent metadata incomplete: %#v", agent)
		}
		roles[agent.Role]++
		toolsByAgent[agent.ID] = agent.Tools
	}
	if roles["leader"] != 1 || roles["member"] != 3 {
		t.Fatalf("roles = %#v, want one leader and three members", roles)
	}
	wantTools := map[string][]string{
		"leader":              {"assign_sections", "save_config"},
		"fundamental_analyst": {"fetch_fundamentals"},
		"news_analyst":        {"fetch_news"},
		"risk_analyst":        {"fetch_risks"},
	}
	if !reflect.DeepEqual(toolsByAgent, wantTools) {
		t.Fatalf("tool assignment = %#v, want %#v", toolsByAgent, wantTools)
	}
}

func TestFinRobotAgentBuilderReplayTraceAndRecordsAreOffline(t *testing.T) {
	base := agentBuilderReplayFixtureDir(t)
	var trace agentBuilderTraceFixture
	decodeAgentBuilderFixture(t, filepath.Join(base, "provider_free_trace_fixture.json"), &trace)
	if trace.SchemaVersion != 1 || !trace.ProviderFree || trace.LiveNetwork || trace.LiveModel || trace.CredentialsRequired {
		t.Fatalf("trace header must be provider-free/offline: %#v", trace)
	}
	if trace.GroupChat.MaxRound != 4 || len(trace.GroupChat.Rounds) != trace.GroupChat.MaxRound {
		t.Fatalf("trace rounds = %d max_round = %d", len(trace.GroupChat.Rounds), trace.GroupChat.MaxRound)
	}
	wantRoundTools := []string{"assign_sections", "fetch_fundamentals", "fetch_news", "fetch_risks"}
	for i, round := range trace.GroupChat.Rounds {
		if round.Round != i+1 || !round.ProviderFree || round.Tool != wantRoundTools[i] || round.EvidenceID == "" {
			t.Fatalf("round %d = %#v", i+1, round)
		}
		if i == 0 && round.Role != "leader" {
			t.Fatalf("first round should be leader: %#v", round)
		}
		if i > 0 && round.Role != "member" {
			t.Fatalf("member round %d role = %q", i+1, round.Role)
		}
	}

	var records agentBuilderReplayRecordsFixture
	decodeAgentBuilderFixture(t, filepath.Join(base, "replay_records_fixture.json"), &records)
	if records.SchemaVersion != 1 || !records.ProviderFree || len(records.Records) != 1 {
		t.Fatalf("records header = %#v", records)
	}
	record := records.Records[0]
	if record.Mode != "local_fixture" || len(record.Models) != 0 || len(record.NetworkRequests) != 0 || len(record.Credentials) != 0 ||
		record.LoadedTurns != 0 || record.ReplayedTurns != 0 {
		t.Fatalf("replay record must not require model/network/credentials: %#v", record)
	}
	if !reflect.DeepEqual(record.ToolCalls, []string{"assign_sections", "save_config", "fetch_fundamentals", "fetch_news", "fetch_risks"}) {
		t.Fatalf("tool calls = %#v", record.ToolCalls)
	}
}

func TestFinRobotAgentBuilderReplaySourceHasNoLiveProviderSurface(t *testing.T) {
	data, err := os.ReadFile(agentBuilderReplayExamplePath(t))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, want := range []string{
		"agent_builder_demo.py",
		"provider_free: true",
		"live_network: false",
		"live_model: false",
		"credentials_required: false",
		"real_dependency_imports: false",
		"max_round: 4",
		"save_builder_config",
		"load_builder_config",
		"metadata:",
		"llm.dispatch",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("agent_builder_replay.leia missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"OPENAI_",
		"ANTHROPIC_",
		"FMP_API_KEY",
		"FINNHUB_TOKEN",
		"env:",
		"http.get",
		"http.post",
		"live_network: true",
		"live_model: true",
		"credentials_required: true",
		"real_dependency_imports: true",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("agent_builder_replay.leia contains forbidden live/provider surface %q", forbidden)
		}
	}
}

func agentBuilderReplayExamplePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "agent_builder_replay.leia")
}

func agentBuilderReplayFixtureDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "agent_builder_replay")
}

func decodeAgentBuilderFixture(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
