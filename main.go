package main

import (
	"fmt"
	"os"

	"gdocker/docker"
	_ "gdocker/ui" // Import for side effects (init function)

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	m, err := docker.InitialModel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
