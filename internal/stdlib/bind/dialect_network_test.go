package bind

import "testing"

func TestDialectNetworkParsesIPCIDRAndHostPort(t *testing.T) {
	interp := runWithLib(t, `
		ip := ipaddr`+"`"+`10.2.3.4`+"`"+`
		ip_in_net := dialect.eval("ipaddr", "10.2.3.4", {cidr: "10.2.0.0/16"})
		ip_out_net := dialect.eval("ipaddr", "10.3.3.4", {cidr: "10.2.0.0/16"})
		bad_ip, bad_ip_err := dialect.eval("ipaddr", "not-an-ip")
		net := cidr`+"`"+`10.2.0.7/16`+"`"+`
		net_contains := dialect.eval("cidr", "10.2.0.0/16", {ip: "10.2.3.4"})
		net_misses := dialect.eval("cidr", "10.2.0.0/16", {ip: "10.3.3.4"})
		bad_cidr, bad_cidr_err := dialect.eval("cidr", "10.2.0.0")
		hp := hostport`+"`"+`[2001:db8::1]:443`+"`"+`
		hp_joined := dialect.eval("hostport", {host: "2001:db8::1", port: "443"}, {mode: "encode"})
		bad_hp, bad_hp_err := dialect.eval("hostport", "2001:db8::1:443")
	`, "dialect", BuildDialect(HostOptions{}, nil))

	ip := interp.GetGlobal("ip").Table()
	if got := ip.RawGetString("text").Str(); got != "10.2.3.4" {
		t.Fatalf("ip text = %q, want 10.2.3.4", got)
	}
	if got := ip.RawGetString("version").Int(); got != 4 {
		t.Fatalf("ip version = %d, want 4", got)
	}
	if !ip.RawGetString("private").Bool() || !ip.RawGetString("global_unicast").Bool() {
		t.Fatalf("ip flags private=%v global_unicast=%v, want true true", ip.RawGetString("private"), ip.RawGetString("global_unicast"))
	}
	if !interp.GetGlobal("ip_in_net").Table().RawGetString("contains").Bool() {
		t.Fatalf("ip_in_net contains = false, want true")
	}
	if interp.GetGlobal("ip_out_net").Table().RawGetString("contains").Bool() {
		t.Fatalf("ip_out_net contains = true, want false")
	}
	if !interp.GetGlobal("bad_ip").IsNil() || !interp.GetGlobal("bad_ip_err").IsString() {
		t.Fatalf("bad ip = %v err %v, want nil error string", interp.GetGlobal("bad_ip"), interp.GetGlobal("bad_ip_err"))
	}

	network := interp.GetGlobal("net").Table()
	if got := network.RawGetString("masked").Str(); got != "10.2.0.0/16" {
		t.Fatalf("masked network = %q, want 10.2.0.0/16", got)
	}
	if got := network.RawGetString("bits").Int(); got != 16 {
		t.Fatalf("network bits = %d, want 16", got)
	}
	if !interp.GetGlobal("net_contains").Table().RawGetString("contains").Bool() {
		t.Fatalf("net_contains contains = false, want true")
	}
	if interp.GetGlobal("net_misses").Table().RawGetString("contains").Bool() {
		t.Fatalf("net_misses contains = true, want false")
	}
	if !interp.GetGlobal("bad_cidr").IsNil() || !interp.GetGlobal("bad_cidr_err").IsString() {
		t.Fatalf("bad cidr = %v err %v, want nil error string", interp.GetGlobal("bad_cidr"), interp.GetGlobal("bad_cidr_err"))
	}

	hostport := interp.GetGlobal("hp").Table()
	if got := hostport.RawGetString("host").Str(); got != "2001:db8::1" {
		t.Fatalf("hostport host = %q, want 2001:db8::1", got)
	}
	if got := hostport.RawGetString("port").Str(); got != "443" {
		t.Fatalf("hostport port = %q, want 443", got)
	}
	if got := interp.GetGlobal("hp_joined").Str(); got != "[2001:db8::1]:443" {
		t.Fatalf("joined hostport = %q, want [2001:db8::1]:443", got)
	}
	if !interp.GetGlobal("bad_hp").IsNil() || !interp.GetGlobal("bad_hp_err").IsString() {
		t.Fatalf("bad hostport = %v err %v, want nil error string", interp.GetGlobal("bad_hp"), interp.GetGlobal("bad_hp_err"))
	}
}
