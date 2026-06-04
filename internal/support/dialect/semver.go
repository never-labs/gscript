package dialect

import (
	"fmt"
	"strconv"
	"strings"
)

type SemVer struct {
	Major      int64
	Minor      int64
	Patch      int64
	Prerelease []string
	Build      []string
}

func ParseSemVer(src string) (SemVer, error) {
	if src == "" {
		return SemVer{}, &ParseError{Kind: "semver", Message: "empty version"}
	}
	coreAndRest := src
	buildText := ""
	if before, after, ok := strings.Cut(coreAndRest, "+"); ok {
		if after == "" {
			return SemVer{}, &ParseError{Kind: "semver", Message: "empty build metadata"}
		}
		if strings.Contains(after, "+") {
			return SemVer{}, &ParseError{Kind: "semver", Message: "multiple build metadata separators"}
		}
		coreAndRest = before
		buildText = after
	}
	coreText := coreAndRest
	prereleaseText := ""
	if before, after, ok := strings.Cut(coreAndRest, "-"); ok {
		if after == "" {
			return SemVer{}, &ParseError{Kind: "semver", Message: "empty prerelease"}
		}
		coreText = before
		prereleaseText = after
	}
	core := strings.Split(coreText, ".")
	if len(core) != 3 {
		return SemVer{}, &ParseError{Kind: "semver", Message: "core version must be major.minor.patch"}
	}
	major, err := parseSemVerCoreNumber("major", core[0])
	if err != nil {
		return SemVer{}, err
	}
	minor, err := parseSemVerCoreNumber("minor", core[1])
	if err != nil {
		return SemVer{}, err
	}
	patch, err := parseSemVerCoreNumber("patch", core[2])
	if err != nil {
		return SemVer{}, err
	}
	prerelease, err := parseSemVerIdentifiers("prerelease", prereleaseText, true)
	if err != nil {
		return SemVer{}, err
	}
	build, err := parseSemVerIdentifiers("build", buildText, false)
	if err != nil {
		return SemVer{}, err
	}
	return SemVer{Major: major, Minor: minor, Patch: patch, Prerelease: prerelease, Build: build}, nil
}

func FormatSemVer(v SemVer) (string, error) {
	if v.Major < 0 || v.Minor < 0 || v.Patch < 0 {
		return "", &ParseError{Kind: "semver", Message: "core numbers must be non-negative"}
	}
	if err := validateSemVerIdentifiers("prerelease", v.Prerelease, true); err != nil {
		return "", err
	}
	if err := validateSemVerIdentifiers("build", v.Build, false); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(strconv.FormatInt(v.Major, 10))
	b.WriteByte('.')
	b.WriteString(strconv.FormatInt(v.Minor, 10))
	b.WriteByte('.')
	b.WriteString(strconv.FormatInt(v.Patch, 10))
	if len(v.Prerelease) > 0 {
		b.WriteByte('-')
		b.WriteString(strings.Join(v.Prerelease, "."))
	}
	if len(v.Build) > 0 {
		b.WriteByte('+')
		b.WriteString(strings.Join(v.Build, "."))
	}
	return b.String(), nil
}

func parseSemVerCoreNumber(name, src string) (int64, error) {
	if src == "" {
		return 0, &ParseError{Kind: "semver", Message: "empty " + name + " number"}
	}
	if len(src) > 1 && src[0] == '0' {
		return 0, &ParseError{Kind: "semver", Message: name + " number has leading zero"}
	}
	for i := 0; i < len(src); i++ {
		if src[i] < '0' || src[i] > '9' {
			return 0, &ParseError{Kind: "semver", Message: name + " number must contain digits only"}
		}
	}
	n, err := strconv.ParseInt(src, 10, 64)
	if err != nil {
		return 0, &ParseError{Kind: "semver", Message: name + " number out of range"}
	}
	return n, nil
}

func parseSemVerIdentifiers(kind, src string, rejectNumericLeadingZero bool) ([]string, error) {
	if src == "" {
		return nil, nil
	}
	ids := strings.Split(src, ".")
	if err := validateSemVerIdentifiers(kind, ids, rejectNumericLeadingZero); err != nil {
		return nil, err
	}
	return ids, nil
}

func validateSemVerIdentifiers(kind string, ids []string, rejectNumericLeadingZero bool) error {
	for _, id := range ids {
		if id == "" {
			return &ParseError{Kind: "semver", Message: kind + " identifier must not be empty"}
		}
		allDigits := true
		for i := 0; i < len(id); i++ {
			c := id[i]
			if (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '-' {
				if c < '0' || c > '9' {
					allDigits = false
				}
				continue
			}
			return &ParseError{Kind: "semver", Message: fmt.Sprintf("%s identifier %q contains invalid character", kind, id)}
		}
		if rejectNumericLeadingZero && allDigits && len(id) > 1 && id[0] == '0' {
			return &ParseError{Kind: "semver", Message: kind + " numeric identifier has leading zero"}
		}
	}
	return nil
}
