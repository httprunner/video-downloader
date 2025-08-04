package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the main application state
type Model struct {
	state     State
	urlInput  textinput.Model
	table     table.Model
	downloads []Download
	width     int
	height    int
	styles    Styles
}

// State represents different screens/states of the TUI
type State int

const (
	MainMenu State = iota
	DownloadScreen
	BatchDownload
	Settings
	Downloads
	Help
)

// Download represents a download entry
type Download struct {
	ID       string
	Platform string
	Title    string
	Author   string
	Status   string
	Progress int
}

// Styles holds all the styling for the TUI
type Styles struct {
	title        lipgloss.Style
	subtitle     lipgloss.Style
	menuItem     lipgloss.Style
	selectedItem lipgloss.Style
	input        lipgloss.Style
	button       lipgloss.Style
	statusBar    lipgloss.Style
	table        lipgloss.Style
}

// InitialModel creates the initial model for the TUI
func InitialModel() Model {
	// Initialize text input
	ti := textinput.New()
	ti.Placeholder = "Enter video URL..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 50

	// Initialize table
	columns := []table.Column{
		{Title: "Platform", Width: 10},
		{Title: "Title", Width: 30},
		{Title: "Author", Width: 20},
		{Title: "Status", Width: 15},
		{Title: "Progress", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Initialize styles
	styles := Styles{
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			PaddingTop(1).
			PaddingBottom(1),
		subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			PaddingBottom(1),
		menuItem: lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingRight(2).
			Margin(0, 1),
		selectedItem: lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingRight(2).
			Margin(0, 1).
			Background(lipgloss.Color("#7D56F4")).
			Foreground(lipgloss.Color("#FFFFFF")),
		input: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1),
		button: lipgloss.NewStyle().
			Background(lipgloss.Color("#7D56F4")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 2).
			Margin(1),
		statusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Background(lipgloss.Color("#F8F8F8")).
			Padding(0, 1),
		table: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")),
	}

	return Model{
		state:     MainMenu,
		urlInput:  ti,
		table:     t,
		downloads: []Download{},
		styles:    styles,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "esc":
			if m.state != MainMenu {
				m.state = MainMenu
				return m, nil
			}

		case "1":
			if m.state == MainMenu {
				m.state = DownloadScreen
				return m, nil
			}

		case "2":
			if m.state == MainMenu {
				m.state = BatchDownload
				return m, nil
			}

		case "3":
			if m.state == MainMenu {
				m.state = Downloads
				return m, nil
			}

		case "4":
			if m.state == MainMenu {
				m.state = Settings
				return m, nil
			}

		case "5":
			if m.state == MainMenu {
				m.state = Help
				return m, nil
			}

		case "enter":
			if m.state == DownloadScreen && m.urlInput.Value() != "" {
				// Here we would trigger the download
				// For now, just add a mock download to the table
				download := Download{
					ID:       fmt.Sprintf("dl_%d", len(m.downloads)+1),
					Platform: "TikTok",
					Title:    "Sample Video",
					Author:   "SampleUser",
					Status:   "Downloading",
					Progress: 0,
				}
				m.downloads = append(m.downloads, download)
				m.updateTable()
				m.urlInput.SetValue("")
				return m, nil
			}
		}
	}

	// Update components based on current state
	switch m.state {
	case DownloadScreen:
		m.urlInput, cmd = m.urlInput.Update(msg)
	case Downloads:
		m.table, cmd = m.table.Update(msg)
	}

	return m, cmd
}

// View renders the UI
func (m Model) View() string {
	switch m.state {
	case MainMenu:
		return m.renderMainMenu()
	case DownloadScreen:
		return m.renderDownloadScreen()
	case BatchDownload:
		return m.renderBatchDownload()
	case Downloads:
		return m.renderDownloads()
	case Settings:
		return m.renderSettings()
	case Help:
		return m.renderHelp()
	default:
		return m.renderMainMenu()
	}
}

func (m Model) renderMainMenu() string {
	title := m.styles.title.Render("Video Downloader TUI")
	subtitle := m.styles.subtitle.Render("Multi-platform video downloader for TikTok, XHS, and Kuaishou")

	menu := []string{
		"1. Single Video Download",
		"2. Batch Download",
		"3. View Downloads",
		"4. Settings",
		"5. Help",
		"",
		"q. Quit",
	}

	var menuItems []string
	for _, item := range menu {
		if item == "" {
			menuItems = append(menuItems, "")
		} else {
			menuItems = append(menuItems, m.styles.menuItem.Render(item))
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		subtitle,
		"",
		strings.Join(menuItems, "\n"),
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) renderDownloadScreen() string {
	title := m.styles.title.Render("Single Video Download")

	inputLabel := "Enter video URL:"
	input := m.styles.input.Render(m.urlInput.View())

	instructions := []string{
		"Supported platforms:",
		"• TikTok: https://www.tiktok.com/@user/video/123456",
		"• XHS: https://www.xiaohongshu.com/explore/abc123",
		"• Kuaishou: https://www.kuaishou.com/short-video/abc123",
		"",
		"Press Enter to download • ESC to go back",
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		inputLabel,
		input,
		"",
		strings.Join(instructions, "\n"),
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) renderBatchDownload() string {
	title := m.styles.title.Render("Batch Download")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		"Batch download functionality coming soon...",
		"",
		"ESC to go back",
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) renderDownloads() string {
	title := m.styles.title.Render("Downloads")

	tableView := m.styles.table.Render(m.table.View())

	instructions := "↑/↓ to navigate • ESC to go back"

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		tableView,
		"",
		instructions,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) renderSettings() string {
	title := m.styles.title.Render("Settings")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		"Settings configuration coming soon...",
		"",
		"ESC to go back",
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) renderHelp() string {
	title := m.styles.title.Render("Help")

	helpText := []string{
		"Video Downloader TUI Help",
		"",
		"Navigation:",
		"• Use number keys to select menu items",
		"• ESC to go back to main menu",
		"• q or Ctrl+C to quit",
		"",
		"Download:",
		"• Enter a valid video URL and press Enter",
		"• Supported platforms: TikTok, XHS (Xiaohongshu), Kuaishou",
		"",
		"Features:",
		"• Single video download",
		"• Batch download (coming soon)",
		"• Download history tracking",
		"• Progress monitoring",
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		strings.Join(helpText, "\n"),
		"",
		"ESC to go back",
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *Model) updateTable() {
	var rows []table.Row
	for _, download := range m.downloads {
		rows = append(rows, table.Row{
			download.Platform,
			download.Title,
			download.Author,
			download.Status,
			fmt.Sprintf("%d%%", download.Progress),
		})
	}
	m.table.SetRows(rows)
}
