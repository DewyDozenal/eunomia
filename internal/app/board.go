package app

import (
	"fmt"
	"strings"

	"github.com/blake/gh-project-tui/internal/github"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	boardHeaderY = 4
	firstCardY   = boardHeaderY + 1
	cardHeight   = 3
	cardStride   = 4
	columnGap    = 1
)

type boardLayout struct {
	start       int
	end         int
	columnWidth int
	visibleRows int
}

func (m model) layout() boardLayout {
	width := m.width
	if width < 1 {
		width = 100
	}
	stageCount := 0
	if m.project != nil {
		stageCount = len(m.project.Stages)
	}
	visible := max(1, min(stageCount, width/26))
	columnWidth := max(20, (width-columnGap*(visible-1))/visible)
	if columnWidth > 34 {
		columnWidth = 34
		visible = max(1, min(stageCount, width/(columnWidth+columnGap)))
	}
	start := max(0, min(m.column-visible+1, stageCount-visible))
	height := m.height
	if height < 1 {
		height = 24
	}
	rows := max(1, (height-firstCardY-3)/cardStride)
	return boardLayout{
		start: start, end: min(stageCount, start+visible),
		columnWidth: columnWidth, visibleRows: rows,
	}
}

func (m model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	layout := m.layout()
	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft {
			return m, nil
		}
		column, issueIndex, ok := m.issueAt(msg.X, msg.Y, layout)
		if !ok {
			return m, nil
		}
		m.column, m.issue = column, issueIndex
		m.dragIssue = m.issuesInColumn(column)[issueIndex]
		m.dragging, m.dragTarget = true, column
		return m, nil
	case tea.MouseActionMotion:
		if m.dragging {
			m.dragTarget = m.columnAtX(msg.X, layout)
		}
		return m, nil
	case tea.MouseActionRelease:
		if !m.dragging {
			return m, nil
		}
		target := m.columnAtX(msg.X, layout)
		lastCardY := firstCardY + (layout.visibleRows-1)*cardStride + cardHeight - 1
		if msg.Y < boardHeaderY || msg.Y > lastCardY {
			target = -1
		}
		m.dragging, m.dragTarget = false, -1
		if target < 0 || target == m.column || target >= len(m.project.Stages) {
			return m, nil
		}
		stage := m.project.Stages[target]
		m.column, m.issue = target, 0
		if stage.Name == m.dragIssue.Stage {
			return m, nil
		}
		client, projectID, fieldID, issue := m.client, m.project.ID, m.project.StatusFieldID, m.dragIssue
		m.screen, m.errorText = loadingScreen, ""
		return m, func() tea.Msg {
			return actionMessage{err: client.MoveIssue(projectID, fieldID, issue, stage)}
		}
	}
	return m, nil
}

func (m model) columnAtX(x int, layout boardLayout) int {
	if x < 0 || layout.columnWidth < 1 {
		return -1
	}
	stride := layout.columnWidth + columnGap
	visibleIndex := x / stride
	if x%stride >= layout.columnWidth || layout.start+visibleIndex >= layout.end {
		return -1
	}
	return layout.start + visibleIndex
}

func (m model) issueAt(x, y int, layout boardLayout) (int, int, bool) {
	column := m.columnAtX(x, layout)
	if column < 0 || y < firstCardY {
		return 0, 0, false
	}
	row := (y - firstCardY) / cardStride
	if (y-firstCardY)%cardStride >= cardHeight || row >= layout.visibleRows {
		return 0, 0, false
	}
	offset := 0
	if column == m.column {
		offset = max(0, m.issue-layout.visibleRows+1)
	}
	issueIndex := offset + row
	if issueIndex >= len(m.issuesInColumn(column)) {
		return 0, 0, false
	}
	return column, issueIndex, true
}

func (m model) boardView() string {
	if m.project == nil {
		return headerStyle.Render("GitHub Projects") + "\n\n" + mutedStyle.Render("No project loaded.")
	}
	layout := m.layout()
	if layout.start >= layout.end {
		return headerStyle.Render("GitHub Projects") + "\n\n" + mutedStyle.Render("This project has no status columns.")
	}
	var out strings.Builder
	out.WriteString(headerStyle.Render("GitHub Projects"))
	out.WriteString("  ")
	out.WriteString(titleStyle.Render(m.project.Title))
	out.WriteString("\n")
	out.WriteString(mutedStyle.Render(m.projectURL))
	out.WriteString("\n")
	if m.errorText != "" {
		out.WriteString(errorStyle.Render(m.errorText))
	} else if m.dragging {
		out.WriteString(mutedStyle.Render("Drop the card on a status column to move it."))
	}
	out.WriteString("\n\n")

	for visibleIndex := layout.start; visibleIndex < layout.end; visibleIndex++ {
		if visibleIndex > layout.start {
			out.WriteByte(' ')
		}
		stage := m.project.Stages[visibleIndex]
		label := fmt.Sprintf("%s (%d)", stage.Name, len(m.issuesInColumn(visibleIndex)))
		background := "238"
		if m.dragging && m.dragTarget == visibleIndex {
			background = "62"
		}
		out.WriteString(renderCell(label, layout.columnWidth, background, "255", true))
	}
	for row := 0; row < layout.visibleRows; row++ {
		issuesByColumn := make([][]github.Issue, layout.end-layout.start)
		indexByColumn := make([]int, len(issuesByColumn))
		for visibleIndex := layout.start; visibleIndex < layout.end; visibleIndex++ {
			issuesByColumn[visibleIndex-layout.start] = m.issuesInColumn(visibleIndex)
			if visibleIndex == m.column {
				indexByColumn[visibleIndex-layout.start] = max(0, m.issue-layout.visibleRows+1) + row
			} else {
				indexByColumn[visibleIndex-layout.start] = row
			}
		}
		for cardLine := 0; cardLine < cardHeight; cardLine++ {
			out.WriteByte('\n')
			for visibleIndex := layout.start; visibleIndex < layout.end; visibleIndex++ {
				columnOffset := visibleIndex - layout.start
				if columnOffset > 0 {
					out.WriteByte(' ')
				}
				issueIndex := indexByColumn[columnOffset]
				issues := issuesByColumn[columnOffset]
				if issueIndex >= len(issues) {
					out.WriteString(strings.Repeat(" ", layout.columnWidth))
					continue
				}
				issue := issues[issueIndex]
				background, foreground := "236", "255"
				if issueIndex%2 == 1 {
					background, foreground = "240", "16"
				}
				selected := visibleIndex == m.column && issueIndex == m.issue
				assignees := "Unassigned"
				if len(issue.Assignees) > 0 {
					names := make([]string, len(issue.Assignees))
					for i, name := range issue.Assignees {
						names[i] = "@" + name
					}
					assignees = strings.Join(names, ", ")
				}
				lines := []string{fmt.Sprintf("#%d", issue.Number), issue.Title, assignees}
				out.WriteString(renderCell(lines[cardLine], layout.columnWidth, background, foreground, selected))
			}
		}
		if row+1 < layout.visibleRows {
			out.WriteByte('\n')
			out.WriteString(strings.Repeat(" ", (layout.end-layout.start)*layout.columnWidth+(layout.end-layout.start-1)*columnGap))
		}
	}
	out.WriteString("\n\n")
	out.WriteString(mutedStyle.Render("←/→ column  ↑/↓ issue  enter view  e edit  m move  r refresh  c change project  q quit"))
	out.WriteString("\n")
	out.WriteString(mutedStyle.Render("Mouse: drag an issue card onto another status column to move it."))
	return out.String()
}

func renderCell(text string, width int, background, foreground string, bold bool) string {
	style := lipgloss.NewStyle().Width(width).Background(lipgloss.Color(background)).Foreground(lipgloss.Color(foreground))
	if bold {
		style = style.Bold(true)
	}
	return style.Render(truncate(text, width))
}

func (m model) issuesInColumn(index int) []github.Issue {
	if m.project == nil || index < 0 || index >= len(m.project.Stages) {
		return nil
	}
	var issues []github.Issue
	for _, issue := range m.project.Issues {
		if issue.Stage == m.project.Stages[index].Name {
			issues = append(issues, issue)
		}
	}
	return issues
}

func (m model) selectedIssue() (github.Issue, bool) {
	issues := m.issuesInColumn(m.column)
	if m.issue < 0 || m.issue >= len(issues) {
		return github.Issue{}, false
	}
	return issues[m.issue], true
}
