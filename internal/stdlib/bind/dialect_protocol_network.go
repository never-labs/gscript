package bind

import (
	"net"
	"net/netip"
)

func registerDialectProtocolNetwork(register dialectRegisterFunc) {
	register([]string{"ipaddr"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectIPAddr(body, options)
		},
	})
	register([]string{"cidr"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectCIDR(body, options)
		},
	})
	register([]string{"hostport"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectHostPort(body, options)
		},
	})
}

func dialectIPAddr(body Value, opts *Table) ([]Value, error) {
	addrText := body.String()
	if body.IsTable() {
		if v := body.Table().RawGetString("addr"); v.IsString() {
			addrText = v.Str()
		} else if v := body.Table().RawGetString("ip"); v.IsString() {
			addrText = v.Str()
		}
	}
	addr, err := netip.ParseAddr(addrText)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	out := tableForIPAddr(addr)
	if opts != nil {
		if cidr := firstStringField(opts, "cidr", "prefix", "network"); cidr != "" {
			prefix, err := netip.ParsePrefix(cidr)
			if err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			out.RawSetString("contains", BoolValue(prefix.Contains(addr)))
			out.RawSetString("network", StringValue(prefix.String()))
		}
	}
	return []Value{TableValue(out)}, nil
}

func dialectCIDR(body Value, opts *Table) ([]Value, error) {
	prefixText := body.String()
	if body.IsTable() {
		if v := body.Table().RawGetString("cidr"); v.IsString() {
			prefixText = v.Str()
		} else if v := body.Table().RawGetString("prefix"); v.IsString() {
			prefixText = v.Str()
		}
	}
	prefix, err := netip.ParsePrefix(prefixText)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	out := tableForCIDR(prefix)
	if opts != nil {
		if ip := firstStringField(opts, "ip", "addr", "contains"); ip != "" {
			addr, err := netip.ParseAddr(ip)
			if err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			out.RawSetString("contains", BoolValue(prefix.Contains(addr)))
			out.RawSetString("contains_ip", StringValue(addr.String()))
		}
	}
	return []Value{TableValue(out)}, nil
}

func dialectHostPort(body Value, opts *Table) ([]Value, error) {
	mode := dialectMode(opts)
	if !dialectModeAllowed(mode, "", "parse", "decode", "encode", "join", "format") {
		return dialectUnknownMode("hostport", mode)
	}
	if body.IsTable() || mode == "encode" || mode == "join" || mode == "format" {
		host := ""
		port := ""
		if body.IsTable() {
			host = firstStringField(body.Table(), "host", "addr", "ip")
			port = firstStringField(body.Table(), "port")
		}
		if opts != nil {
			if host == "" {
				host = firstStringField(opts, "host", "addr", "ip")
			}
			if port == "" {
				port = firstStringField(opts, "port")
			}
		}
		if host == "" || port == "" {
			return []Value{NilValue(), StringValue("hostport dialect: host and port required for encode")}, nil
		}
		joined := net.JoinHostPort(host, port)
		return []Value{StringValue(joined)}, nil
	}

	host, port, err := net.SplitHostPort(body.String())
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	out := NewTable()
	out.RawSetString("text", StringValue(net.JoinHostPort(host, port)))
	out.RawSetString("host", StringValue(host))
	out.RawSetString("port", StringValue(port))
	if addr, err := netip.ParseAddr(host); err == nil {
		out.RawSetString("addr", TableValue(tableForIPAddr(addr)))
	}
	return []Value{TableValue(out)}, nil
}

func tableForIPAddr(addr netip.Addr) *Table {
	out := NewTable()
	out.RawSetString("text", StringValue(addr.String()))
	out.RawSetString("ip", StringValue(addr.String()))
	out.RawSetString("zone", StringValue(addr.Zone()))
	out.RawSetString("version", IntValue(ipVersion(addr)))
	out.RawSetString("is4", BoolValue(addr.Is4()))
	out.RawSetString("is6", BoolValue(addr.Is6()))
	out.RawSetString("loopback", BoolValue(addr.IsLoopback()))
	out.RawSetString("private", BoolValue(addr.IsPrivate()))
	out.RawSetString("unspecified", BoolValue(addr.IsUnspecified()))
	out.RawSetString("multicast", BoolValue(addr.IsMulticast()))
	out.RawSetString("global_unicast", BoolValue(addr.IsGlobalUnicast()))
	out.RawSetString("link_local_unicast", BoolValue(addr.IsLinkLocalUnicast()))
	return out
}

func tableForCIDR(prefix netip.Prefix) *Table {
	masked := prefix.Masked()
	out := NewTable()
	out.RawSetString("text", StringValue(prefix.String()))
	out.RawSetString("cidr", StringValue(prefix.String()))
	out.RawSetString("masked", StringValue(masked.String()))
	out.RawSetString("addr", StringValue(prefix.Addr().String()))
	out.RawSetString("network", StringValue(masked.Addr().String()))
	out.RawSetString("bits", IntValue(int64(prefix.Bits())))
	out.RawSetString("version", IntValue(ipVersion(prefix.Addr())))
	out.RawSetString("is4", BoolValue(prefix.Addr().Is4()))
	out.RawSetString("is6", BoolValue(prefix.Addr().Is6()))
	return out
}

func ipVersion(addr netip.Addr) int64 {
	switch {
	case addr.Is4():
		return 4
	case addr.Is6():
		return 6
	default:
		return 0
	}
}

func firstStringField(tbl *Table, names ...string) string {
	if tbl == nil {
		return ""
	}
	for _, name := range names {
		if v := tbl.RawGetString(name); v.IsString() {
			return v.Str()
		}
	}
	return ""
}
