package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	listview "github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

type panel int

const (
	nav panel = iota
	contents
)

type model struct {
	page         manPage
	ready        bool
	viewport     viewport.Model
	navigation   listview.Model
	help         help.Model
	keys         keyMap
	windowWidth  int
	windowHeight int
	focus        panel
}

type keyMap struct {
	PageDown     key.Binding
	PageUp       key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	Down         key.Binding
	Up           key.Binding
	Navigate     key.Binding
	Search       key.Binding
	Help         key.Binding
	Quit         key.Binding
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
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Navigate,
		k.Search,
		k.Down,
		k.Up,
		k.Help,
		k.Quit,
	}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			k.PageDown,
			k.PageUp,
		}, {
			k.HalfPageUp,
			k.HalfPageDown,
		}, {
			k.Down,
			k.Up,
		}, {
			k.Help,
			k.Quit,
		},
	}
}

var (
	scrollPctStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

	tocItemStyle         = lipgloss.NewStyle()
	selectedTocItemStyle = tocItemStyle.Copy().Foreground(lipgloss.Color("#ae00ff"))

	focusNavTitleStyle     = lipgloss.NewStyle().Background(lipgloss.Color("#64708d")).Foreground(lipgloss.Color("#ddd")).Padding(0, 1).Margin(1, 0)
	unfocusedNavTitleStyle = lipgloss.NewStyle().Background(lipgloss.Color("#282a2e")).Foreground(lipgloss.Color("#888")).Padding(0, 1).Margin(1, 0)
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
		page:  page,
		help:  help.New(),
		keys:  defaultKeyMap(),
		focus: nav,
	}

	var sections []listview.Item
	for _, section := range m.page.Sections {
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
	m.navigation = listview.New(sections, navItemDelegate{}, maxWidth, 100)

	m.navigation.SetShowTitle(false)
	m.navigation.SetShowStatusBar(false)
	m.navigation.SetShowHelp(false)
	m.navigation.SetFilteringEnabled(false)

	return m
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
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height

		footerHeight := lipgloss.Height(m.footerView())
		navWidth := lipgloss.Width(m.sidebarView())
		verticalMarginHeight := footerHeight + 4 // TODO: 4 is for the titles
		contentWidth := m.windowWidth - navWidth

		if !m.ready {
			// Since this program is using the full size of the viewport we
			// need to wait until we've received the window dimensions before
			// we can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we wait for them
			// here.
			m.viewport = viewport.New(contentWidth, m.windowHeight-verticalMarginHeight)
			// m.viewport.YPosition = headerHeight
			// m.viewport.HighPerformanceRendering = true
			m.viewport.SetContent(wordwrap.String(m.page.render(contentWidth), m.windowWidth-navWidth))

			m.ready = true
			// This is only necessary for high performance rendering, which in
			// most cases you won't need.
			//
			// Render the viewport one line below the header.
			// m.viewport.YPosition = headerHeight + 1
		} else {
			m.viewport.Width = m.windowWidth - navWidth
			m.viewport.Height = m.windowHeight - verticalMarginHeight
		}

		m.navigation.SetHeight(m.windowHeight - verticalMarginHeight)
		m.help.Width = m.windowWidth

		// cmds = append(cmds, viewport.Sync(m.viewport))

		// default:
		// 	m.viewport, cmd = m.viewport.Update(msg)
		// 	cmds = append(cmds, cmd)

		// 	m.navigation, cmd = m.navigation.Update(msg)
		// 	cmds = append(cmds, cmd)
	}

	if m.focus == nav {
		m.navigation, cmd = m.navigation.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	return m.mainView() + "\n" + m.footerView()
}

var panelStyle = lipgloss.NewStyle().Margin(1)

func (m model) sidebarView() string {
	var title string
	if m.focus == nav {
		title = focusNavTitleStyle.Render("Table of Contents")
	} else {
		title = unfocusedNavTitleStyle.Render("Table of Contents")
	}

	return panelStyle.Render(title + "\n" + m.navigation.View())
}

func (m model) contentsView() string {
	var title string
	if m.focus == contents {
		title = focusNavTitleStyle.Render(fmt.Sprintf("%s(%d)", m.page.Name, m.page.Section))
	} else {
		title = unfocusedNavTitleStyle.Render(fmt.Sprintf("%s(%d)", m.page.Name, m.page.Section))
	}

	return panelStyle.Render(title + "\n" + m.viewport.View())
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

func (m model) footerView() string {
	info := scrollPctStyle.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	help := m.help.View(m.keys)

	remainingWidth := m.windowWidth - lipgloss.Width(info) - 1
	helpStyle := lipgloss.NewStyle().
		MarginBottom(1).
		PaddingLeft(2).
		Width(remainingWidth)

	return lipgloss.JoinHorizontal(lipgloss.Bottom, helpStyle.Render(help), info)
}
