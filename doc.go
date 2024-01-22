package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/shlex"
)

type manPage struct {
	Name     string
	Section  int
	Date     string
	Sections []section
	Extra    string
}

type section struct {
	Name     string
	Contents []Span
}

type textTag int

const (
	tagPlain textTag = iota
	tagNameRef
	tagArg
	tagEnvVar
	tagVariable
	tagPath
	tagSubsectionHeader
	tagLiteral
	tagSymbolic
	tagStandard
	tagBold
	tagItalic
	tagUnderline
	tagSingleQuote
	tagDoubleQuote
	tagTableCellSeparator
)

type textSpan struct {
	Typ     textTag
	Text    string
	NoSpace bool // Set to false by default
}

type decorationTag int

const (
	decorationNone decorationTag = iota
	decorationOptional
	decorationParens
	decorationSingleQuote
	decorationDoubleQuote
	decorationQuotedLiteral
)

type decoratedSpan struct {
	Typ      decorationTag
	Contents []Span
}

type flagSpan struct {
	Flag    string
	Dash    bool
	NoSpace bool // Set to false by default
}

type manRef struct {
	Name    string
	Section *int
}

type listType int

const (
	bulletList listType = iota // Bullet item list
	dashList                   // Hyphenated list
	itemList                   // Unlabeled list
	enumList                   // Enumerated list
	tagList                    // Tag labeled list
	diagList                   // Diagnostic list
	hangList                   // Hanging labeled list
	ohangList                  // Overhanging labeled list
	insetList                  // Inset or run-on labeled list
	columnList                 // Columnar list (table)
)

type list struct {
	Typ     listType
	Items   []listItem
	Compact bool
	Width   int
	Columns []string
	Indent  int
}

type listItem struct {
	Tag      []Span
	Contents []Span
}

type font int

const (
	fontPlain font = iota // Roman
	fontBold
	fontItalic
)

type parser struct {
	lastFont    font
	currentFont font
}

func parseError(line int, info string, err error) error {
	return fmt.Errorf("Error parsing %s on line %d: %w", info, line, err)
}

// Merge adjacent spans if possible. This makes ast.json much easier to read.
func (page *manPage) mergeSpans() {
	for i, section := range page.Sections {

		var contents []Span
		var merged *textSpan = nil
		for _, span := range section.Contents {

			if merged == nil { // new range
				if ts, ok := span.(textSpan); ok {
					merged = &ts
				} else {
					contents = append(contents, span)
				}
			} else { // try merge
				// TODO: merge list contents
				if next, ok := span.(textSpan); ok && next.Typ == merged.Typ && next.NoSpace == merged.NoSpace { // ok to merge
					mergedText := merged.Text
					if !next.NoSpace {
						mergedText += " "
					}
					mergedText += next.Text
					merged = &textSpan{
						Typ:     merged.Typ,
						Text:    mergedText,
						NoSpace: merged.NoSpace,
					}
				} else { // no match, don't merge
					contents = append(contents, *merged, span)
					merged = nil
				}
			}

		}
		if merged != nil {
			contents = append(contents, merged)
		}
		section.Contents = contents
		page.Sections[i] = section

	}
}

func nextToken(input string) (string, string) {
	if len(input) == 0 {
		return "", ""
	}

	inQuote := false
	token := ""

	for i, c := range input {
		if c == '\\' && i+1 < len(input) && input[i+1] == 'f' { // font sequence, this will be the next token
			if inQuote {
				token += "\\"
			} else if i == 0 {
				return input[:3], input[3:] // \fX is the current token
			} else {
				return token, input[i:] // \fX will be the next token
			}
		} else if c == '\\' {
			// don't add \
		} else if c == '"' && !inQuote { // start quoted words
			inQuote = true
		} else if c == '"' && inQuote { // end quoted words
			inQuote = false
		} else if c == ' ' && !inQuote {
			return token, input[i+1:]
		} else {
			token += string(c)
		}
	}
	return token, ""
}

func (p *parser) parseLine(line string) []Span {
	if line == "" {
		return nil
	}

	var res []Span
	lastMacro := ""
	repeatMacro := false

tokenizer:
	for {
		token, rest := nextToken(line)
		if token == "" && len(rest) > 0 { // eat spaces
			line = rest
			continue
		}
		switch token {
		case "Fl": // command line flag with dash
			flag, rest := nextToken(rest)
			res = append(res, flagSpan{flag, true, false})
			line = rest
			lastMacro = "Fl"
		case "Cm", "Ic": // command line something with no dash
			flag, rest := nextToken(rest)
			res = append(res, flagSpan{flag, false, false})
			line = rest
			lastMacro = "Cm"
		case "Ar": // command line argument
			arg, rest := nextToken(rest)
			if arg == "" {
				arg = "file ..."
			}
			res = append(res, textSpan{tagArg, arg, false})
			line = rest
			lastMacro = "Ar"
		case "Ev": // environment variable
			env, rest := nextToken(rest)
			res = append(res, textSpan{tagEnvVar, env, false})
			line = rest
			lastMacro = "Ev"
		case "Va", "Dv": // variable
			vari, rest := nextToken(rest)
			res = append(res, textSpan{tagVariable, vari, false})
			line = rest
			lastMacro = "Va"
		case "Pa": // path
			pa, rest := nextToken(rest)
			res = append(res, textSpan{tagPath, pa, false})
			line = rest
			lastMacro = "Pa"
		case "Sy": // symbolic
			sym, rest := nextToken(rest)
			res = append(res, textSpan{tagSymbolic, sym, false})
			line = rest
			lastMacro = "Sy"
		case "Li": // literal
			literal, rest := nextToken(rest)
			res = append(res, textSpan{tagLiteral, literal, false})
			line = rest
			lastMacro = "Li"
		case "St": // standard
			standard, rest := nextToken(rest)
			res = append(res, textSpan{tagStandard, standard, false})
			line = rest
			lastMacro = "St"
		case "Ta": // table cell separator
			res = append(res, textSpan{tagTableCellSeparator, "", false})
			line = rest
			lastMacro = "Ta"
		case "No": // no format
			no, rest := nextToken(rest)
			res = append(res, textSpan{tagPlain, no, false})
			line = rest
			lastMacro = "No"
		case "B": // bold
			bold, rest := nextToken(rest)
			res = append(res, textSpan{tagBold, bold, false})
			line = rest
			lastMacro = "B"
		case "I": // italic
			italic, rest := nextToken(rest)
			res = append(res, textSpan{tagItalic, italic, false})
			line = rest
			lastMacro = "I"
		case "Em": // emphasis or underline
			em, rest := nextToken(rest)
			res = append(res, textSpan{tagUnderline, em, false})
			line = rest
			lastMacro = "Em"
		case "BR": // alternate bold and normal
			bold, rest := nextToken(rest)
			if bold != "" {
				res = append(res, textSpan{tagBold, bold, false})
				line = "RB " + rest
			} else {
				line = rest
			}
			lastMacro = "BR"
		case "RB": // alternate normal and bold
			roman, rest := nextToken(rest)
			if roman != "" {
				res = append(res, textSpan{tagPlain, roman, false})
				line = "BR " + rest
			} else {
				line = rest
			}
			lastMacro = "RB"
		case "RI": // alternate normal and italic
			roman, rest := nextToken(rest)
			if roman != "" {
				res = append(res, textSpan{tagPlain, roman, false})
				line = "IR " + rest
			} else {
				line = rest
			}
			lastMacro = "RI"
		case "IR": // alternate italic and normal
			italic, rest := nextToken(rest)
			if italic != "" {
				res = append(res, textSpan{tagItalic, italic, false})
				line = "RI " + rest
			} else {
				line = rest
			}
			lastMacro = "IR"
		case "Ns": // no space
			index := len(res) - 1
			last := res[index]
			switch span := last.(type) {
			case textSpan:
				span.NoSpace = true
				res[index] = span
			case flagSpan:
				span.NoSpace = true
				res[index] = span
			default:
				fmt.Printf("%+v\n", res)
				panic("Don't know how to handle Ns macro")
			}
			line = rest
		case "Ql": // quoted literal
			res = append(res, decoratedSpan{decorationQuotedLiteral, p.parseLine(rest)})
			break tokenizer
		case "Pq": // parens
			res = append(res, decoratedSpan{decorationParens, p.parseLine(rest)})
			break tokenizer
		case "Sq": // single quote
			res = append(res, decoratedSpan{decorationSingleQuote, p.parseLine(rest)})
			break tokenizer
		case "Dq": // double quote
			res = append(res, decoratedSpan{decorationDoubleQuote, p.parseLine(rest)})
			break tokenizer
		case "Op": // optional
			res = append(res, decoratedSpan{decorationOptional, p.parseLine(rest)})
			break tokenizer

		// escape sequences
		case "\\fB": // bold
			p.lastFont = p.currentFont
			p.currentFont = fontBold
			line = rest
		case "\\fI": // italic
			p.lastFont = p.currentFont
			p.currentFont = fontItalic
			line = rest
		case "\\fR": // plain text (roman)
			p.lastFont = p.currentFont
			p.currentFont = fontPlain
			line = rest
		case "\\fP": // use previous font
			p.currentFont = p.lastFont
			line = rest
		case "\\-", "\\,", "\\/":
			res = append(res, textSpan{tagPlain, token[1:2], true})
			line = rest

		case ",", "|":
			res = append(res, textSpan{tagPlain, token, false})
			line = rest
			repeatMacro = true
		case "":
			break tokenizer
		default:
			if repeatMacro {
				line = lastMacro + " " + line
				repeatMacro = false
			} else {
				style := tagPlain
				switch p.currentFont {
				case fontPlain:
					style = tagPlain
				case fontBold:
					style = tagBold
				case fontItalic:
					style = tagItalic
				default:
					panic(fmt.Sprintf("unknown font %d", p.currentFont))
				}
				res = append(res, textSpan{style, token, false})
				line = rest
			}
		}
	}

	return res
}

func (p *parser) parseMdoc(doc string) manPage {
	mdocTitle, _ := regexp.Compile(`\.Dt ([A-Za-z_]+) (\d+)`) // .Dt macro
	xr, _ := regexp.Compile(`\.Xr (\S+)(?: (\d+))?`)          // .Xr macro
	nameFull, _ := regexp.Compile(`\.Nm (\S+)(?: (\S+))?`)    // .Nm macro
	savedName := ""

	page := manPage{}
	var currentSection *section

	lists := stack[*list]{}

	addSpans := func(spans ...Span) {
		if lists.Len() > 0 {
			currentItem := &lists.Peek().Items[len(lists.Peek().Items)-1]
			currentItem.Contents = append(currentItem.Contents, spans...)
		} else if currentSection != nil {
			currentSection.Contents = append(currentSection.Contents, spans...)
		} else {
			panic(fmt.Sprintf("can't add [%+v], no current section", spans))
		}
	}

	for lineNo, line := range strings.Split(doc, "\n") {
		switch {

		case strings.HasPrefix(line, ".\\\"") || strings.HasPrefix(line, "'\\\""): // commenr
			// ignore

		case strings.HasPrefix(line, ".Dd"): // document date
			page.Date = line[4:]

		case mdocTitle.MatchString(line): // mdoc page title
			parts := mdocTitle.FindStringSubmatch(line)
			page.Name = parts[1]
			section, err := strconv.Atoi(parts[2])
			if err != nil {
				panic(err)
			}
			page.Section = section

		case strings.HasPrefix(line, ".TH"): // man page title
			parts, err := shlex.Split(line[4:]) // use shlex to handle quoting
			if err != nil {
				panic(err)
			}

			page.Name = parts[0]
			section, err := strconv.Atoi(parts[1])
			if err != nil {
				panic(err)
			}
			page.Section = section
			page.Date = parts[2]
			page.Extra = strings.Join(parts[3:], " ")

		case strings.HasPrefix(line, ".Sh") || strings.HasPrefix(line, ".SH"): // section header
			if currentSection != nil {
				page.Sections = append(page.Sections, *currentSection)
			}

			name := line[4:]
			name = strings.Trim(name, "\"")

			currentSection = &section{Name: name}

		case nameFull.MatchString(line): // .Nm - page name
			parts := nameFull.FindStringSubmatch(line)
			name := parts[1]
			if savedName == "" { // first invocation, save the name
				savedName = name
			}
			addSpans(textSpan{tagNameRef, name, false})
			if len(parts) > 2 && parts[2] != "" {
				addSpans(textSpan{Text: parts[2]})
			}

		case line == ".Nm": // .Nm - page name
			if currentSection.Name == "SYNOPSIS" {
				addSpans(textSpan{tagPlain, "\n", true})
			}
			addSpans(textSpan{tagNameRef, savedName, false})

		case strings.HasPrefix(line, ".Nd"): // page description
			addSpans(textSpan{Text: "– " + line[4:]})

		case strings.HasPrefix(line, ".In"): // #include
			addSpans(textSpan{Text: fmt.Sprintf("#include <%s>", line[4:])})

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
			header := strings.Trim(line[4:], "\"")
			addSpans(textSpan{tagSubsectionHeader, header, true})

		case strings.HasPrefix(line, ".Dl"): // indented literal
			addSpans(textSpan{tagPlain, "\t", false})
			addSpans(p.parseLine(line[4:])...)

		case strings.HasPrefix(line, ".IP"): // indented paragraph
			tag := ""
			indent := 0
			maxWidth := 8

			if len(line) > 3 {
				arg1, rest := nextToken(line[4:])
				if arg1 == `\(bu` {
					tag = "•"
				} else if arg1 == `\(em` {
					tag = "—"
				} else {
					tag = arg1
				}

				arg2, _ := nextToken(rest)
				if arg2 != "" {
					indentVal, err := strconv.Atoi(arg2)
					if err != nil {
						panic(parseError(lineNo+1, arg2, err))
					}
					indent = indentVal
				}
			}

			addSpans(textSpan{tagPlain, "\n" + strings.Repeat("  ", indent) + tag, false})
			if indent+len(tag)+1 > maxWidth {
				addSpans(textSpan{tagPlain, "\n" + strings.Repeat(" ", maxWidth), false}) // TODO: proper IP support, like Bl
			}

		case strings.HasPrefix(line, ".TP"):
			addSpans(textSpan{tagPlain, "\n", false})

		case strings.HasPrefix(line, ".ft"): // font
			// not supported

		case strings.HasPrefix(line, ".Bl"): // begin list
			list := list{}

			args, err := shlex.Split(line[4:])
			if err != nil {
				panic(err)
			}
			for i := 0; i < len(args); i += 1 {
				arg := args[i]

				switch arg {
				case "-bullet":
					list.Typ = bulletList
				case "-dash":
					list.Typ = dashList
				case "-enum":
					list.Typ = enumList
				case "-tag":
					list.Typ = tagList
				case "-diag":
					list.Typ = diagList
				case "-hang":
					list.Typ = hangList
				case "-ohang":
					list.Typ = ohangList
				case "-inset":
					list.Typ = insetList
				case "-column":
					list.Typ = columnList
				case "-width":
					list.Width = len(args[i+1])
				case "-compact":
					list.Compact = true
				case "-offset":
					// TODO: handle left, center, indent, indent-two, right
					i += 1
				default:
					if list.Typ == columnList {
						list.Columns = append(list.Columns, arg)
					}
				}
			}
			if list.Typ == tagList && list.Width == 0 {
				panic("missing -width argument to .Bl tag list")
			}
			lists.Push(&list)

		case strings.HasPrefix(line, ".It"): // list item
			nextItem := listItem{}
			if len(line) > 4 {
				nextItem.Tag = p.parseLine(line[4:])
			}
			lists.Peek().Items = append(lists.Peek().Items, nextItem)

		case strings.HasPrefix(line, ".El"): // end list
			endedList := lists.Pop()
			addSpans(endedList)

		case strings.HasPrefix(line, ".Os"): // OS
			// TODO: do we need this?

		case line == ".Pp" || line == ".PP":
			addSpans(textSpan{tagPlain, "\n\n", false})

		case line == ".br":
			addSpans(textSpan{tagPlain, "\n", false})

		case line == ".na":
			// TODO: something around justification. "Ragged-right text"

		case line == ".nh":
			// TODO: disable hyphenation

		case strings.HasPrefix(line, ".nr"):
			// TODO: new register

		case line == "." || line == "":
			// ignore

		case strings.HasPrefix(line, "."):
			addSpans(p.parseLine(line[1:])...)

		default:
			addSpans(p.parseLine(line)...)

		}
	}
	page.Sections = append(page.Sections, *currentSection)
	return page
}
