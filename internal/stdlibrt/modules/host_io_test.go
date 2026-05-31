package modules

import (
	"strings"
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func TestBuildFSWithPolicyReadGate(t *testing.T) {
	lib := BuildFSWithPolicy(HostOptions{
		FilesystemRead:  func() bool { return false },
		FilesystemWrite: func() bool { return true },
	})
	fn := lib.RawGetString("readfile").GoFunction()
	_, err := fn.Fn([]runtime.Value{runtime.StringValue("missing.txt")})
	if err == nil || !strings.Contains(err.Error(), "filesystem read access disabled") {
		t.Fatalf("fs.readfile err = %v, want filesystem read access disabled", err)
	}
}

func TestBuildNetWithPolicyNetworkGate(t *testing.T) {
	lib := BuildNet(HostOptions{
		NetworkAllowed: func() bool { return false },
	})
	fn := lib.RawGetString("get").GoFunction()
	_, err := fn.Fn([]runtime.Value{runtime.StringValue("http://127.0.0.1:1")})
	if err == nil || !strings.Contains(err.Error(), "network access disabled") {
		t.Fatalf("net.get err = %v, want network access disabled", err)
	}
}

func TestBuildOSWithPolicyEnvironmentAllowlist(t *testing.T) {
	t.Setenv("GSCRIPT_STDLIBRT_HOST_IO_ALLOWED", "yes")
	lib := BuildOSWithPolicy(HostOptions{
		EnvironmentRead: func() bool { return true },
		EnvironmentAllowed: func(name string) bool {
			return name == "GSCRIPT_STDLIBRT_HOST_IO_ALLOWED"
		},
	})
	getenv := lib.RawGetString("getenv").GoFunction()
	got, err := getenv.Fn([]runtime.Value{runtime.StringValue("GSCRIPT_STDLIBRT_HOST_IO_ALLOWED")})
	if err != nil || len(got) != 1 || got[0].Str() != "yes" {
		t.Fatalf("allowed getenv = %v, %v; want yes", got, err)
	}
	blocked, err := getenv.Fn([]runtime.Value{runtime.StringValue("PATH")})
	if err != nil || len(blocked) != 1 || !blocked[0].IsNil() {
		t.Fatalf("blocked getenv = %v, %v; want nil", blocked, err)
	}
}
