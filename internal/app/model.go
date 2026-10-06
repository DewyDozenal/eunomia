package app

import (
	"fmt"
	"strings"

	"github.com/blake/gh-project-tui/internal/github"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type screen int

const (
	setupScreen screen = iota
	loadingScreen
	boardScreen
	detailScreen
	editScreen
	moveScreen
)

type loadMessage struct {
	client  *github.Client
	project *github.Project
	url     string
	err     error
}

type actionMessage struct {
	err error
}

type model struct {
	screen       screen
	client       *github.Client
	project      *github.Project
	projectURL   string
	urlInput     textinput.Model
	titleInput   textinput.Model
	bodyInput    textarea.Model
	editIssue    github.Issue
	returnScreen screen
	column       int
	issue        int
	moveTo       int
	dragging     bool
	dragIssue    github.Issue
	dragTarget   int
	width        int
	height       int
	errorText    string
	infoText     string
}

func New() tea.Model {
	urlInput := textinput.New()
	urlInput.Placeholder = "https://github.com/orgs/OWNER/projects/1"
	urlInput.Prompt = "> "
	urlInput.CharLimit = 300
	urlInput.Focus()

	titleInput := textinput.New()
	titleInput.CharLimit = 256
	titleInput.Prompt = ""

	bodyInput := textarea.New()
	bodyInput.SetWidth(72)
	bodyInput.SetHeight(10)
	bodyInput.ShowLineNumbers = false

	m := model{screen: setupScreen, urlInput: urlInput, titleInput: titleInput, bodyInput: bodyInput, dragTarget: -1}
	cfg, err := loadConfig()
	if err != nil {
		m.errorText = "Could not read saved settings: " + err.Error()
		return m
	}
	if cfg.ProjectURL != "" {
		m.projectURL = cfg.ProjectURL
		m.urlInput.SetValue(cfg.ProjectURL)
		m.screen = loadingScreen
		return m
	}
	return m
}

func (m model) Init() tea.Cmd {
	if m.screen == loadingScreen {
		return m.loadProject(m.projectURL, nil)
	}
	return textinput.Blink
}

func (m model) loadProject(rawURL string, existing *github.Client) tea.Cmd {
	return func() tea.Msg {
		client := existing
		var err error
		if client == nil {
			client, err = github.NewClient()
			if err != nil {
				return loadMessage{url: rawURL, err: err}
			}
		}
		ref, err := github.ParseProjectURL(rawURL)
		if err != nil {
			return loadMessage{url: rawURL, err: err}
		}
		project, err := client.Load(ref)
		return loadMessage{client: client, project: project, url: rawURL, err: err}
	}
}

func (m model) reload() tea.Cmd {
	return m.loadProject(m.projectURL, m.client)
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.bodyInput.SetWidth(max(30, min(90, msg.Width-12)))
		m.bodyInput.SetHeight(max(5, msg.Height-15))
		return m, nil
	case loadMessage:
		if msg.err != nil {
			m.errorText = msg.err.Error()
			if m.project != nil {
				m.screen = boardScreen
			} else {
				m.screen = setupScreen
			}
			return m, nil
		}
		m.client, m.project, m.projectURL = msg.client, msg.project, msg.url
		m.screen = boardScreen
		m.errorText, m.infoText = "", ""
		m.column, m.issue = clampSelection(m.column, m.issue, m.project)
		if err := saveConfig(config{ProjectURL: m.projectURL}); err != nil {
			m.errorText = "Project loaded, but settings could not be saved: " + err.Error()
		}
		return m, nil
	case actionMessage:
		if msg.err != nil {
			m.errorText = msg.err.Error()
			if m.screen == loadingScreen {
				m.screen = boardScreen
			}
			return m, nil
		}
		m.errorText = ""
		m.screen = loadingScreen
		return m, m.reload()
	case tea.MouseMsg:
		if m.screen == boardScreen {
			return m.updateMouse(msg)
		}
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m.updateKey(msg)
	}
	return m, nil
}

func (m model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case setupScreen:
		switch msg.String() {
		case "esc":
			if m.project != nil {
				m.screen, m.errorText = boardScreen, ""
				return m, nil
			}
		case "enter":
			raw := strings.TrimSpace(m.urlInput.Value())
			if raw == "" {
				m.errorText = "Enter a GitHub Projects v2 URL."
				return m, nil
			}
			m.errorText, m.screen = "", loadingScreen
			return m, m.loadProject(raw, nil)
		case "q":
			if m.project == nil {
				return m, tea.Quit
			}
		}
		var cmd tea.Cmd
		m.urlInput, cmd = m.urlInput.Update(msg)
		return m, cmd

	case loadingScreen:
		if msg.String() == "q" || msg.String() == "esc" {
			if m.project != nil {
				m.screen = boardScreen
				m.errorText = ""
				return m, nil
			}
		}
		return m, nil

	case boardScreen:
		m.dragging, m.dragTarget = false, -1
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "r":
			m.screen, m.errorText = loadingScreen, ""
			return m, m.reload()
		case "c":
			m.urlInput.SetValue(m.projectURL)
			m.urlInput.CursorEnd()
			m.errorText, m.screen = "", setupScreen
			return m, textinput.Blink
		case "left", "h":
			if m.column > 0 {
				m.column--
				m.issue = 0
			}
		case "right", "l", "tab":
			if m.column+1 < len(m.project.Stages) {
				m.column++
				m.issue = 0
			}
		case "up", "k":
			if m.issue > 0 {
				m.issue--
			}
		case "down", "j":
			issues := m.issuesInColumn(m.column)
			if m.issue+1 < len(issues) {
				m.issue++
			}
		case "enter":
			if issue, ok := m.selectedIssue(); ok {
				m.editIssue = issue
				m.screen = detailScreen
			}
		case "e":
			if issue, ok := m.selectedIssue(); ok {
				m.beginEdit(issue)
			}
		case "m":
			if issue, ok := m.selectedIssue(); ok && len(m.project.Stages) > 0 {
				m.editIssue = issue
				m.returnScreen = boardScreen
				m.moveTo = 0
				for i, stage := range m.project.Stages {
					if stage.Name == issue.Stage {
						m.moveTo = i
						break
					}
				}
				m.screen = moveScreen
			}
		}
		return m, nil

	case detailScreen:
		switch msg.String() {
		case "esc", "backspace":
			m.screen = boardScreen
		case "e":
			m.returnScreen = detailScreen
			m.beginEdit(m.editIssue)
		case "m":
			m.returnScreen = detailScreen
			m.moveTo = 0
			for i, stage := range m.project.Stages {
				if stage.Name == m.editIssue.Stage {
					m.moveTo = i
					break
				}
			}
			m.screen = moveScreen
		case "q":
			return m, tea.Quit
		}
		return m, nil

	case editScreen:
		switch msg.String() {
		case "esc":
			m.screen = m.returnScreen
			return m, nil
		case "ctrl+s":
			title := strings.TrimSpace(m.titleInput.Value())
			if title == "" {
				m.errorText = "Issue title cannot be empty."
				return m, nil
			}
			issue := m.editIssue
			body := m.bodyInput.Value()
			m.screen, m.errorText = loadingScreen, ""
			return m, func() tea.Msg {
				return actionMessage{err: m.client.EditIssue(issue, title, body)}
			}
		case "tab":
			if m.titleInput.Focused() {
				m.titleInput.Blur()
				m.bodyInput.Focus()
			} else {
				m.bodyInput.Blur()
				m.titleInput.Focus()
			}
			return m, nil
		}
		var cmd tea.Cmd
		if m.titleInput.Focused() {
			m.titleInput, cmd = m.titleInput.Update(msg)
		} else {
			m.bodyInput, cmd = m.bodyInput.Update(msg)
		}
		return m, cmd

	case moveScreen:
		switch msg.String() {
		case "esc":
			m.screen = m.returnScreen
			return m, nil
		case "up", "k":
			if m.moveTo > 0 {
				m.moveTo--
			}
		case "down", "j":
			if m.moveTo+1 < len(m.project.Stages) {
				m.moveTo++
			}
		case "enter":
			if m.moveTo >= 0 && m.moveTo < len(m.project.Stages) {
				client, projectID, issue, stage := m.client, m.project.ID, m.editIssue, m.project.Stages[m.moveTo]
				m.screen, m.errorText = loadingScreen, ""
				return m, func() tea.Msg {
					return actionMessage{err: client.MoveIssue(projectID, m.project.StatusFieldID, issue, stage)}
				}
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *model) beginEdit(issue github.Issue) {
	if m.screen != editScreen {
		m.returnScreen = m.screen
	}
	m.editIssue = issue
	m.titleInput.SetValue(issue.Title)
	m.titleInput.Focus()
	m.bodyInput.SetValue(issue.Body)
	m.bodyInput.Blur()
	m.errorText = ""
	m.screen = editScreen
}

func clampSelection(column, issue int, project *github.Project) (int, int) {
	if project == nil || len(project.Stages) == 0 {
		return 0, 0
	}
	column = max(0, min(column, len(project.Stages)-1))
	var count int
	for _, item := range project.Issues {
		if item.Stage == project.Stages[column].Name {
			count++
		}
	}
	return column, max(0, min(issue, count-1))
}

func (m model) View() string {
	switch m.screen {
	case setupScreen:
		return m.setupView()
	case loadingScreen:
		return headerStyle.Render("GitHub Projects") + "\n\n" + mutedStyle.Render("Connecting to GitHub…")
	case boardScreen:
		return m.boardView()
	case detailScreen:
		return m.detailView()
	case editScreen:
		return m.editView()
	case moveScreen:
		return m.moveView()
	default:
		return fmt.Sprint("Unknown screen")
	}
}
