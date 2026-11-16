package main

import (
	"fmt"
	"os"

	"gdocker/docker"
	_ "gdocker/ui" // Import for side effects (init function)

	tea "charm.land/bubbletea/v2"
)

func main() {
	m, err := docker.InitialModel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
