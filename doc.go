package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

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
	fmt.Println(findDoc(target))
}
