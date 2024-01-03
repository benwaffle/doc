package main

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var sectionHeader = lipgloss.NewStyle().
	Bold(true).
	BorderStyle(lipgloss.RoundedBorder()).
	BorderBottom(true)

func (page manPage) render() string {
	res := ""
	for i, section := range page.Sections {
		if i != 0 {
			res += "\n\n"
		}
		res += fmt.Sprintf("%s\n", sectionHeader.Render(section.Name))
		for _, content := range section.Contents {
			res += content.Render()
			res += " "
		}
	}
	return res
}
