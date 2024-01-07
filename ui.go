package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	listview "github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

type panel int

const (
	nav panel = iota
	contents
	search
)

type searchResult struct {
	row, col, len int
}

type searchState struct {
	results []searchResult
	current int // index of currently highlighted result
}

type model struct {
	page         manPage
	lines        []string
	viewport     viewport.Model
	navigation   listview.Model
	searchbox    textinput.Model
	help         help.Model
	keys         keyMap
	searchKeys   searchKeyMap
	windowWidth  int
	windowHeight int
	focus        panel
	search       searchState
	debug        string
}

type keyMap struct {
	PageDown     key.Binding
	PageUp       key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	Down         key.Binding
	Up           key.Binding
	Navigate     key.Binding
	BeginSearch  key.Binding
	Next         key.Binding
	Previous     key.Binding
	Help         key.Binding
	Quit         key.Binding
}

type searchKeyMap struct {
	SubmitSearch key.Binding
	Cancel       key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", " ", "f"),
			key.WithHelp("f/pgdn", "page down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("b/pgup", "page up"),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("u", "ctrl+u"),
			key.WithHelp("u", "½ page up"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("d", "ctrl+d"),
			key.WithHelp("d", "½ page down"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Navigate: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "navigate"),
		),
		BeginSearch: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Next: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next"),
		),
		Previous: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "previous"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Navigate,
		k.BeginSearch,
		k.Down,
		k.Up,
		k.Next,
		k.Previous,
		k.Help,
		k.Quit,
	}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			k.Navigate,
			k.BeginSearch,
		}, {
			k.PageDown,
			k.PageUp,
		}, {
			k.HalfPageUp,
			k.HalfPageDown,
		}, {
			k.Down,
			k.Up,
		}, {
			k.Next,
			k.Previous,
		}, {
			k.Help,
			k.Quit,
		},
	}
}

func defaultSearchKeyMap() searchKeyMap {
	return searchKeyMap{
		SubmitSearch: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "submit"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

func (sk searchKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		sk.SubmitSearch,
		sk.Cancel,
	}
}

func (sk searchKeyMap) FullHelp() [][]key.Binding {
	return nil
}

var (
	scrollPctStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	tocItemStyle         = lipgloss.NewStyle()
	selectedTocItemStyle = tocItemStyle.Copy().Foreground(lipgloss.Color("#ae00ff"))

	focusColor = lipgloss.Color("#64708d")

	titleStyle             = lipgloss.NewStyle().Padding(0, 1).Margin(1, 0)
	focusNavTitleStyle     = titleStyle.Copy().Background(focusColor).Foreground(lipgloss.Color("#ddd"))
	unfocusedNavTitleStyle = titleStyle.Copy().Background(lipgloss.Color("#282a2e")).Foreground(lipgloss.Color("#888"))
)

type navItem string

func (n navItem) FilterValue() string { return string(n) }

type navItemDelegate struct{}

func (navItemDelegate) Height() int  { return 1 }
func (navItemDelegate) Spacing() int { return 0 }
func (navItemDelegate) Update(_ tea.Msg, _ *listview.Model) tea.Cmd {
	return nil
}
func (navItemDelegate) Render(w io.Writer, m listview.Model, index int, listItem listview.Item) {
	i, ok := listItem.(navItem)
	if !ok {
		return
	}

	str := fmt.Sprintf("%s", i)

	if index == m.Index() {
		fmt.Fprint(w, selectedTocItemStyle.Render(str))
	} else {
		fmt.Fprint(w, tocItemStyle.Render(str))
	}
}

func NewModel(page manPage) *model {
	m := &model{
		page:       page,
		help:       help.New(),
		keys:       defaultKeyMap(),
		searchKeys: defaultSearchKeyMap(),
		focus:      contents,
		navigation: buildTableOfContents(page),
		viewport:   viewport.New(0, 0),
		searchbox:  buildSearchBox(),
		debug:      "debug text",
	}

	return m
}

func buildSearchBox() textinput.Model {
	t := textinput.New()
	t.Prompt = "Search: "
	t.Width = 60
	t.TextStyle = lipgloss.NewStyle().Background(focusColor).Foreground(lipgloss.Color("#fff"))
	t.Cursor.TextStyle = t.TextStyle
	return t
}

func buildTableOfContents(page manPage) listview.Model {
	var sections []listview.Item
	for _, section := range page.Sections {
		sections = append(sections, navItem(section.Name))

		for _, content := range section.Contents {
			if span, ok := content.(textSpan); ok && span.Typ == tagSubsectionHeader {
				text := strings.TrimSuffix(span.Text, ":")
				sections = append(sections, navItem("  "+text))
			}
		}
	}
	maxWidth := 0
	for _, item := range sections {
		maxWidth = max(maxWidth, lipgloss.Width(string(item.(navItem))))
	}
	navigation := listview.New(sections, navItemDelegate{}, maxWidth, 100)

	navigation.SetShowTitle(false)
	navigation.SetShowStatusBar(false)
	navigation.SetShowHelp(false)
	navigation.SetFilteringEnabled(false)

	return navigation
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.focus == search {
			switch {
			case key.Matches(msg, m.searchKeys.Cancel):
				m.focus = contents
				m.search.current = 0
				m.searchbox.SetValue("")
				m.searchbox.Blur()
			case key.Matches(msg, m.searchKeys.SubmitSearch):
				m.focus = contents
				m.searchbox.Blur()
			default:
				m.searchbox, cmd = m.searchbox.Update(msg)
				cmds = append(cmds, cmd)
			}
			m.updateSearchResults(m.searchbox.Value())
		} else {
			switch {
			// case key.Matches(msg, m.keys.PageDown):
			// 	m.viewport.ViewDown()
			// case key.Matches(msg, m.keys.PageUp):
			// 	m.viewport.ViewUp()
			// case key.Matches(msg, m.keys.HalfPageDown):
			// 	m.viewport.HalfViewDown()
			// case key.Matches(msg, m.keys.HalfPageUp):
			// 	m.viewport.HalfViewUp()
			// case key.Matches(msg, m.keys.Down):
			// 	m.viewport.LineDown(1)
			// case key.Matches(msg, m.keys.Up):
			// 	m.viewport.LineUp(1)
			case key.Matches(msg, m.keys.Help):
				m.help.ShowAll = !m.help.ShowAll
			case key.Matches(msg, m.keys.Navigate):
				if m.focus == nav {
					m.focus = contents
				} else {
					m.focus = nav
				}
			case key.Matches(msg, m.keys.BeginSearch):
				m.focus = search
				m.searchbox.Focus()
				m.help.ShowAll = false
			case key.Matches(msg, m.keys.Next):
				m.search.current = min(m.search.current+1, len(m.search.results)-1)
				m.renderContents()
			case key.Matches(msg, m.keys.Previous):
				m.search.current = max(m.search.current-1, 0)
				m.renderContents()
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			default:
				if m.focus == nav {
					m.navigation, cmd = m.navigation.Update(msg)
					cmds = append(cmds, cmd)
				} else if m.focus == contents {
					m.viewport, cmd = m.viewport.Update(msg)
					cmds = append(cmds, cmd)
				}
			}
		}

	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height

		titleHeight := lipgloss.Height(m.titleView(nav))
		footerHeight := lipgloss.Height(m.footerView())
		verticalMargins := titleHeight + footerHeight // +1 for panel margins

		navWidth := lipgloss.Width(m.sidebarView())

		m.renderContents()

		m.viewport.Width = m.windowWidth - navWidth
		m.viewport.Height = m.windowHeight - verticalMargins

		m.navigation.SetHeight(m.windowHeight - verticalMargins)

	default:
		if m.focus == nav {
			m.navigation, cmd = m.navigation.Update(msg)
			cmds = append(cmds, cmd)
		} else if m.focus == contents {
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		} else if m.focus == search {
			m.searchbox, cmd = m.searchbox.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *model) searchForString(query string) []searchResult {
	var results []searchResult
	for row := 0; row < len(m.lines); row++ {
		col := 0
		for {
			found := strings.Index(m.lines[row][col:], query)
			if found == -1 {
				break
			}

			results = append(results, searchResult{
				row: row,
				col: col + found,
				len: len(query),
			})
			col += found + len(query) + 1
			if col > len(m.lines[row]) {
				break
			}
		}
	}
	return results
}

func (m *model) updateSearchResults(query string) {
	if query == "" {
		return
	}
	m.search.results = m.searchForString(query)
	m.renderContents()
}

func (m *model) renderContents() {
	navWidth := lipgloss.Width(m.sidebarView())
	contentWidth := m.windowWidth - navWidth

	oldLines := m.lines
	_ = oldLines

	contents := wordwrap.String(m.page.Render(contentWidth), contentWidth)
	m.lines = strings.Split(contents, "\n")
	lines := make([]string, len(m.lines))
	copy(lines, m.lines)
	if len(m.search.results) > 0 {
		result := m.search.results[m.search.current]
		m.debug = fmt.Sprintf("row[%d] col[%d]", result.row, result.col)
		line := lines[result.row]

		left := line[:result.col]
		instance := line[result.col : result.col+result.len]
		right := line[result.col+result.len:]

		highlight := lipgloss.NewStyle().Bold(true).Reverse(true).Render
		line = left + highlight(instance) + right
		lines[result.row] = line

		contents = strings.Join(lines, "\n")
	}

	m.viewport.SetContent(contents)
}

func (m model) View() string {
	return m.mainView() + "\n" + m.footerView()
}

func (m model) titleView(panel panel) string {
	style := unfocusedNavTitleStyle
	if m.focus == panel {
		style = focusNavTitleStyle
	}

	if panel == nav {
		return style.Render("Table of Contents")
	} else {
		return style.Render(fmt.Sprintf("%s(%d)", m.page.Name, m.page.Section))
	}
}

func (m model) sidebarView() string {
	style := lipgloss.NewStyle().Margin(0, 2, 0, 1)
	return style.Render(m.titleView(nav) + "\n" + m.navigation.View())
}

func (m model) contentsView() string {
	return m.titleView(contents) + "\n" + m.viewport.View()
}

/*
mainView

- sidebarView
  - title
  - navigation

- contentsView
  - title
  - viewport

- footerView
  - help
*/
func (m model) mainView() string {
	return lipgloss.JoinHorizontal(lipgloss.Top, m.sidebarView(), m.contentsView())
}

func (m model) scrollPercentageView() string {
	return scrollPctStyle.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
}

func (m model) footerView() string {
	margin := lipgloss.NewStyle().Margin(0, 1).Render // whole footer margin

	scrollPct := m.scrollPercentageView()
	leftWidth := m.windowWidth - lipgloss.Width(scrollPct) - 2
	helpStyle := lipgloss.NewStyle().Width(leftWidth).Render
	m.help.Width = leftWidth

	var left string

	if m.focus == search {
		searchState := ""
		if m.searchbox.Value() != "" {
			searchState = fmt.Sprintf("Found %d results for `%s'", len(m.search.results), m.searchbox.Value())
		}
		left = lipgloss.JoinVertical(lipgloss.Left,
			m.searchbox.View()+"     "+searchState,
			helpStyle(m.help.View(m.searchKeys)))
	} else if len(m.search.results) > 0 {
		left = lipgloss.JoinVertical(lipgloss.Left,
			fmt.Sprintf("Found %d results for `%s'", len(m.search.results), m.searchbox.Value()),
			helpStyle(m.help.View(m.keys)))
	} else {
		left = helpStyle(m.help.View(m.keys))
	}

	return margin(lipgloss.JoinHorizontal(lipgloss.Bottom, left, scrollPct)) //+ "\n" + m.debug
}
