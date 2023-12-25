package main

import "fmt"

func (page manPage) render() string {
	res := ""
	for _, section := range page.sections {
		res += fmt.Sprintf("%s\n", section.name)
	}
	return res
}
