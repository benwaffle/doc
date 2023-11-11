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
}

func parseManDoc(doc string) manPage {
	title, _ := regexp.Compile(`\.Dt ([A-Z]+) (\d+)`)

	page := manPage{}
	for _, line := range strings.Split(doc, "\n") {
		switch {

		case strings.HasPrefix(line, ".\\\""):
			fmt.Printf("// %s\n", line)

		case strings.HasPrefix(line, ".Dd "):
			page.date = line[4:]

		case title.MatchString(line):
			parts := title.FindStringSubmatch(line)
			fmt.Println(parts)
			page.name = parts[1]
			section, err := strconv.Atoi(parts[2])
			if err != nil {
				panic(err)
			}
			page.section = section

		default:
			fmt.Printf("?? %s\n", line)

		}
	}
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

	parseManDoc(string(data))
}
