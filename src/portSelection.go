package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"go.bug.st/serial"
)

var frameStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()). // Choices: Rounded, Normal, Thick, Double
	BorderForeground(lipgloss.Color("62")).
	Padding(1, 2).
	Margin(1)

type model struct {
	choices  []string
	cursor   int
	selected string
}

func initialModel() model {
	serialPorts, err := serial.GetPortsList()
	check(err)
	return model{
		choices:  serialPorts,
		selected: "",
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			// Capture the value before quitting
			m.selected = m.choices[m.cursor]
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() tea.View {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Select Serial Port") + "\n\n")

	for i, choice := range m.choices {
		cursor := " "
		style := lipgloss.NewStyle()
		if m.cursor == i {
			cursor = ">"
			style = style.Foreground(lipgloss.Color("205")).Bold(true)
		}
		b.WriteString(fmt.Sprintf("%s %s\n", cursor, style.Render(choice)))

	}

	return tea.NewView(frameStyle.Render(b.String()))
}
