package app

import (
	"strings"
	"testing"

	"github.com/blake/gh-project-tui/internal/github"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestBoardMouseHitTesting(t *testing.T) {
	m := model{
		width: 80, height: 24, column: 0,
		project: &github.Project{
			Stages: []github.Stage{{Name: "Todo"}, {Name: "Doing"}, {Name: "Done"}},
			Issues: []github.Issue{
				{Number: 1, Stage: "Todo"},
				{Number: 2, Stage: "Todo"},
				{Number: 3, Stage: "Doing"},
			},
		},
	}
	layout := m.layout()
	tests := []struct {
		name        string
		x, y        int
		wantColumn  int
		wantIssue   int
		wantIssueOK bool
	}{
		{name: "first card", x: 2, y: firstCardY, wantColumn: 0, wantIssue: 0, wantIssueOK: true},
		{name: "second column card", x: layout.columnWidth + columnGap + 2, y: firstCardY, wantColumn: 1, wantIssue: 0, wantIssueOK: true},
		{name: "column separator", x: layout.columnWidth, y: firstCardY},
		{name: "column header is not a card", x: 2, y: boardHeaderY, wantColumn: 0},
		{name: "empty card slot", x: 2, y: firstCardY + 2*cardStride},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			column, issue, ok := m.issueAt(test.x, test.y, layout)
			if column != test.wantColumn || issue != test.wantIssue || ok != test.wantIssueOK {
				t.Fatalf("issueAt(%d, %d) = (%d, %d, %v), want (%d, %d, %v)",
					test.x, test.y, column, issue, ok, test.wantColumn, test.wantIssue, test.wantIssueOK)
			}
		})
	}
	if got := m.columnAtX(layout.columnWidth+columnGap+2, layout); got != 1 {
		t.Fatalf("columnAtX() = %d, want 1", got)
	}
}

func TestBoardCardRowsMatchMouseColumnWidths(t *testing.T) {
	m := model{
		width: 80, height: 24,
		project: &github.Project{
			Stages: []github.Stage{{Name: "Todo"}, {Name: "Doing"}, {Name: "Done"}},
			Issues: []github.Issue{{Number: 1, Title: "Example", Stage: "Todo"}},
		},
	}
	lines := strings.Split(m.boardView(), "\n")
	for y := firstCardY; y < firstCardY+cardHeight; y++ {
		if got := lipgloss.Width(lines[y]); got != 80 {
			t.Errorf("rendered card row %d has width %d, want 80", y, got)
		}
	}
}

func TestBoardViewShowsIssueCardFields(t *testing.T) {
	m := model{
		width: 80, height: 24,
		project: &github.Project{
			Title:  "Roadmap",
			Stages: []github.Stage{{Name: "Todo"}},
			Issues: []github.Issue{{
				Number: 42, Title: "Improve the board", Stage: "Todo",
				Assignees: []string{"octocat"},
			}},
		},
	}
	view := m.boardView()
	for _, want := range []string{"#42", "Improve the board", "@octocat"} {
		if !strings.Contains(view, want) {
			t.Errorf("boardView() does not show %q", want)
		}
	}
}

func TestSelectedCardUsesDistinctTextColor(t *testing.T) {
	for _, selected := range []bool{false, true} {
		style := cardTextStyle(selected, "236", "255")
		foreground, ok := style.GetForeground().(lipgloss.Color)
		if !ok {
			t.Fatalf("GetForeground() = %T, want lipgloss.Color", style.GetForeground())
		}
		if selected && foreground != lipgloss.Color("183") {
			t.Errorf("selected card foreground = %q, want highlight color", foreground)
		}
		if !selected && foreground != lipgloss.Color("255") {
			t.Errorf("unselected card foreground = %q, want default color", foreground)
		}
	}
}

func TestSelectedColumnTitleIsBold(t *testing.T) {
	for _, selected := range []bool{false, true} {
		style := cardTextStyle(selected, "238", "255")
		if got := style.GetBold(); got != selected {
			t.Errorf("column bold = %v when selected=%v", got, selected)
		}
	}
}

func TestDraggingIssueSelectsDropColumn(t *testing.T) {
	m := model{
		screen: boardScreen, width: 80, height: 24,
		project: &github.Project{
			ID: "project", StatusFieldID: "status",
			Stages: []github.Stage{{ID: "todo", Name: "Todo"}, {ID: "done", Name: "Done"}},
			Issues: []github.Issue{{ItemID: "item", Number: 7, Stage: "Todo"}},
		},
	}
	layout := m.layout()
	pressed, _ := m.updateMouse(tea.MouseMsg{
		X: 2, Y: firstCardY, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	})
	dragging := pressed.(model)
	if !dragging.dragging || dragging.dragIssue.Number != 7 {
		t.Fatalf("press did not start dragging issue: %+v", dragging)
	}

	moved, _ := dragging.updateMouse(tea.MouseMsg{
		X: layout.columnWidth + columnGap + 2, Y: boardHeaderY, Action: tea.MouseActionMotion,
	})
	targeted := moved.(model)
	if targeted.dragTarget != 1 {
		t.Fatalf("drag target = %d, want column 1", targeted.dragTarget)
	}

	dropped, cmd := targeted.updateMouse(tea.MouseMsg{
		X: layout.columnWidth + columnGap + 2, Y: boardHeaderY, Action: tea.MouseActionRelease,
	})
	result := dropped.(model)
	if cmd == nil || result.screen != boardScreen || result.column != 0 || result.dragging || !result.busy {
		t.Fatalf("release did not submit the move: screen=%v column=%d dragging=%v busy=%v cmd=%v",
			result.screen, result.column, result.dragging, result.busy, cmd != nil)
	}
}

func TestBoardShowsSpinnerWithoutReplacingScreen(t *testing.T) {
	m := model{
		screen: boardScreen, width: 80, height: 24, busy: true,
		spinner: spinner.New(),
		project: &github.Project{
			Title:  "Roadmap",
			Stages: []github.Stage{{Name: "Todo"}},
		},
	}
	view := m.View()
	if !strings.Contains(view, m.spinner.View()) {
		t.Fatal("busy board does not show the spinner")
	}
	if strings.Contains(view, "Connecting to GitHub") {
		t.Fatal("busy board was replaced with the old loading screen")
	}
	if m.screen != boardScreen {
		t.Fatalf("screen changed while operation was in progress: %v", m.screen)
	}
}

func TestActionResultKeepsCurrentScreenUntilRefreshCompletes(t *testing.T) {
	m := model{screen: commentScreen, busy: true}
	updated, cmd := m.Update(actionMessage{})
	result := updated.(model)
	if result.screen != commentScreen || !result.busy || cmd == nil {
		t.Fatalf("action result changed screen or stopped spinner before refresh: screen=%v busy=%v cmd=%v",
			result.screen, result.busy, cmd != nil)
	}
}
