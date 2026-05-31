package url

import neturl "net/url"

type Parsed struct {
	Scheme   string
	Host     string
	Port     string
	Path     string
	Fragment string
	Raw      string
	User     string
	HasUser  bool
	Password *string
	Query    map[string]string
}

type Parts struct {
	Scheme   string
	Host     string
	Port     string
	Path     string
	Fragment string
	User     string
	HasUser  bool
	Password *string
	Query    map[string]string
}

func Parse(s string) (Parsed, error) {
	u, err := neturl.Parse(s)
	if err != nil {
		return Parsed{}, err
	}

	parsed := Parsed{
		Scheme:   u.Scheme,
		Host:     u.Hostname(),
		Port:     u.Port(),
		Path:     u.Path,
		Fragment: u.Fragment,
		Raw:      u.String(),
		Query:    firstQueryValues(u.Query()),
	}
	if u.User != nil {
		parsed.HasUser = true
		parsed.User = u.User.Username()
		if pwd, ok := u.User.Password(); ok {
			parsed.Password = &pwd
		}
	}
	return parsed, nil
}

func Build(parts Parts) string {
	u := &neturl.URL{
		Scheme:   parts.Scheme,
		Path:     parts.Path,
		Fragment: parts.Fragment,
	}

	host := parts.Host
	if parts.Port != "" {
		host += ":" + parts.Port
	}
	u.Host = host

	if parts.HasUser {
		if parts.Password != nil {
			u.User = neturl.UserPassword(parts.User, *parts.Password)
		} else {
			u.User = neturl.User(parts.User)
		}
	}

	if len(parts.Query) > 0 {
		u.RawQuery = QueryEncode(parts.Query)
	}

	return u.String()
}

func Encode(s string) string {
	return neturl.QueryEscape(s)
}

func Decode(s string) (string, error) {
	return neturl.QueryUnescape(s)
}

func QueryEncode(values map[string]string) string {
	q := neturl.Values{}
	for k, v := range values {
		q.Set(k, v)
	}
	return q.Encode()
}

func QueryDecode(s string) (map[string]string, error) {
	vals, err := neturl.ParseQuery(s)
	if err != nil {
		return nil, err
	}
	return firstQueryValues(vals), nil
}

func Join(base, ref string) (string, error) {
	baseURL, err := neturl.Parse(base)
	if err != nil {
		return "", err
	}
	refURL, err := neturl.Parse(ref)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(refURL).String(), nil
}

func IsValid(s string) bool {
	u, err := neturl.Parse(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func Host(s string) (string, error) {
	u, err := neturl.Parse(s)
	if err != nil {
		return "", err
	}
	return u.Hostname(), nil
}

func Path(s string) (string, error) {
	u, err := neturl.Parse(s)
	if err != nil {
		return "", err
	}
	return u.Path, nil
}

func firstQueryValues(vals neturl.Values) map[string]string {
	query := make(map[string]string, len(vals))
	for k, values := range vals {
		if len(values) > 0 {
			query[k] = values[0]
		}
	}
	return query
}
