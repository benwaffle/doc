package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/table"
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
	tagSymbolic:  lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	tagBold:      lipgloss.NewStyle().Bold(true),
	tagItalic:    lipgloss.NewStyle().Italic(true),
	tagUnderline: lipgloss.NewStyle().Underline(true),
	tagLiteral:   lipgloss.NewStyle(),
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

var standardStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

func (std standardRef) Render(width int) string {
	res := ""
	switch std.Standard {
	case "-ansiC":
		res = `ANSI X3.159-1989 (“ANSI C89”)`
	case "-ansiC-89":
		res = `ANSI X3.159-1989 (“ANSI C89”)`
	case "-isoC":
		res = `ISO/IEC 9899:1990 (“ISO C90”)`
	case "-isoC-90":
		res = `ISO/IEC 9899:1990 (“ISO C90”)`
	case "-isoC-amd1":
		res = `ISO/IEC 9899/AMD1:1995 (“ISO C90, Amendment 1”)`
	case "-isoC-tcor1":
		res = `ISO/IEC 9899/TCOR1:1994 (“ISO C90, Technical Corrigendum 1”)`
	case "-isoC-tcor2":
		res = `ISO/IEC 9899/TCOR2:1995 (“ISO C90, Technical Corrigendum 2”)`
	case "-isoC-99":
		res = `ISO/IEC 9899:1999 (“ISO C99”)`
	case "-isoC-2011":
		res = `ISO/IEC 9899:2011 (“ISO C11”)`
	case "-p1003.1-88":
		res = `IEEE Std 1003.1-1988 (“POSIX.1”)`
	case "-p1003.1":
		res = `IEEE Std 1003.1 (“POSIX.1”)`
	case "-p1003.1-90":
		res = `IEEE Std 1003.1-1990 (“POSIX.1”)`
	case "-iso9945-1-90":
		res = `ISO/IEC 9945-1:1990 (“POSIX.1”)`
	case "-p1003.1b-93":
		res = `IEEE Std 1003.1b-1993 (“POSIX.1b”)`
	case "-p1003.1b":
		res = `IEEE Std 1003.1b (“POSIX.1b”)`
	case "-p1003.1c-95":
		res = `IEEE Std 1003.1c-1995 (“POSIX.1c”)`
	case "-p1003.1i-95":
		res = `IEEE Std 1003.1i-1995 (“POSIX.1i”)`
	case "-p1003.1-96":
		res = `ISO/IEC 9945-1:1996 (“POSIX.1”)`
	case "-iso9945-1-96":
		res = `ISO/IEC 9945-1:1996 (“POSIX.1”)`
	case "-p1003.2":
		res = `IEEE Std 1003.2 (“POSIX.2”)`
	case "-p1003.2-92":
		res = `IEEE Std 1003.2-1992 (“POSIX.2”)`
	case "-iso9945-2-93":
		res = `ISO/IEC 9945-2:1993 (“POSIX.2”)`
	case "-p1003.2a-92":
		res = `IEEE Std 1003.2a-1992 (“POSIX.2”)`
	case "-xpg4":
		res = `X/Open Portability Guide Issue 4 (“XPG4”)`
	case "-susv1":
		res = `Version 1 of the Single UNIX Specification (“SUSv1”)`
	case "-xpg4.2":
		res = `X/Open Portability Guide Issue 4, Version 2 (“XPG4.2”)`
	case "-xsh4.2":
		res = `X/Open System Interfaces and Headers Issue 4, Version 2 (“XSH4.2”)`
	case "-xcurses4.2":
		res = `X/Open Curses Issue 4, Version 2 (“XCURSES4.2”)`
	case "-p1003.1g-2000":
		res = `IEEE Std 1003.1g-2000 (“POSIX.1g”)`
	case "-svid4":
		res = `System V Interface Definition, Fourth Edition (“SVID4”),`
	case "-susv2":
		res = `Version 2 of the Single UNIX Specification (“SUSv2”)`
	case "-xbd5":
		res = `X/Open Base Definitions Issue 5 (“XBD5”)`
	case "-xsh5":
		res = `X/Open System Interfaces and Headers Issue 5 (“XSH5”)`
	case "-xcu5":
		res = `X/Open Commands and Utilities Issue 5 (“XCU5”)`
	case "-xns5":
		res = `X/Open Networking Services Issue 5 (“XNS5”)`
	case "-xns5.2":
		res = `X/Open Networking Services Issue 5.2 (“XNS5.2”)`
	case "-p1003.1-2001":
		res = `IEEE Std 1003.1-2001 (“POSIX.1”)`
	case "-susv3":
		res = `Version 3 of the Single UNIX Specification (“SUSv3”)`
	case "-p1003.1-2004":
		res = `IEEE Std 1003.1-2004 (“POSIX.1”)`
	case "-p1003.1-2008":
		res = `IEEE Std 1003.1-2008 (“POSIX.1”)`
	case "-susv4":
		res = `Version 4 of the Single UNIX Specification (“SUSv4”)`
	case "-ieee754":
		res = `IEEE Std 754-1985`
	case "-iso8601":
		res = `ISO 8601`
	case "-iso8802-3":
		res = `ISO 8802-3: 1989`
	case "-ieee1275-94":
		res = `IEEE Std 1275-1994 (“Open Firmware”)`
	default:
		res = std.Standard
	}
	return standardStyle.Render(res)
}

func (l list) Render(width int) string {
	if l.Typ == columnList {
		return l.RenderTable(width)
	}

	res := ""
	maxTagWidth := 8
	switch l.Typ {
	case bulletList, dashList:
		maxTagWidth = 2
	case tagList:
		maxTagWidth = l.Width + 1
	case ohangList:
		maxTagWidth = 0
	case enumList:
		maxTagWidth = 4
	case itemList:
		maxTagWidth = 0
	default:
		panic(fmt.Sprintf("Don't know how to render %d list", l.Typ))
	}
	indent := lipgloss.NewStyle().MarginLeft(l.Indent).Render
	tagFillWidth := lipgloss.NewStyle().Width(maxTagWidth)
	contentFillWidth := lipgloss.NewStyle().Width(width - maxTagWidth)
	contentMargin := lipgloss.NewStyle().MarginLeft(maxTagWidth)

	for i, item := range l.Items {
		res += "\n"
		if !l.Compact {
			res += "\n"
		}

		tag := ""

		switch l.Typ {
		case tagList, ohangList:
			for _, span := range item.Tag {
				tag += span.Render(width)
			}
			tag = strings.TrimSpace(tag)
		case bulletList:
			tag = "• "
		case dashList:
			tag = "- "
		case enumList:
			tag = fmt.Sprintf("%2d. ", i+1)
		case itemList:
			// no tag
		default:
			panic(fmt.Sprintf("Don't know how to render %d list", l.Typ))
		}

		contents := ""
		for _, span := range item.Contents {
			contents += span.Render(width - maxTagWidth)
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
	return indent(res)
}

func (l list) RenderTable(width int) string {
	var columns []table.Column
	var rows []table.Row

	for i, col := range l.Columns {
		colWidth := len(col) + 3 // +2 for padding, not sure why 3 is needed
		if i == len(l.Columns)-1 {
			// compute remaining width
			colWidth = width
			for _, col := range columns {
				colWidth -= col.Width
			}
			colWidth -= 4 // TODO: why does this fix wrapping?
		}

		columns = append(columns, table.Column{
			Title: col,
			Width: colWidth,
		})
	}

	nCols := len(columns)

	for _, item := range l.Items {
		row := table.Row{}
		cell := ""
		for _, span := range item.Tag {
			if len(row) >= nCols { // too many cells in this row, parsing error?
				break
			}
			if ts, ok := span.(textSpan); ok && ts.Typ == tagTableCellSeparator {
				row = append(row, cell)
				cell = ""
				continue
			}
			cell += span.Render(columns[len(row)].Width)
		}
		if len(cell) > 0 {
			row = append(row, cell)
		}
		rows = append(rows, row)
	}

	s := table.DefaultStyles()
	s.Selected = lipgloss.NewStyle()
	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithWidth(width),
		table.WithHeight(len(rows)),
		table.WithStyles(s),
	)

	rendered := tbl.View()
	firstLine := strings.Index(rendered, "\n")
	withoutHeader := rendered[firstLine+1:]

	return "\n\n" + withoutHeader
}
