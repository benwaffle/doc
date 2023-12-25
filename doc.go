package main

import (
	"bufio"
	"compress/gzip"
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
	name     string
	section  int
	date     string
	sections []section
	extra    string
}

func (m manPage) String() string {
	res := fmt.Sprintf("-----\nname: %s\nsection: %d\ndate: %s\n-----\n", m.name, m.section, m.date)
	for _, section := range m.sections {
		res += fmt.Sprintf("%+v\n", section)
	}
	return res
}

type section struct {
	name     string
	contents []any
}

func (s section) String() string {
	if false {
		res := "# " + s.name + "\n"
		for i, span := range s.contents {
			res += fmt.Sprintf("\t%d %+v\n", i, span)
		}
		return res
	} else {
		return fmt.Sprintf("# %s\n%+v\n", s.name, s.contents)
	}
}

type textTag int

const (
	tagPlain textTag = iota
	tagNameRef
	tagArg
	tagEnvVar
	tagNoSpace
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
	tagPlain:            lipgloss.NewStyle(),
	tagNameRef:          lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	tagArg:              lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
	tagVariable:         lipgloss.NewStyle().Foreground(lipgloss.Color("13")),
	tagPath:             lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
	tagSubsectionHeader: lipgloss.NewStyle().Bold(true).BorderStyle(lipgloss.NormalBorder()),
	tagSymbolic:         lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
	tagStandard:         lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
	tagBold:             lipgloss.NewStyle().Bold(true),
	tagItalic:           lipgloss.NewStyle().Italic(true),
}

type textSpan struct {
	typ  textTag
	text string
}

func (t textSpan) String() string {
	if sty, ok := styles[t.typ]; ok {
		return sty.Render(t.text)
	}

	switch t.typ {
	case tagEnvVar:
		return fmt.Sprintf("$%s", t.text)
	case tagNoSpace:
		return ""
	case tagLiteral:
		return t.text
	case tagParens:
		return fmt.Sprintf("(%s)", t.text)
	default:
		panic("unknown text tag")
	}
}

type flagSpan struct {
	flag string
	dash bool
}

var flagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

func (f flagSpan) String() string {
	dash := ""
	if f.dash {
		dash = "-"
	}
	return flagStyle.Render(dash + f.flag)
}

type manRef struct {
	name    string
	section *int
}

func (m manRef) String() string {
	res := m.name
	if m.section != nil {
		res += fmt.Sprintf("(%d)", *m.section)
	}
	return res
}

type optional struct {
	contents []any
}

func (o optional) String() string {
	return fmt.Sprintf("[%+v]", o.contents)
}

type list struct {
	items []listItem
}

func (l list) String() string {
	res := ""
	for i, item := range l.items {
		res += fmt.Sprintf("\n%d\t%+v\t%+v", i, item.tag, item.contents)
	}
	return res
}

type listItem struct {
	tag      []any
	contents []any
}

func nextToken(input string) (string, string) {
	loc := strings.Index(input, " ")
	if loc == -1 {
		return input, ""
	}
	return input[:loc], input[loc+1:]
}

func parseLine(line string) []any {
	if line == "" {
		return nil
	}

	var res []any
	lastMacro := ""

tokenizer:
	for {
		token, rest := nextToken(line)
		switch token {
		case "Fl": // command line flag with dash
			flag, rest := nextToken(rest)
			res = append(res, flagSpan{flag, true})
			line = rest
			lastMacro = "Fl"
		case "Cm": // command line something with no dash
			flag, rest := nextToken(rest)
			res = append(res, flagSpan{flag, false})
			line = rest
			lastMacro = "Cm"
		case "Ar": // command line argument
			arg, rest := nextToken(rest)
			res = append(res, textSpan{tagArg, arg})
			line = rest
			lastMacro = "Ar"
		case "Ev": // environment variable
			env, rest := nextToken(rest)
			res = append(res, textSpan{tagEnvVar, env})
			line = rest
			lastMacro = "Ev"
		case "Va": // variable
			vari, rest := nextToken(rest)
			res = append(res, textSpan{tagVariable, vari})
			line = rest
			lastMacro = "Va"
		case "Pa": // path
			pa, rest := nextToken(rest)
			res = append(res, textSpan{tagPath, pa})
			line = rest
			lastMacro = "Pa"
		case "Sy": // symbolic
			sym, rest := nextToken(rest)
			res = append(res, textSpan{tagSymbolic, sym})
			line = rest
			lastMacro = "Sy"
		case "Li": // literal
			literal, rest := nextToken(rest)
			res = append(res, textSpan{tagLiteral, literal})
			line = rest
			lastMacro = "Li"
		case "St": // standard
			standard, rest := nextToken(rest)
			res = append(res, textSpan{tagStandard, standard})
			line = rest
			lastMacro = "St"
		case "Pq": // parens
			parens, rest := nextToken(rest)
			res = append(res, textSpan{tagParens, parens})
			line = rest
			lastMacro = "Pq"
		case "B": // bold
			bold, rest := nextToken(rest)
			res = append(res, textSpan{tagBold, bold})
			line = rest
			lastMacro = "B"
		case "I": // italic
			italic, rest := nextToken(rest)
			res = append(res, textSpan{tagItalic, italic})
			line = rest
			lastMacro = "I"
		case "BR": // alternate bold and normal
			bold, rest := nextToken(rest)
			if bold != "" {
				res = append(res, textSpan{tagBold, bold})
				line = "RB " + rest
			} else {
				line = rest
			}
			lastMacro = "BR"
		case "RB": // alternate normal and bold
			roman, rest := nextToken(rest)
			if roman != "" {
				res = append(res, textSpan{tagPlain, roman})
				line = "BR " + rest
			} else {
				line = rest
			}
			lastMacro = "RB"
		case "RI": // alternate normal and italic
			roman, rest := nextToken(rest)
			if roman != "" {
				res = append(res, textSpan{tagPlain, roman})
				line = "IR " + rest
			} else {
				line = rest
			}
			lastMacro = "RI"
		case "IR": // alternate italic and normal
			italic, rest := nextToken(rest)
			if italic != "" {
				res = append(res, textSpan{tagItalic, italic})
				line = "RI " + rest
			} else {
				line = rest
			}
			lastMacro = "IR"
		case "Ns": // no space
			res = append(res, textSpan{tagNoSpace, ""})
			line = rest
		case "Op": // optional
			res = append(res, optional{parseLine(rest)})
			break tokenizer
		case ",", "|":
			res = append(res, textSpan{tagPlain, token})
			line = lastMacro + " " + rest
		case "":
			break tokenizer
		default:
			res = append(res, textSpan{tagPlain, token})
			line = rest
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

	addSpans := func(spans ...any) {
		if currentListItem != nil {
			currentListItem.contents = append(currentListItem.contents, spans...)
		} else if currentSection != nil {
			currentSection.contents = append(currentSection.contents, spans...)
		} else {
			panic("no current section")
		}
	}

	for _, line := range strings.Split(doc, "\n") {
		switch {

		case strings.HasPrefix(line, ".\\\"") || strings.HasPrefix(line, "'\\\""): // commenr
			// ignore

		case strings.HasPrefix(line, ".Dd"): // document date
			page.date = line[4:]

		case mdocTitle.MatchString(line): // mdoc page title
			parts := mdocTitle.FindStringSubmatch(line)
			page.name = parts[1]
			section, err := strconv.Atoi(parts[2])
			if err != nil {
				panic(err)
			}
			page.section = section

		case strings.HasPrefix(line, ".TH"): // man page title
			parts := strings.Split(line[4:], " ")
			page.name = parts[0]
			section, err := strconv.Atoi(parts[1])
			if err != nil {
				panic(err)
			}
			page.section = section
			page.date = parts[2]
			page.extra = parts[3] + " " + parts[4]

		case strings.HasPrefix(line, ".Sh") || strings.HasPrefix(line, ".SH"): // section header
			if currentSection != nil {
				page.sections = append(page.sections, *currentSection)
			}

			currentSection = &section{
				name: line[4:],
			}

		case nameFull.MatchString(line): // .Nm - page name
			parts := nameFull.FindStringSubmatch(line)
			name := parts[1]
			if savedName == "" { // first invocation, save the name
				savedName = name
			}
			addSpans(textSpan{tagNameRef, name})
			if len(parts) > 2 {
				// TODO: i think this adds blank spans
				addSpans(textSpan{text: parts[2]})
			}

		case line == ".Nm": // .Nm - page name
			if savedName == "" { // first invocation, save the name
				name := line[4:]
				savedName = name
			}
			addSpans(textSpan{tagNameRef, savedName})

		case strings.HasPrefix(line, ".Nd"): // page description
			addSpans(textSpan{text: line[4:]})

		case strings.HasPrefix(line, ".In"): // #include
			addSpans(textSpan{text: fmt.Sprintf("#include <%s>", line[4:])})

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
			addSpans(textSpan{tagSubsectionHeader, line[4:]})

		case strings.HasPrefix(line, ".Dl"): // indented literal
			addSpans(textSpan{tagPlain, "\t"})
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

			addSpans(textSpan{tagPlain, "\n" + strings.Repeat("  ", indent) + tag})

		case strings.HasPrefix(line, ".TP"):
			addSpans(textSpan{tagPlain, "\n"})

		case strings.HasPrefix(line, ".ft"): // font
			// not supported

		case strings.HasPrefix(line, ".Bl"): // begin list
			// TODO: parse list options
			currentList = list{}

		case strings.HasPrefix(line, ".It"): // list item
			if currentListItem != nil {
				currentList.items = append(currentList.items, *currentListItem)
			}

			currentListItem = &listItem{}
			if len(line) > 4 {
				currentListItem.tag = append(currentListItem.contents, parseLine(line[4:])...)
			}

		case strings.HasPrefix(line, ".El"): // end list
			if currentListItem != nil {
				currentList.items = append(currentList.items, *currentListItem)
				currentListItem = nil
			}
			currentSection.contents = append(currentSection.contents, currentList)

		case strings.HasPrefix(line, ".Os"): // OS
			// TODO: do we need this?

		case line == ".Pp" || line == ".PP":
			addSpans(textSpan{tagPlain, "\n\n"})

		case line == ".br":
			addSpans(textSpan{tagPlain, "\n"})

		case line == "." || line == "":
			// ignore

		case strings.HasPrefix(line, "."):
			addSpans(parseLine(line[1:])...)

		default:
			addSpans(parseLine(line)...)

		}
	}
	page.sections = append(page.sections, *currentSection)
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

	p := tea.NewProgram(
		model{page: page},
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	)

	if _, err := p.Run(); err != nil {
		fmt.Println("could not run program:", err)
		os.Exit(1)
	}
}
