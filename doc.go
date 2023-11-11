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
	name    string
	section int
	date    string
	sections []section
}

type section struct {
	name string
}

func parseMdoc(doc string) manPage {
	title, _ := regexp.Compile(`\.Dt ([A-Z_]+) (\d+)`)

	page := manPage{}
	var currentSection *section

	for _, line := range strings.Split(doc, "\n") {
		fmt.Printf("👀 %s\n", line)
		switch {

		case strings.HasPrefix(line, ".\\\""): // comment
			// ignore

		case strings.HasPrefix(line, ".Dd "): // document date
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

		default:
			fmt.Printf("?? %s\n", line)

		}
	}
	page.sections = append(page.sections, *currentSection)
	fmt.Printf("%v\n", page)
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

	data, err := os.ReadFile(manFile)
	if err != nil {
		panic(err)
	}

	parseMdoc(string(data))
}
