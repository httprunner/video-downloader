package main

import (
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"video-downloader/internal/tui"
)

func main() {
	// Initialize the TUI application
	model := tui.InitialModel()

	// Create a new Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Run the program
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
