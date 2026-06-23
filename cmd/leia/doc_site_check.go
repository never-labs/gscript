package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type docSiteReport struct {
	SchemaVersion      int                    `json:"schema_version"`
	Status             string                 `json:"status"`
	SiteDir            string                 `json:"site_dir"`
	HTMLFileCount      int                    `json:"html_file_count"`
	LocalLinkCount     int                    `json:"local_link_count"`
	AssetRefCount      int                    `json:"asset_ref_count"`
	FragmentCheckCount int                    `json:"fragment_check_count"`
	FailureKindCount   int                    `json:"failure_kind_count"`
	FailureCount       int                    `json:"failure_count"`
	FailureKinds       []string               `json:"failure_kinds"`
	FailureDetails     []docSiteFailureDetail `json:"failure_details"`
}

type docSiteFailureDetail struct {
	Kind      string `json:"kind"`
	Path      string `json:"path,omitempty"`
	Attribute string `json:"attribute,omitempty"`
	Value     string `json:"value,omitempty"`
	Message   string `json:"message"`
	Target    string `json:"target,omitempty"`
	Fragment  string `json:"fragment,omitempty"`
}

type docSiteParser struct {
	Refs    []docSiteRef
	Anchors map[string]bool
}

type docSiteRef struct {
	Tag   string
	Attr  string
	Value string
}

func runDocSiteCheckCommand(args []string, outw, errw io.Writer) int {
	root, err := findCLIRepoRootFromCWD()
	if err != nil {
		fmt.Fprintf(errw, "leia doc site-check: %v\n", err)
		return 1
	}
	fs := flag.NewFlagSet("doc site-check", flag.ContinueOnError)
	fs.SetOutput(errw)
	siteDir := fs.String("site-dir", filepath.Join(root, "_site"), "rendered site directory to inspect")
	jsonOut := fs.Bool("json", false, "print a machine-readable site report")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: leia doc site-check [--site-dir DIR] [--json]")
		return 2
	}
	report := checkDocSite(root, *siteDir)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "leia doc site-check: %v\n", err)
			return 1
		}
	} else if report.FailureCount > 0 {
		fmt.Fprintf(outw, "site_check.sh: %d issue(s) in %s\n", report.FailureCount, report.SiteDir)
		for _, item := range report.FailureDetails {
			fmt.Fprintf(outw, "  - %s: %s (%s: %s)\n", item.Kind, item.Message, item.Path, item.Value)
		}
	} else {
		fmt.Fprintf(outw, "site_check.sh: checked %d HTML files, %d local links, %d asset references, %d fragment anchors.\n", report.HTMLFileCount, report.LocalLinkCount, report.AssetRefCount, report.FragmentCheckCount)
	}
	if report.FailureCount > 0 {
		return 1
	}
	return 0
}

func checkDocSite(root, siteDir string) docSiteReport {
	rootAbs, _ := filepath.Abs(root)
	siteAbs, _ := filepath.Abs(siteDir)
	report := docSiteReport{
		SchemaVersion:  1,
		Status:         "pass",
		SiteDir:        docSiteRel(rootAbs, siteAbs, siteAbs),
		FailureKinds:   []string{},
		FailureDetails: []docSiteFailureDetail{},
	}
	parserCache := map[string]docSiteParser{}
	var htmlFiles []string
	if info, err := os.Stat(siteAbs); err == nil && info.IsDir() {
		_ = filepath.WalkDir(siteAbs, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".html") {
				htmlFiles = append(htmlFiles, path)
			}
			return nil
		})
		slices.Sort(htmlFiles)
	} else {
		report.FailureDetails = append(report.FailureDetails, docSiteFailureDetail{
			Kind:    "missing_site_dir",
			Path:    docSiteRel(rootAbs, siteAbs, siteAbs),
			Message: fmt.Sprintf("rendered site directory does not exist: %s", siteAbs),
		})
	}
	report.HTMLFileCount = len(htmlFiles)
	parseHTML := func(path string) docSiteParser {
		if cached, ok := parserCache[path]; ok {
			return cached
		}
		parser := parseDocSiteHTML(readFileBestEffort(path))
		parserCache[path] = parser
		return parser
	}
	for _, htmlFile := range htmlFiles {
		parser := parseHTML(htmlFile)
		for _, ref := range parser.Refs {
			if ref.Value == "" || docSiteIsExternal(ref.Value) {
				continue
			}
			target, fragment := docSiteTargetFor(htmlFile, ref.Value, siteAbs)
			parsed, _ := url.Parse(ref.Value)
			parsedPath := ""
			if parsed != nil {
				parsedPath = parsed.Path
			}
			isAnchor := ref.Tag == "a" || filepath.Ext(target) == ".html" || parsedPath == "" || strings.HasSuffix(parsedPath, "/")
			if isAnchor {
				report.LocalLinkCount++
			} else {
				report.AssetRefCount++
			}
			if !docSiteWithin(siteAbs, target) {
				report.FailureDetails = append(report.FailureDetails, docSiteFailure(rootAbs, siteAbs, "link_escape", htmlFile, ref.Attr, ref.Value, "local reference escapes rendered site", target, fragment))
				continue
			}
			if _, err := os.Stat(target); err != nil {
				report.FailureDetails = append(report.FailureDetails, docSiteFailure(rootAbs, siteAbs, "missing_target", htmlFile, ref.Attr, ref.Value, "local reference target is missing", target, fragment))
				continue
			}
			if fragment != "" && filepath.Ext(target) == ".html" {
				report.FragmentCheckCount++
				targetParser := parseHTML(target)
				if !targetParser.Anchors[fragment] {
					report.FailureDetails = append(report.FailureDetails, docSiteFailure(rootAbs, siteAbs, "missing_anchor", htmlFile, ref.Attr, ref.Value, "local fragment anchor is missing", target, fragment))
				}
			}
		}
	}
	seenKinds := map[string]bool{}
	for _, item := range report.FailureDetails {
		if !seenKinds[item.Kind] {
			seenKinds[item.Kind] = true
			report.FailureKinds = append(report.FailureKinds, item.Kind)
		}
	}
	report.FailureKindCount = len(report.FailureKinds)
	report.FailureCount = len(report.FailureDetails)
	if report.FailureCount > 0 {
		report.Status = "issues"
	}
	return report
}

func parseDocSiteHTML(source string) docSiteParser {
	parser := docSiteParser{Anchors: map[string]bool{}}
	tagRe := regexp.MustCompile(`(?is)<\s*([a-z][a-z0-9:-]*)\b([^>]*)>`)
	attrRe := regexp.MustCompile(`(?is)([a-z_:][a-z0-9_.:-]*)\s*(?:=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>` + "`" + `]+)))?`)
	for _, tagMatch := range tagRe.FindAllStringSubmatch(source, -1) {
		tag := strings.ToLower(tagMatch[1])
		attrs := map[string]string{}
		for _, attrMatch := range attrRe.FindAllStringSubmatch(tagMatch[2], -1) {
			name := strings.ToLower(attrMatch[1])
			value := ""
			for _, candidate := range attrMatch[2:] {
				if candidate != "" {
					value = candidate
					break
				}
			}
			attrs[name] = value
		}
		for _, anchorAttr := range []string{"id", "name"} {
			if value := attrs[anchorAttr]; value != "" {
				parser.Anchors[value] = true
			}
		}
		switch tag {
		case "a":
			if value := attrs["href"]; value != "" {
				parser.Refs = append(parser.Refs, docSiteRef{Tag: tag, Attr: "href", Value: value})
			}
		case "img", "script", "iframe", "source", "track", "audio", "video":
			if value := attrs["src"]; value != "" {
				parser.Refs = append(parser.Refs, docSiteRef{Tag: tag, Attr: "src", Value: value})
			}
		case "link":
			if value := attrs["href"]; value != "" && !strings.Contains(strings.ToLower(attrs["rel"]), "canonical") {
				parser.Refs = append(parser.Refs, docSiteRef{Tag: tag, Attr: "href", Value: value})
			}
		}
	}
	return parser
}

func readFileBestEffort(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func docSiteIsExternal(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	switch parsed.Scheme {
	case "http", "https", "mailto", "tel", "sms", "data", "javascript":
		return true
	}
	return strings.HasPrefix(value, "//")
}

func docSiteTargetFor(current, value, siteDir string) (string, string) {
	parsed, _ := url.Parse(value)
	parsedPath := ""
	fragment := ""
	if parsed != nil {
		parsedPath, _ = url.PathUnescape(parsed.Path)
		fragment, _ = url.QueryUnescape(parsed.Fragment)
	}
	var candidate string
	if parsedPath == "" {
		return current, fragment
	}
	if strings.HasPrefix(parsedPath, "/") {
		candidate = filepath.Join(siteDir, strings.TrimPrefix(parsedPath, "/"))
	} else {
		candidate = filepath.Join(filepath.Dir(current), parsedPath)
	}
	if strings.HasSuffix(parsedPath, "/") {
		candidate = filepath.Join(candidate, "index.html")
	} else if filepath.Ext(candidate) == "" {
		htmlCandidate := candidate + ".html"
		indexCandidate := filepath.Join(candidate, "index.html")
		if fileExists(htmlCandidate) {
			candidate = htmlCandidate
		} else if fileExists(indexCandidate) {
			candidate = indexCandidate
		}
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return candidate, fragment
	}
	return abs, fragment
}

func docSiteFailure(root, siteDir, kind, htmlFile, attr, value, message, target, fragment string) docSiteFailureDetail {
	return docSiteFailureDetail{
		Kind:      kind,
		Path:      docSiteRel(root, siteDir, htmlFile),
		Attribute: attr,
		Value:     value,
		Message:   message,
		Target:    docSiteRel(root, siteDir, target),
		Fragment:  fragment,
	}
}

func docSiteRel(root, siteDir, path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if rel, err := filepath.Rel(root, abs); err == nil && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
		return filepath.ToSlash(rel)
	}
	if rel, err := filepath.Rel(siteDir, abs); err == nil && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func docSiteWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
