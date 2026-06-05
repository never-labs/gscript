package bind

import "testing"

func TestDialectProtocolNetworkParsesIPCIDRAndHostPort(t *testing.T) {
	interp := runWithLib(t, `
		ip := ipaddr`+"`"+`10.2.3.4`+"`"+`
		ip_in_net := dialect.eval("ipaddr", "10.2.3.4", {cidr: "10.2.0.0/16"})
		ip_out_net := dialect.eval("ipaddr", "10.3.3.4", {cidr: "10.2.0.0/16"})
		bad_ip, bad_ip_err := dialect.eval("ipaddr", "not-an-ip")
		bad_ip_cidr, bad_ip_cidr_err := dialect.eval("ipaddr", "10.2.3.4", {cidr: "not-a-cidr"})
		net := cidr`+"`"+`10.2.0.7/16`+"`"+`
		net_contains := dialect.eval("cidr", "10.2.0.0/16", {ip: "10.2.3.4"})
		net_misses := dialect.eval("cidr", "10.2.0.0/16", {ip: "10.3.3.4"})
		v6_contains := dialect.eval("cidr", "2001:db8::/32", {ip: "2001:db8::42"})
		v6_misses := dialect.eval("cidr", "2001:db8::/32", {ip: "2001:db9::1"})
		any_v4_contains := dialect.eval("cidr", "0.0.0.0/0", {ip: "203.0.113.9"})
		exact_v4_contains := dialect.eval("cidr", "192.168.1.7/32", {ip: "192.168.1.7"})
		exact_v4_misses := dialect.eval("cidr", "192.168.1.7/32", {ip: "192.168.1.8"})
		any_v6_contains := dialect.eval("cidr", "::/0", {ip: "2001:db8::1"})
		exact_v6_contains := dialect.eval("cidr", "2001:db8::1/128", {ip: "2001:db8::1"})
		exact_v6_misses := dialect.eval("cidr", "2001:db8::1/128", {ip: "2001:db8::2"})
		bad_cidr, bad_cidr_err := dialect.eval("cidr", "10.2.0.0")
		bad_cidr_ip, bad_cidr_ip_err := dialect.eval("cidr", "2001:db8::/32", {ip: "not-an-ip"})
		zone_ip := ipaddr`+"`"+`fe80::1%eth0`+"`"+`
		hp := hostport`+"`"+`[2001:db8::1]:443`+"`"+`
		zone_hp := hostport`+"`"+`[fe80::1%lo0]:8080`+"`"+`
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
	if !interp.GetGlobal("bad_ip_cidr").IsNil() || !interp.GetGlobal("bad_ip_cidr_err").IsString() {
		t.Fatalf("bad ip cidr = %v err %v, want nil error string", interp.GetGlobal("bad_ip_cidr"), interp.GetGlobal("bad_ip_cidr_err"))
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
	if !interp.GetGlobal("v6_contains").Table().RawGetString("contains").Bool() {
		t.Fatalf("v6_contains contains = false, want true")
	}
	if interp.GetGlobal("v6_misses").Table().RawGetString("contains").Bool() {
		t.Fatalf("v6_misses contains = true, want false")
	}
	for _, name := range []string{"any_v4_contains", "exact_v4_contains", "any_v6_contains", "exact_v6_contains"} {
		if !interp.GetGlobal(name).Table().RawGetString("contains").Bool() {
			t.Fatalf("%s contains = false, want true", name)
		}
	}
	for _, name := range []string{"exact_v4_misses", "exact_v6_misses"} {
		if interp.GetGlobal(name).Table().RawGetString("contains").Bool() {
			t.Fatalf("%s contains = true, want false", name)
		}
	}
	if !interp.GetGlobal("bad_cidr").IsNil() || !interp.GetGlobal("bad_cidr_err").IsString() {
		t.Fatalf("bad cidr = %v err %v, want nil error string", interp.GetGlobal("bad_cidr"), interp.GetGlobal("bad_cidr_err"))
	}
	if !interp.GetGlobal("bad_cidr_ip").IsNil() || !interp.GetGlobal("bad_cidr_ip_err").IsString() {
		t.Fatalf("bad cidr ip = %v err %v, want nil error string", interp.GetGlobal("bad_cidr_ip"), interp.GetGlobal("bad_cidr_ip_err"))
	}
	zoneIP := interp.GetGlobal("zone_ip").Table()
	if got := zoneIP.RawGetString("zone").Str(); got != "eth0" {
		t.Fatalf("zone ip zone = %q, want eth0", got)
	}
	if !zoneIP.RawGetString("is6").Bool() || !zoneIP.RawGetString("link_local_unicast").Bool() {
		t.Fatalf("zone ip flags is6=%v link_local=%v, want true true", zoneIP.RawGetString("is6"), zoneIP.RawGetString("link_local_unicast"))
	}

	hostport := interp.GetGlobal("hp").Table()
	if got := hostport.RawGetString("host").Str(); got != "2001:db8::1" {
		t.Fatalf("hostport host = %q, want 2001:db8::1", got)
	}
	if got := hostport.RawGetString("port").Str(); got != "443" {
		t.Fatalf("hostport port = %q, want 443", got)
	}
	zoneHostport := interp.GetGlobal("zone_hp").Table()
	if got := zoneHostport.RawGetString("host").Str(); got != "fe80::1%lo0" {
		t.Fatalf("zone hostport host = %q, want fe80::1%%lo0", got)
	}
	if got := zoneHostport.RawGetString("addr").Table().RawGetString("zone").Str(); got != "lo0" {
		t.Fatalf("zone hostport addr zone = %q, want lo0", got)
	}
	if got := interp.GetGlobal("hp_joined").Str(); got != "[2001:db8::1]:443" {
		t.Fatalf("joined hostport = %q, want [2001:db8::1]:443", got)
	}
	if !interp.GetGlobal("bad_hp").IsNil() || !interp.GetGlobal("bad_hp_err").IsString() {
		t.Fatalf("bad hostport = %v err %v, want nil error string", interp.GetGlobal("bad_hp"), interp.GetGlobal("bad_hp_err"))
	}
}

func TestDialectHostPortModeAliasesAndUnknownMode(t *testing.T) {
	interp := runWithLib(t, `
		parsed := dialect.eval("hostport", "example.com:443", {mode: "parse"})
		decoded := dialect.eval("hostport", "example.com:443", {mode: "decode"})
		joined := dialect.eval("hostport", {host: "example.com", port: "443"}, {mode: "join"})
		formatted := dialect.eval("hostport", {host: "example.com", port: "443"}, {mode: "format"})
		missing_host, missing_host_err := dialect.eval("hostport", {port: "443"}, {mode: "encode"})
		missing_port, missing_port_err := dialect.eval("hostport", {host: "example.com"}, {mode: "encode"})
		bad, bad_err := dialect.eval("hostport", "example.com:443", {mode: "bogus"})
	`, "dialect", BuildDialect(HostOptions{}, nil))

	if got := interp.GetGlobal("parsed").Table().RawGetString("host").Str(); got != "example.com" {
		t.Fatalf("parsed host = %q, want example.com", got)
	}
	if got := interp.GetGlobal("decoded").Table().RawGetString("port").Str(); got != "443" {
		t.Fatalf("decoded port = %q, want 443", got)
	}
	if got := interp.GetGlobal("joined").Str(); got != "example.com:443" {
		t.Fatalf("joined = %q, want example.com:443", got)
	}
	if got := interp.GetGlobal("formatted").Str(); got != "example.com:443" {
		t.Fatalf("formatted = %q, want example.com:443", got)
	}
	assertDialectModeError(t, interp.GetGlobal("missing_host"), interp.GetGlobal("missing_host_err"), "hostport dialect: host and port required for encode")
	assertDialectModeError(t, interp.GetGlobal("missing_port"), interp.GetGlobal("missing_port_err"), "hostport dialect: host and port required for encode")
	assertDialectModeError(t, interp.GetGlobal("bad"), interp.GetGlobal("bad_err"), "hostport dialect: unknown mode")
}
