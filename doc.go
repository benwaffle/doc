package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type manPage struct {
	Name     string
	Section  int
	Date     string
	Sections []section
	Extra    string
}

func (m manPage) String() string {
	res := fmt.Sprintf("-----\nname: %s\nsection: %d\ndate: %s\n-----\n", m.Name, m.Section, m.Date)
	for _, section := range m.Sections {
		res += fmt.Sprintf("%+v\n", section)
	}
	return res
}

type Span interface {
	Render() string
}

type section struct {
	Name     string
	Contents []Span
}

func (s section) String() string {
	if false {
		res := "# " + s.Name + "\n"
		for i, span := range s.Contents {
			res += fmt.Sprintf("\t%d %+v\n", i, span)
		}
		return res
	} else {
		return fmt.Sprintf("# %s\n%+v\n", s.Name, s.Contents)
	}
}

type textTag int

const (
	tagPlain textTag = iota
	tagNameRef
	tagArg
	tagEnvVar
	tagVariable
	tagPath
	tagSubsectionHeader
	tagLiteral
	tagSymbolic
	tagStandard
	tagParens
	tagBold
	tagItalic
)

var styles = map[textTag]lipgloss.Style{
	tagPlain:    lipgloss.NewStyle(),
	tagNameRef:  lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	tagArg:      lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
	tagVariable: lipgloss.NewStyle().Foreground(lipgloss.Color("13")),
	tagPath:     lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
	tagSubsectionHeader: lipgloss.NewStyle().
		Bold(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		Margin(1),
	tagSymbolic: lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	tagStandard: lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
	tagBold:     lipgloss.NewStyle().Bold(true),
	tagItalic:   lipgloss.NewStyle().Italic(true),
	tagLiteral:  lipgloss.NewStyle(),
}

type textSpan struct {
	Typ     textTag
	Text    string
	NoSpace bool // Set to false by default
}

func (t textSpan) Render() string {
	var res string
	switch t.Typ {
	case tagEnvVar:
		res = fmt.Sprintf("$%s", t.Text)
	case tagParens:
		res = fmt.Sprintf("(%s)", t.Text)
	default:
		res = styles[t.Typ].Render(t.Text)
	}
	if !t.NoSpace {
		res += " "
	}
	return res
}

type flagSpan struct {
	Flag    string
	Dash    bool
	NoSpace bool // Set to false by default
}

var flagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

func (f flagSpan) Render() string {
	dash := ""
	if f.Dash {
		dash = "-"
	}
	res := flagStyle.Render(dash + f.Flag)
	if !f.NoSpace {
		res += " "
	}
	return res
}

type manRef struct {
	Name    string
	Section *int
}

func (m manRef) Render() string {
	res := m.Name
	if m.Section != nil {
		res += fmt.Sprintf("(%d)", *m.Section)
	}
	return res
}

type optional struct {
	Contents []Span `json:"optionalContents"`
}

func (o optional) Render() string {
	res := "["
	for _, span := range o.Contents {
		res += span.Render()
	}
	res = strings.TrimSuffix(res, " ")
	res += "] "
	return res
}

type list struct {
	Items []listItem
}

func (l list) Render() string {
	res := ""
	for i, item := range l.Items {
		res += fmt.Sprintf("\n%d\ttag[%+v]\tcontents[%+v]", i, item.Tag, item.Contents)
	}
	return res
}

type listItem struct {
	Tag      []Span
	Contents []Span
}

func nextToken(input string) (string, string) {
	loc := strings.Index(input, " ")
	if loc == -1 {
		return input, ""
	}
	return input[:loc], input[loc+1:]
}

func parseLine(line string) []Span {
	if line == "" {
		return nil
	}

	var res []Span
	lastMacro := ""
	repeatMacro := false

tokenizer:
	for {
		token, rest := nextToken(line)
		switch token {
		case "Fl": // command line flag with dash
			flag, rest := nextToken(rest)
			res = append(res, flagSpan{flag, true, false})
			line = rest
			lastMacro = "Fl"
		case "Cm": // command line something with no dash
			flag, rest := nextToken(rest)
			res = append(res, flagSpan{flag, false, false})
			line = rest
			lastMacro = "Cm"
		case "Ar": // command line argument
			arg, rest := nextToken(rest)
			if arg == "" {
				arg = "file ..."
			}
			res = append(res, textSpan{tagArg, arg, false})
			line = rest
			lastMacro = "Ar"
		case "Ev": // environment variable
			env, rest := nextToken(rest)
			res = append(res, textSpan{tagEnvVar, env, false})
			line = rest
			lastMacro = "Ev"
		case "Va": // variable
			vari, rest := nextToken(rest)
			res = append(res, textSpan{tagVariable, vari, false})
			line = rest
			lastMacro = "Va"
		case "Pa": // path
			pa, rest := nextToken(rest)
			res = append(res, textSpan{tagPath, pa, false})
			line = rest
			lastMacro = "Pa"
		case "Sy": // symbolic
			sym, rest := nextToken(rest)
			res = append(res, textSpan{tagSymbolic, sym, false})
			line = rest
			lastMacro = "Sy"
		case "Li": // literal
			literal, rest := nextToken(rest)
			res = append(res, textSpan{tagLiteral, literal, false})
			line = rest
			lastMacro = "Li"
		case "St": // standard
			standard, rest := nextToken(rest)
			res = append(res, textSpan{tagStandard, standard, false})
			line = rest
			lastMacro = "St"
		case "Pq": // parens
			parens, rest := nextToken(rest)
			res = append(res, textSpan{tagParens, parens, false})
			line = rest
			lastMacro = "Pq"
		case "B": // bold
			bold, rest := nextToken(rest)
			res = append(res, textSpan{tagBold, bold, false})
			line = rest
			lastMacro = "B"
		case "I": // italic
			italic, rest := nextToken(rest)
			res = append(res, textSpan{tagItalic, italic, false})
			line = rest
			lastMacro = "I"
		case "BR": // alternate bold and normal
			bold, rest := nextToken(rest)
			if bold != "" {
				res = append(res, textSpan{tagBold, bold, false})
				line = "RB " + rest
			} else {
				line = rest
			}
			lastMacro = "BR"
		case "RB": // alternate normal and bold
			roman, rest := nextToken(rest)
			if roman != "" {
				res = append(res, textSpan{tagPlain, roman, false})
				line = "BR " + rest
			} else {
				line = rest
			}
			lastMacro = "RB"
		case "RI": // alternate normal and italic
			roman, rest := nextToken(rest)
			if roman != "" {
				res = append(res, textSpan{tagPlain, roman, false})
				line = "IR " + rest
			} else {
				line = rest
			}
			lastMacro = "RI"
		case "IR": // alternate italic and normal
			italic, rest := nextToken(rest)
			if italic != "" {
				res = append(res, textSpan{tagItalic, italic, false})
				line = "RI " + rest
			} else {
				line = rest
			}
			lastMacro = "IR"
		case "Ns": // no space
			index := len(res) - 1
			last := res[index]
			switch span := last.(type) {
			case textSpan:
				span.NoSpace = true
				res[index] = span
			case flagSpan:
				span.NoSpace = true
				res[index] = span
			default:
				fmt.Printf("%+v\n", res)
				panic("Don't know how to handle Ns macro")
			}
			line = rest
		case "Op": // optional
			res = append(res, optional{parseLine(rest)})
			break tokenizer
		case ",", "|":
			res = append(res, textSpan{tagPlain, token, false})
			line = rest
			repeatMacro = true
		case "":
			break tokenizer
		default:
			if repeatMacro {
				line = lastMacro + " " + line
				repeatMacro = false
			} else {
				res = append(res, textSpan{tagPlain, token, false})
				line = rest
			}
		}
	}

	return res
}

func parseMdoc(doc string) manPage {
	mdocTitle, _ := regexp.Compile(`\.Dt ([A-Z_]+) (\d+)`) // .Dt macro
	xr, _ := regexp.Compile(`\.Xr (\S+)(?: (\d+))?`)       // .Xr macro
	nameFull, _ := regexp.Compile(`\.Nm (\S+)(?: (\S+))?`) // .Nm macro
	savedName := ""

	page := manPage{}
	var currentSection *section
	currentList := list{}
	var currentListItem *listItem

	addSpans := func(spans ...Span) {
		if currentListItem != nil {
			currentListItem.Contents = append(currentListItem.Contents, spans...)
		} else if currentSection != nil {
			currentSection.Contents = append(currentSection.Contents, spans...)
		} else {
			panic("no current section")
		}
	}

	for _, line := range strings.Split(doc, "\n") {
		switch {

		case strings.HasPrefix(line, ".\\\"") || strings.HasPrefix(line, "'\\\""): // commenr
			// ignore

		case strings.HasPrefix(line, ".Dd"): // document date
			page.Date = line[4:]

		case mdocTitle.MatchString(line): // mdoc page title
			parts := mdocTitle.FindStringSubmatch(line)
			page.Name = parts[1]
			section, err := strconv.Atoi(parts[2])
			if err != nil {
				panic(err)
			}
			page.Section = section

		case strings.HasPrefix(line, ".TH"): // man page title
			parts := strings.Split(line[4:], " ")
			page.Name = parts[0]
			section, err := strconv.Atoi(parts[1])
			if err != nil {
				panic(err)
			}
			page.Section = section
			page.Date = parts[2]
			page.Extra = parts[3] + " " + parts[4]

		case strings.HasPrefix(line, ".Sh") || strings.HasPrefix(line, ".SH"): // section header
			if currentSection != nil {
				page.Sections = append(page.Sections, *currentSection)
			}

			currentSection = &section{
				Name: line[4:],
			}

		case nameFull.MatchString(line): // .Nm - page name
			parts := nameFull.FindStringSubmatch(line)
			name := parts[1]
			if savedName == "" { // first invocation, save the name
				savedName = name
			}
			addSpans(textSpan{tagNameRef, name, false})
			if len(parts) > 2 && parts[2] != "" {
				addSpans(textSpan{Text: parts[2]})
			}

		case line == ".Nm": // .Nm - page name
			if savedName == "" { // first invocation, save the name
				name := line[4:]
				savedName = name
			}
			addSpans(textSpan{tagNameRef, savedName, false})

		case strings.HasPrefix(line, ".Nd"): // page description
			addSpans(textSpan{Text: line[4:]})

		case strings.HasPrefix(line, ".In"): // #include
			addSpans(textSpan{Text: fmt.Sprintf("#include <%s>", line[4:])})

		case xr.MatchString(line): // man reference
			parts := xr.FindStringSubmatchIndex(line)
			name := line[parts[2]:parts[3]]
			var section *int
			if len(parts) > 3 {
				sec, err := strconv.Atoi(line[parts[4]:parts[5]])
				if err != nil {
					panic(err)
				}
				section = &sec
			}
			// TODO: parse rest of line
			addSpans(manRef{name, section})

		case strings.HasPrefix(line, ".Ss") || strings.HasPrefix(line, ".SS"): // subsection header
			addSpans(textSpan{tagSubsectionHeader, line[4:], false})

		case strings.HasPrefix(line, ".Dl"): // indented literal
			addSpans(textSpan{tagPlain, "\t", false})
			addSpans(parseLine(line[4:])...)

		case strings.HasPrefix(line, ".IP"): // indented paragraph
			arg1, rest := nextToken(line[4:])
			tag := ""
			if arg1 == `\(bu` {
				tag = "•"
			} else if arg1 == `\(em` {
				tag = "—"
			} else {
				tag = arg1
			}

			arg2, _ := nextToken(rest)
			indent := 0
			if arg2 != "" {
				indentVal, err := strconv.Atoi(arg2)
				if err != nil {
					panic(err)
				}
				indent = indentVal
			}

			addSpans(textSpan{tagPlain, "\n" + strings.Repeat("  ", indent) + tag, false})

		case strings.HasPrefix(line, ".TP"):
			addSpans(textSpan{tagPlain, "\n", false})

		case strings.HasPrefix(line, ".ft"): // font
			// not supported

		case strings.HasPrefix(line, ".Bl"): // begin list
			// TODO: parse list options
			currentList = list{}

		case strings.HasPrefix(line, ".It"): // list item
			if currentListItem != nil {
				currentList.Items = append(currentList.Items, *currentListItem)
			}

			currentListItem = &listItem{}
			if len(line) > 4 {
				currentListItem.Tag = append(currentListItem.Contents, parseLine(line[4:])...)
			}

		case strings.HasPrefix(line, ".El"): // end list
			if currentListItem != nil {
				currentList.Items = append(currentList.Items, *currentListItem)
				currentListItem = nil
			}
			currentSection.Contents = append(currentSection.Contents, currentList)

		case strings.HasPrefix(line, ".Os"): // OS
			// TODO: do we need this?

		case line == ".Pp" || line == ".PP":
			addSpans(textSpan{tagPlain, "\n\n", false})

		case line == ".br":
			addSpans(textSpan{tagPlain, "\n", false})

		case line == "." || line == "":
			// ignore

		case strings.HasPrefix(line, "."):
			addSpans(parseLine(line[1:])...)

		default:
			addSpans(parseLine(line)...)

		}
	}
	page.Sections = append(page.Sections, *currentSection)
	return page
}

func findDocInDir(target string, dir string) string {
	var foundPath string
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		name := filepath.Base(path)
		dir := filepath.Base(filepath.Dir(path))
		section := strings.TrimPrefix(dir, "man")

		if name == fmt.Sprintf("%s.%s", target, section) || name == fmt.Sprintf("%s.%s.gz", target, section) {
			foundPath = path
			return filepath.SkipAll
		}

		return nil
	})
	return foundPath
}

func findDoc(target string) string {
	manPath := os.Getenv("MANPATH")
	for _, dir := range strings.Split(manPath, ":") {
		path := findDocInDir(target, dir)
		if path != "" {
			return path
		}
	}
	return findDocInDir(target, "/usr/share/man")
}

func readManPage(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil
	}
	defer file.Close()

	var reader io.Reader = bufio.NewReader(file)

	if strings.HasSuffix(path, ".gz") {
		gzipReader, err := gzip.NewReader(reader)
		if err != nil {
			return "", err
		}
		reader = gzipReader
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func dumpAst(page manPage) {
	bytes, err := json.Marshal(page)
	if err != nil {
		panic(err)
	}
	os.WriteFile("ast.json", bytes, 0666)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command>\n", os.Args[0])
		os.Exit(1)
	}

	target := os.Args[1]
	var manFile string

	if _, err := os.Stat(target); err == nil {
		manFile = target
	} else {
		manFile = findDoc(target)
		if manFile == "" {
			fmt.Fprintf(os.Stderr, "cannot find man page for \"%s\"\n", target)
			os.Exit(1)
		}
	}

	fmt.Println(manFile)

	data, err := readManPage(manFile)
	if err != nil {
		panic(err)
	}

	page := parseMdoc(data)

	dumpAst(page)

	p := tea.NewProgram(
		NewModel(page),
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	)

	if _, err := p.Run(); err != nil {
		fmt.Println("could not run program:", err)
		os.Exit(1)
	}
}
