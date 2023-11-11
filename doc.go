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
		return fmt.Sprintf("# %s\n%+v", s.name, s.contents)
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
}

func (f flagSpan) String() string {
	return fmt.Sprintf("\x1b[92m-%s\x1b[0m", f.flag)
}

type argSpan struct {
	arg string
}

func (a argSpan) String() string {
	return fmt.Sprintf("\x1b[93m%s\x1b[0m", a.arg)
}

func parseFlagsAndArgs(line string) []any {
	if line == "" {
		return nil
	}

	fl, _ := regexp.Compile(`Fl (\S+)`)
	ar, _ := regexp.Compile(`Ar (\S+)`)

	_ = ar

	switch {
	case fl.MatchString(line):
		parts := fl.FindStringSubmatchIndex(line)
		begin := line[:parts[0]]
		flag := line[parts[2]:parts[3]]
		rest := parseFlagsAndArgs(line[parts[3]:])
		return append([]any{textSpan{begin}, flagSpan{flag}}, rest...)

	case ar.MatchString(line):
		parts := ar.FindStringSubmatchIndex(line)
		begin := line[:parts[0]]
		arg := line[parts[2]:parts[3]]
		rest := parseFlagsAndArgs(line[parts[3]:])
		return append([]any{textSpan{begin}, argSpan{arg}}, rest...)

	default:
		return []any{textSpan{line}}
	}
}

func parseMdoc(doc string) manPage {
	title, _ := regexp.Compile(`\.Dt ([A-Z_]+) (\d+)`)
	// .Nm macro
	nameFull, _ := regexp.Compile(`\.Nm (\S+)(?: ([\S]+))?`)
	savedName := ""

	page := manPage{}
	var currentSection *section

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
			currentSection.contents = append(currentSection.contents, nameRef{name})
			if len(parts) > 2 {
				// TODO: i think this adds blank spans
				currentSection.contents = append(currentSection.contents, textSpan{text: parts[2]})
			}

		case line == ".Nm": // .Nm - page name
			if savedName == "" { // first invocation, save the name
				name := line[4:]
				savedName = name
			}
			currentSection.contents = append(currentSection.contents, nameRef{savedName})

		case strings.HasPrefix(line, ".Nd"): // page description
			currentSection.contents = append(currentSection.contents, textSpan{text: line[4:]})

		case strings.HasPrefix(line, ".Op"): // optional flag
			currentSection.contents = append(currentSection.contents, parseFlagsAndArgs(line[4:])...)

		case strings.HasPrefix(line, ".Ar"): // argument
			currentSection.contents = append(currentSection.contents, argSpan{line[4:]})

		case strings.HasPrefix(line, ".In"): // #include
			currentSection.contents = append(currentSection.contents, textSpan{text: fmt.Sprintf("#include <%s>", line[4:])})

		case strings.HasPrefix(line, ".Os"): // OS
			// TODO: do we need this?

		case line == ".":
			// ignore

		default:
			fmt.Printf("?? %s\n", line)
			currentSection.contents = append(currentSection.contents, parseFlagsAndArgs(line)...)

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
	manFile := findDoc(target)
	if manFile == "" {
		fmt.Fprintf(os.Stderr, "cannot find man page for \"%s\"\n", target)
		os.Exit(1)
	}

	fmt.Println(manFile)

	data, err := os.ReadFile(manFile)
	if err != nil {
		panic(err)
	}

	parseMdoc(string(data))
}
