package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	selectedStyle = lipgloss.NewStyle().
			Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62"))
)

func (m model) setupView() string {
	var out strings.Builder
	out.WriteString(headerStyle.Render("GitHub Projects"))
	out.WriteString("\n\n")
	if m.project == nil {
		out.WriteString("Connect to a GitHub Projects v2 board.\n")
		out.WriteString(mutedStyle.Render("Authenticate first with `gh auth login`."))
	} else {
		out.WriteString("Enter the URL of the project board to connect to.")
	}
	out.WriteString("\n\n")
	out.WriteString(m.urlInput.View())
	if m.errorText != "" {
		out.WriteString("\n\n")
		out.WriteString(errorStyle.Render(m.errorText))
	}
	out.WriteString("\n\n")
	out.WriteString(mutedStyle.Render("enter connect  •  esc cancel  •  ctrl+c quit"))
	return out.String()
}

func (m model) detailView() string {
	issue := m.editIssue
	var out strings.Builder
	out.WriteString(headerStyle.Render(fmt.Sprintf("%s  #%d", issue.Repo, issue.Number)))
	out.WriteString("\n\n")
	out.WriteString(titleStyle.Render(issue.Title))
	out.WriteString("\n")
	out.WriteString(mutedStyle.Render(fmt.Sprintf("%s · %s · %s", issue.State, issue.Stage, strings.Join(issue.Labels, ", "))))
	out.WriteString("\n")
	out.WriteString(mutedStyle.Render(issue.URL))
	out.WriteString("\n\n")
	body := strings.TrimSpace(issue.Body)
	if body == "" {
		body = mutedStyle.Render("No description.")
	} else {
		maxWidth := max(40, min(100, m.width-8))
		body = lipgloss.NewStyle().Width(maxWidth).Render(body)
	}
	out.WriteString(body)
	out.WriteString("\n\n")
	out.WriteString(mutedStyle.Render("e edit  m move stage  esc back  q quit"))
	return out.String()
}

func (m model) editView() string {
	var out strings.Builder
	out.WriteString(headerStyle.Render(fmt.Sprintf("Edit issue #%d", m.editIssue.Number)))
	out.WriteString("\n\nTitle\n")
	out.WriteString(m.titleInput.View())
	out.WriteString("\n\nDescription\n")
	out.WriteString(m.bodyInput.View())
	if m.errorText != "" {
		out.WriteString("\n")
		out.WriteString(errorStyle.Render(m.errorText))
	}
	out.WriteString("\n\n")
	out.WriteString(mutedStyle.Render("tab switch field  ctrl+s save  esc cancel"))
	return out.String()
}

func (m model) moveView() string {
	var out strings.Builder
	out.WriteString(headerStyle.Render(fmt.Sprintf("Move issue #%d", m.editIssue.Number)))
	out.WriteString("\n\n")
	for i, stage := range m.project.Stages {
		line := stage.Name
		if i == m.moveTo {
			line = selectedStyle.Render("› " + line)
		} else {
			line = "  " + line
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	out.WriteString("\n")
	out.WriteString(mutedStyle.Render("↑/↓ choose stage  enter move  esc cancel"))
	return out.String()
}

func truncate(value string, limit int) string {
	if limit < 1 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit == 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}
