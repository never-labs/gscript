package dialect

import (
	"regexp"
	"strconv"
	"strings"
)

type MarkdownSummary struct {
	Title      string
	Headings   []MarkdownHeading
	Links      []MarkdownLink
	CodeBlocks []MarkdownCodeBlock
	ListItems  int
	Paragraphs int
	PlainText  string
}

type MarkdownHeading struct {
	Level int
	Text  string
}

type MarkdownLink struct {
	Text string
	URL  string
}

type MarkdownCodeBlock struct {
	Info string
	Text string
}

var markdownLinkRE = regexp.MustCompile(`\[([^\]\n]+)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

func ParseMarkdownSummary(src string) MarkdownSummary {
	lines := Lines(src, true, true)
	out := MarkdownSummary{}
	var plain []string
	var paragraph strings.Builder
	var inCode bool
	var codeFence string
	var code MarkdownCodeBlock
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inCode {
			if strings.HasPrefix(trimmed, codeFence) {
				out.CodeBlocks = append(out.CodeBlocks, code)
				inCode = false
				codeFence = ""
				code = MarkdownCodeBlock{}
				continue
			}
			if code.Text != "" {
				code.Text += "\n"
			}
			code.Text += line
			continue
		}
		if fence, info, ok := markdownFence(trimmed); ok {
			flushMarkdownParagraph(&out, &paragraph, &plain)
			inCode = true
			codeFence = fence
			code = MarkdownCodeBlock{Info: info}
			continue
		}
		if trimmed == "" {
			flushMarkdownParagraph(&out, &paragraph, &plain)
			continue
		}
		for _, match := range markdownLinkRE.FindAllStringSubmatch(line, -1) {
			out.Links = append(out.Links, MarkdownLink{Text: match[1], URL: match[2]})
		}
		if heading, ok := markdownHeading(trimmed); ok {
			flushMarkdownParagraph(&out, &paragraph, &plain)
			out.Headings = append(out.Headings, heading)
			if out.Title == "" && heading.Level == 1 {
				out.Title = heading.Text
			}
			plain = append(plain, heading.Text)
			continue
		}
		if text, ok := markdownListItem(trimmed); ok {
			flushMarkdownParagraph(&out, &paragraph, &plain)
			out.ListItems++
			if text != "" {
				plain = append(plain, text)
			}
			continue
		}
		if paragraph.Len() > 0 {
			paragraph.WriteByte(' ')
		}
		paragraph.WriteString(trimmed)
	}
	if inCode {
		out.CodeBlocks = append(out.CodeBlocks, code)
	}
	flushMarkdownParagraph(&out, &paragraph, &plain)
	out.PlainText = strings.Join(plain, "\n")
	return out
}

func markdownFence(trimmed string) (string, string, bool) {
	for _, fence := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, fence) {
			return fence, strings.TrimSpace(strings.TrimPrefix(trimmed, fence)), true
		}
	}
	return "", "", false
}

func markdownHeading(trimmed string) (MarkdownHeading, bool) {
	level := 0
	for level < len(trimmed) && level < 6 && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level == len(trimmed) || trimmed[level] != ' ' {
		return MarkdownHeading{}, false
	}
	return MarkdownHeading{Level: level, Text: strings.TrimSpace(trimmed[level+1:])}, true
}

func markdownListItem(trimmed string) (string, bool) {
	for _, marker := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(trimmed, marker) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, marker)), true
		}
	}
	i := 0
	for i < len(trimmed) && trimmed[i] >= '0' && trimmed[i] <= '9' {
		i++
	}
	if i == 0 || i+1 >= len(trimmed) {
		return "", false
	}
	if trimmed[i] != '.' && trimmed[i] != ')' {
		return "", false
	}
	if _, err := strconv.Atoi(trimmed[:i]); err != nil {
		return "", false
	}
	if trimmed[i+1] != ' ' {
		return "", false
	}
	return strings.TrimSpace(trimmed[i+2:]), true
}

func flushMarkdownParagraph(out *MarkdownSummary, paragraph *strings.Builder, plain *[]string) {
	text := strings.TrimSpace(paragraph.String())
	if text == "" {
		paragraph.Reset()
		return
	}
	out.Paragraphs++
	*plain = append(*plain, text)
	paragraph.Reset()
}
