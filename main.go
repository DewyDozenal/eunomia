package main

import (
	"fmt"
	"os"

	"github.com/blake/gh-project-tui/internal/app"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	program := tea.NewProgram(app.New(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
