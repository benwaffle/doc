package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type manPage struct {
	name     string
	section  int
	date     string
	sections []section
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

type nameRef struct {
	name string
}

func (n nameRef) String() string {
	return fmt.Sprintf("\x1b[91m%s\x1b[0m", n.name)
}

type textSpan struct {
	text string
}

func (t textSpan) String() string {
	return fmt.Sprintf("\x1b[4m%s\x1b[0m", t.text)
}

type flagSpan struct {
	flag string
	dash bool
}

func (f flagSpan) String() string {
	dash := ""
	if f.dash {
		dash = "-"
	}
	return fmt.Sprintf("\x1b[92m%s%s\x1b[0m", dash, f.flag)
}

type argSpan struct {
	arg string
}

func (a argSpan) String() string {
	return fmt.Sprintf("\x1b[93m%s\x1b[0m", a.arg)
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

type envVar struct {
	name string
}

func (e envVar) String() string {
	return "$" + e.name
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
	tag []any
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

	tokenizer: for {
		token, rest := nextToken(line)
		switch token {
		case "Fl":
			flag, rest := nextToken(rest)
			res = append(res, flagSpan{flag, true})
			line = rest
			lastMacro = "Fl"
		case "Cm":
			flag, rest := nextToken(rest)
			res = append(res, flagSpan{flag, false})
			line = rest
			lastMacro = "Cm"
		case "Ar":
			arg, rest := nextToken(rest)
			res = append(res, argSpan{arg})
			line = rest
			lastMacro = "Ar"
		case "Ev":
			env, rest := nextToken(rest)
			res = append(res, envVar{env})
			line = rest
			lastMacro = "Ev"
		case "Op":
			res = append(res, optional{parseLine(rest)})
			break tokenizer
		case ",", "|":
			res = append(res, textSpan{token})
			line = lastMacro + " " + rest
		case "":
			break tokenizer
		default:
			res = append(res, textSpan{line})
			break tokenizer
		}
	}

	return res
}

func parseMdoc(doc string) manPage {
	title, _ := regexp.Compile(`\.Dt ([A-Z_]+) (\d+)`)
	xr, _ := regexp.Compile(`\.Xr (\S+)(?: (\d+))?`)
	// .Nm macro
	nameFull, _ := regexp.Compile(`\.Nm (\S+)(?: (\S+))?`)
	savedName := ""

	page := manPage{}
	var currentSection *section
	currentList := list{}
	var currentListItem *listItem

	addSpans := func(spans ...any) {
		if currentListItem != nil {
			currentListItem.contents = append(currentListItem.contents, spans...)
		} else {
			currentSection.contents = append(currentSection.contents, spans...)
		}
	}

	for _, line := range strings.Split(doc, "\n") {
		fmt.Printf("👀 %s\n", line)
		switch {

		case strings.HasPrefix(line, ".\\\""): // comment
			// ignore

		case strings.HasPrefix(line, ".Dd"): // document date
			page.date = line[4:]

		case title.MatchString(line): // man page title
			parts := title.FindStringSubmatch(line)
			page.name = parts[1]
			section, err := strconv.Atoi(parts[2])
			if err != nil {
				panic(err)
			}
			page.section = section

		case strings.HasPrefix(line, ".Sh"): // section header
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
			addSpans(nameRef{name})
			if len(parts) > 2 {
				// TODO: i think this adds blank spans
				addSpans(textSpan{text: parts[2]})
			}

		case line == ".Nm": // .Nm - page name
			if savedName == "" { // first invocation, save the name
				name := line[4:]
				savedName = name
			}
			addSpans(nameRef{savedName})

		case strings.HasPrefix(line, ".Nd"): // page description
			currentSection.contents = append(currentSection.contents, textSpan{text: line[4:]})

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

		case line == ".Pp":
			addSpans(textSpan{"\n\n"})

		case line == "." || line == "":
			// ignore

		case strings.HasPrefix(line, "."):
			fmt.Printf("?? %s\n", line)
			addSpans(parseLine(line[1:])...)

		default:
			fmt.Printf("?? %s\n", line)
			addSpans(parseLine(line)...)

		}
	}
	page.sections = append(page.sections, *currentSection)
	fmt.Printf("%+v\n", page)
	return page
}

func findDoc(target string) string {
	var foundPath string
	// TODO: read $MANPATH
	filepath.WalkDir("/usr/share/man", func(path string, d fs.DirEntry, err error) error {
		name := filepath.Base(path)
		dir := filepath.Base(filepath.Dir(path))
		section := strings.TrimPrefix(dir, "man")

		// TODO: handle .gz
		if name == fmt.Sprintf("%s.%s", target, section) {
			foundPath = path
			return filepath.SkipAll
		}

		return nil
	})
	return foundPath
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

	data, err := os.ReadFile(manFile)
	if err != nil {
		panic(err)
	}

	parseMdoc(string(data))
}
