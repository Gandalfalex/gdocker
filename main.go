package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"gdocker/app"
)

func main() {
	m, err := app.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
