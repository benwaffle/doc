package main

import (
	"fmt"
	"io"
	"strings"

	listview "github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	page         manPage
	ready        bool
	viewport     viewport.Model
	sections     listview.Model
	windowWidth  int
	windowHeight int
}

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return titleStyle.Copy().BorderStyle(b)
	}()
)

var itemStyle = lipgloss.NewStyle()
var selectedItemStyle = itemStyle.Copy().Foreground(lipgloss.Color("#ae00ff"))

type navItem string

func (n navItem) FilterValue() string { return "" }

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
		fmt.Fprint(w, selectedItemStyle.Render(str))
	} else {
		fmt.Fprint(w, itemStyle.Render(str))
	}
}

func NewModel(page manPage) *model {
	m := &model{
		page: page,
	}
	var sections []listview.Item
	maxWidth := 0
	for _, section := range m.page.Sections {
		sections = append(sections, navItem(section.Name))
		maxWidth = max(maxWidth, lipgloss.Width(section.Name))
	}
	m.sections = listview.New(sections, navItemDelegate{}, maxWidth, 100)
	m.sections.SetShowStatusBar(false)
	m.sections.SetShowFilter(false)
	m.sections.SetShowTitle(false)
	m.sections.SetShowHelp(false)

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
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		navWidth := lipgloss.Width(m.sidebarView())
		verticalMarginHeight := headerHeight + footerHeight

		m.windowWidth = msg.Width
		m.windowHeight = msg.Height

		if !m.ready {
			// Since this program is using the full size of the viewport we
			// need to wait until we've received the window dimensions before
			// we can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we wait for them
			// here.
			m.viewport = viewport.New(m.windowWidth-navWidth, m.windowHeight-verticalMarginHeight)
			// m.viewport.YPosition = headerHeight
			// m.viewport.HighPerformanceRendering = true
			m.viewport.SetContent(m.page.render())

			m.ready = true
			// This is only necessary for high performance rendering, which in
			// most cases you won't need.
			//
			// Render the viewport one line below the header.
			// m.viewport.YPosition = headerHeight + 1
		} else {
			m.viewport.Width = msg.Width - navWidth
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

		m.sections.SetHeight(msg.Height - verticalMarginHeight)

		// cmds = append(cmds, viewport.Sync(m.viewport))

	}

	m.sections, cmd = m.sections.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	return m.headerView() + "\n" + m.mainView() + "\n" + m.footerView()
}

func (m model) headerView() string {
	name := titleStyle.Render(m.page.Name)
	section := titleStyle.Render(fmt.Sprintf("Section %d", m.page.Section))
	date := titleStyle.Render(m.page.Date)

	line := strings.Repeat("─", max(0, m.windowWidth-lipgloss.Width(name)-lipgloss.Width(section)-lipgloss.Width(date)-2))
	return lipgloss.JoinHorizontal(lipgloss.Center, name, "─", section, "─", date, line)
}

var border = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderRight(true).
	PaddingLeft(1).
	PaddingRight(1).
	MarginRight(1)

func (m model) sidebarView() string {
	return border.Render(m.sections.View())
}

func (m model) mainView() string {
	return lipgloss.JoinHorizontal(lipgloss.Top, m.sidebarView(), m.viewport.View())
}

func (m model) footerView() string {
	info := infoStyle.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	line := strings.Repeat("─", max(0, m.windowWidth-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}
