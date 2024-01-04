package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var sectionHeader = lipgloss.NewStyle().
	Bold(true).
	BorderStyle(lipgloss.RoundedBorder()).
	BorderBottom(true)

func (page manPage) render(width int) string {
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
	return res
}
