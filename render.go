package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Span interface {
	Render(width int) string
}

var sectionHeader = lipgloss.NewStyle().
	Bold(true).
	BorderStyle(lipgloss.RoundedBorder()).
	BorderBottom(true)

func (page manPage) Render(width int) string {
	res := ""
	for i, section := range page.Sections {
		if i != 0 {
			res += "\n\n"
		}
		res += fmt.Sprintf("%s\n", sectionHeader.Render(section.Name))

		contents := ""
		for _, content := range section.Contents {
			contents += content.Render(width)
		}
		res += strings.TrimSpace(contents)
	}
	res += lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, false, false, false).Margin(2, 0).Render(page.Date)
	return res
}

var allWhitespace, _ = regexp.Compile(`^\s+$`)
var textStyles = map[textTag]lipgloss.Style{
	tagPlain:    lipgloss.NewStyle(),
	tagNameRef:  lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	tagArg:      lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
	tagVariable: lipgloss.NewStyle().Foreground(lipgloss.Color("13")),
	tagPath:     lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
	tagSubsectionHeader: lipgloss.NewStyle().
		Bold(true).
		Margin(2, 0, 0, 0),
	tagSymbolic: lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	tagStandard: lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
	tagBold:     lipgloss.NewStyle().Bold(true),
	tagItalic:   lipgloss.NewStyle().Italic(true),
	tagLiteral:  lipgloss.NewStyle(),
}

func (t textSpan) Render(_ int) string {
	text := strings.ReplaceAll(t.Text, "\\&", "") // unescape literals

	var res string
	switch t.Typ {
	case tagEnvVar:
		res = fmt.Sprintf("$%s", text)
	case tagSingleQuote:
		res = fmt.Sprintf("'%s'", text)
	case tagDoubleQuote:
		res = fmt.Sprintf("\"%s\"", text)
	case tagSubsectionHeader:
		res = textStyles[tagSubsectionHeader].Render(text) + "\n"
	default:
		res = textStyles[t.Typ].Render(text)
	}
	if !t.NoSpace && !allWhitespace.MatchString(t.Text) {
		res += " "
	}
	return res
}

var decorationStyles = map[decorationTag][]string{
	decorationOptional:      {"[", "]"},
	decorationParens:        {"(", ")"},
	decorationSingleQuote:   {"'", "'"},
	decorationDoubleQuote:   {"\"", "\""},
	decorationQuotedLiteral: {"‘", "’"},
}

func (d decoratedSpan) Render(width int) string {
	res := ""
	for _, span := range d.Contents {
		res += span.Render(width)
	}
	res = strings.TrimSuffix(res, " ")
	res = decorationStyles[d.Typ][0] + res + decorationStyles[d.Typ][1] + " "
	return res
}

var flagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

func (f flagSpan) Render(_ int) string {
	flag := strings.ReplaceAll(f.Flag, "\\&", "") // unescape literals

	dash := ""
	if f.Dash {
		dash = "-"
	}
	res := flagStyle.Render(dash + flag)
	if !f.NoSpace {
		res += " "
	}
	return res
}

func (m manRef) Render(_ int) string {
	res := m.Name
	if m.Section != nil {
		res += fmt.Sprintf("(%d)", *m.Section)
	}
	return res
}

func (l list) Render(width int) string {
	res := ""
	maxTagWidth := 8
	tagFillWidth := lipgloss.NewStyle().Width(maxTagWidth)
	contentFillWidth := lipgloss.NewStyle().Width(width - maxTagWidth)
	contentMargin := lipgloss.NewStyle().MarginLeft(maxTagWidth)

	for _, item := range l.Items {
		res += "\n"
		if !l.Compact {
			res += "\n"
		}

		tag := ""
		for _, span := range item.Tag {
			tag += span.Render(width)
		}
		tag = strings.TrimSpace(tag)

		contents := ""
		for _, span := range item.Contents {
			contents += span.Render(width)
		}
		contents = contentFillWidth.Render(contents)

		if lipgloss.Width(tag) > maxTagWidth {
			res += tag
			res += "\n"
			res += contentMargin.Render(contents)
		} else {
			tag = tagFillWidth.Render(tag)
			res += lipgloss.JoinHorizontal(lipgloss.Top, tag, contents)
		}
	}
	return res
}
