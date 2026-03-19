package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true)

	branchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14"))

	pathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	// PR state colors
	stateOpenStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")) // cyan

	stateDraftStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")) // dim gray

	stateMergedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("10")) // green

	stateClosedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("9")) // red

	stateNoPRStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")) // yellow

	stateMainStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")). // bright blue
			Bold(true)

	// Checkbox styles
	checkboxCleanableStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	checkboxReadOnlyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("243"))

	// Footer / help
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9"))

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	// Help screen styles
	helpSectionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("14")).
				Bold(true)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("99"))

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	filterPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("11")).
				Bold(true)

	filterInputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	filterActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("11"))
)
