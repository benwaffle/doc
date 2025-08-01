package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func findDocInManSection(sectionDir, target string) string {
	section := strings.TrimPrefix(filepath.Base(sectionDir), "man")
	fullTarget := fmt.Sprintf("%s.%s", target, section)
	fullTargetGz := fmt.Sprintf("%s.%s.gz", target, section)

	files, err := os.ReadDir(sectionDir)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if file.Name() == fullTarget || file.Name() == fullTargetGz {
			return sectionDir + "/" + file.Name()
		}
	}
	return ""
}

func findDocInManDir(mandir, target string) string {
	dirs, err := os.ReadDir(mandir)
	if err != nil {
		panic(err)
	}

	for _, dir := range dirs {
		if strings.HasPrefix(dir.Name(), "man") {
			path := findDocInManSection(mandir+"/"+dir.Name(), target)
			if path != "" {
				return path
			}
		}
	}
	return ""
}

func findDoc(target string) string {
	manPath := os.Getenv("MANPATH")
	if len(manPath) > 0 {
		for _, dir := range strings.Split(manPath, ":") {
			if len(dir) == 0 {
				continue
			}
			path := findDocInManDir(dir, target)
			if path != "" {
				return path
			}
		}
	}
	// TODO: locale support
	return findDocInManDir("/usr/share/man", target)
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
	bytes, err := json.MarshalIndent(page, "", "  ")
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

	parser := parser{}
	page := parser.parseMdoc(data)
	page.mergeSpans()
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
