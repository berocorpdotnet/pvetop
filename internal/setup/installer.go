package setup

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/berocorpdotnet/pvetop/internal/api"
	"github.com/berocorpdotnet/pvetop/internal/config"
	"github.com/berocorpdotnet/pvetop/internal/theme"
)


type installState int

const (
	stateForm installState = iota
	stateConnecting
	stateCreatingToken
	stateSaving
	stateComplete
	stateError
)

const (
	focusHost = iota
	focusPort
	focusUser
	focusPass
	focusRealm
	focusSubmit
)

type installerModel struct {
	state         installState
	hostInput     textinput.Model
	portInput     textinput.Model
	userInput     textinput.Model
	passInput     textinput.Model
	realmInput    textinput.Model
	focusedInput  int
	width         int
	height        int
	statusMsg     string
	config        *config.Config
	client        *api.Client
	progress      float64
	errorMsg      string
}

type progressMsg struct {
	state   installState
	message string
	error   error
	token   string
}

func NewInstallerModel() installerModel {
	hostInput := textinput.New()
	hostInput.Placeholder = ""
	hostInput.Focus()
	hostInput.CharLimit = 100
	hostInput.Width = 40

	portInput := textinput.New()
	portInput.SetValue("8006")
	portInput.CharLimit = 5
	portInput.Width = 40

	userInput := textinput.New()
	userInput.Placeholder = "root"
	userInput.CharLimit = 50
	userInput.Width = 40

	passInput := textinput.New()
	passInput.Placeholder = ""
	passInput.EchoMode = textinput.EchoPassword
	passInput.CharLimit = 100
	passInput.Width = 40

	realmInput := textinput.New()
	realmInput.SetValue("pam")
	realmInput.CharLimit = 20
	realmInput.Width = 40

	return installerModel{
		state:        stateForm,
		hostInput:    hostInput,
		portInput:    portInput,
		userInput:    userInput,
		passInput:    passInput,
		realmInput:   realmInput,
		focusedInput: 0,
		statusMsg:    "",
	}
}

func (m installerModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m installerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.state != stateForm {
			switch msg.String() {
			case "ctrl+c", "q":
				if m.state == stateComplete || m.state == stateError {
					return m, tea.Quit
				}
			case "enter":
				if m.state == stateComplete || m.state == stateError {
					return m, tea.Quit
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "tab", "shift+tab", "up", "down":
			if msg.String() == "up" || msg.String() == "shift+tab" {
				m.focusedInput--
			} else {
				m.focusedInput++
			}

			if m.focusedInput > focusSubmit {
				m.focusedInput = focusHost
			} else if m.focusedInput < focusHost {
				m.focusedInput = focusSubmit
			}

			m.updateFocus()

		case "enter":
			if m.focusedInput == focusSubmit {
				if m.isFormValid() {
					m.statusMsg = "Starting installation..."
					return m.startInstallation()
				} else {
					m.statusMsg = "Please fill in all fields"
				}
				return m, nil
			}
			if m.focusedInput == focusRealm {
				if m.realmInput.Value() == "pam" {
					m.realmInput.SetValue("pve")
				} else {
					m.realmInput.SetValue("pam")
				}
				return m, nil
			}
			m.focusedInput++
			if m.focusedInput > focusSubmit {
				m.focusedInput = focusHost
			}
			m.updateFocus()

		case " ":
			if m.focusedInput == focusRealm {
				if m.realmInput.Value() == "pam" {
					m.realmInput.SetValue("pve")
				} else {
					m.realmInput.SetValue("pam")
				}
				return m, nil
			}
		}

	case progressMsg:
		m.state = msg.state
		m.statusMsg = msg.message
		
		if msg.error != nil {
			m.state = stateError
			m.errorMsg = msg.error.Error()
			m.statusMsg = msg.message + ": " + msg.error.Error()
		} else {
			if msg.token != "" {
				m.config.Token = msg.token
			}
		}

		switch msg.state {
		case stateConnecting:
			return m, m.testConnection()
		case stateCreatingToken:
			return m, m.createToken()
		case stateSaving:
			return m, m.saveConfig()
		case stateComplete:
			m.statusMsg = "Setup completed! Starting pvetop..."
		}
	}

	switch m.focusedInput {
	case focusHost:
		m.hostInput, cmd = m.hostInput.Update(msg)
	case focusPort:
		m.portInput, cmd = m.portInput.Update(msg)
	case focusUser:
		m.userInput, cmd = m.userInput.Update(msg)
	case focusPass:
		m.passInput, cmd = m.passInput.Update(msg)
	case focusRealm:
		cmd = nil
	case focusSubmit:
		cmd = nil
	}
	return m, cmd
}

func (m *installerModel) updateFocus() {
	m.hostInput.Blur()
	m.portInput.Blur()
	m.userInput.Blur()
	m.passInput.Blur()
	m.realmInput.Blur()

	switch m.focusedInput {
	case focusHost:
		m.hostInput.Focus()
	case focusPort:
		m.portInput.Focus()
	case focusUser:
		m.userInput.Focus()
	case focusPass:
		m.passInput.Focus()
	case focusRealm:
		m.realmInput.Focus()
	case focusSubmit:
	}
}

func (m installerModel) isFormValid() bool {
	host := strings.TrimSpace(m.hostInput.Value())
	port := strings.TrimSpace(m.portInput.Value())
	user := strings.TrimSpace(m.userInput.Value())
	pass := strings.TrimSpace(m.passInput.Value())
	realm := strings.TrimSpace(m.realmInput.Value())
	
	if user == "" {
		user = "root"
	}
	if port == "" {
		port = "8006"
	}
	if realm == "" {
		realm = "pam"
	}
	
	return host != "" && port != "" && user != "" && pass != "" && realm != ""
}

func (m installerModel) startInstallation() (installerModel, tea.Cmd) {
	host, err := ValidateHost(m.hostInput.Value())
	if err != nil {
		m.statusMsg = "Invalid host: " + err.Error()
		return m, nil
	}

	user := strings.TrimSpace(m.userInput.Value())
	if user == "" {
		user = "root"
	}
	
	realm := strings.TrimSpace(m.realmInput.Value())
	if realm == "" {
		realm = "pam"
	}
	
	username := fmt.Sprintf("%s@%s", user, realm)

	port := strings.TrimSpace(m.portInput.Value())
	if port == "" {
		port = "8006"
	}

	m.config = &config.Config{
		Host:     host,
		Port:	  port,
		Username: username,
	}

	m.statusMsg = "Starting Proxmox VE setup..."
	m.state = stateConnecting

	return m, func() tea.Msg {
		return progressMsg{
			state:   stateConnecting,
			message: "Testing connection to " + host,
		}
	}
}

func (m installerModel) testConnection() tea.Cmd {
	return func() tea.Msg {
		client := api.NewClient(m.config.Host, m.config.Port)
		if err := client.Login(m.config.Username, m.passInput.Value()); err != nil {
			return progressMsg{
				state:   stateError,
				message: "Connection failed",
				error:   err,
			}
		}

		nodes, err := client.GetNodes()
		if err != nil {
			return progressMsg{
				state:   stateError,
				message: "Failed to get nodes",
				error:   err,
			}
		}

		return progressMsg{
			state:   stateCreatingToken,
			message: fmt.Sprintf("Connected! Found %d node(s). Creating API token...", len(nodes)),
		}
	}
}

func (m installerModel) createToken() tea.Cmd {
	return func() tea.Msg {
		client := api.NewClient(m.config.Host, m.config.Port)
		if err := client.Login(m.config.Username, m.passInput.Value()); err != nil {
			return progressMsg{
				state:   stateError,
				message: "Failed to authenticate for token creation",
				error:   err,
			}
		}

		tokenID := fmt.Sprintf("pvetop-%d", time.Now().Unix())
		
		token, err := client.CreateAPIToken(m.config.Username, tokenID)
		if err != nil {
			return progressMsg{
				state:   stateError,
				message: "Failed to create API token",
				error:   err,
			}
		}

		return progressMsg{
			state:   stateSaving,
			message: "API token created successfully. Saving configuration...",
			token:   token,
		}
	}
}

func (m installerModel) saveConfig() tea.Cmd {
	return func() tea.Msg {
		if err := config.Save(m.config); err != nil {
			return progressMsg{
				state:   stateError,
				message: "Failed to save configuration",
				error:   err,
			}
		}

		return progressMsg{
			state:   stateComplete,
			message: "Configuration saved securely",
		}
	}
}


func (m installerModel) View() string {
	if m.width < 80 || m.height < 24 {
		return lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(theme.Catppuccin.Text).
			Render("Terminal too small\nMinimum: 80x24\nCurrent: " + fmt.Sprintf("%dx%d", m.width, m.height))
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Catppuccin.Blue).
		Align(lipgloss.Center).
		Width(60).
		MarginBottom(1).
		Render("pvetop Setup")

	subtitle := lipgloss.NewStyle().
		Foreground(theme.Catppuccin.Subtext1).
		Align(lipgloss.Center).
		Width(60).
		MarginBottom(2).
		Render("Configure your Proxmox VE connection")

	formStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Catppuccin.Surface1).
		Padding(1, 2).
		Width(60).
		MarginBottom(2)

	labelStyle := lipgloss.NewStyle().
		Foreground(theme.Catppuccin.Subtext1).
		Bold(true).
		Width(10).
		Align(lipgloss.Right)

	focusedLabelStyle := lipgloss.NewStyle().
		Foreground(theme.Catppuccin.Blue).
		Bold(true).
		Width(10).
		Align(lipgloss.Right)

	inputStyle := lipgloss.NewStyle().
		Width(40).
		Background(theme.Catppuccin.Surface0).
		Padding(0, 1)

	buttonStyle := lipgloss.NewStyle().
		Foreground(theme.Catppuccin.Text).
		Background(theme.Catppuccin.Surface1).
		Padding(0, 2).
		MarginLeft(12).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Catppuccin.Surface1)

	focusedButtonStyle := lipgloss.NewStyle().
		Foreground(theme.Catppuccin.Base).
		Background(theme.Catppuccin.Blue).
		Padding(0, 2).
		MarginLeft(12).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Catppuccin.Blue).
		Bold(true)

	renderField := func(label string, value string, placeholder string, focused bool, isPassword bool) string {
		labelS := labelStyle
		if focused {
			labelS = focusedLabelStyle
		}
		
		displayValue := value
		if isPassword && value != "" {
			displayValue = strings.Repeat("*", len(value))
		}
		
		if value == "" && placeholder != "" {
			if focused {
				if len(placeholder) > 1 {
					displayValue = "█" + placeholder[1:]
				} else {
					displayValue = "█"
				}
			} else {
				displayValue = lipgloss.NewStyle().Foreground(theme.Catppuccin.Overlay0).Render(placeholder)
			}
		} else if focused {
			displayValue += "█"
		}
		
		labelPart := labelS.Render(label)
		inputPart := inputStyle.Render(displayValue)
		
		return labelPart + " " + inputPart
	}

	renderRealmField := func(value string, focused bool) string {
		labelS := labelStyle
		if focused {
			labelS = focusedLabelStyle
		}
		
		displayValue := value
		if value == "pam" {
			displayValue = "PAM"
		} else if value == "pve" {
			displayValue = "Proxmox VE"
		}
		
		if focused {
			displayValue += " ▼█"
		} else {
			displayValue += " ▼"
		}
		
		labelPart := labelS.Render("Realm:")
		inputPart := inputStyle.Render(displayValue)
		
		return labelPart + " " + inputPart
	}

	hostField := renderField("Host:", m.hostInput.Value(), "", m.focusedInput == focusHost, false)
	portField := renderField("Port:", m.portInput.Value(), "", m.focusedInput == focusPort, false)
	userField := renderField("Username:", m.userInput.Value(), "root", m.focusedInput == focusUser, false)
	passField := renderField("Password:", m.passInput.Value(), "", m.focusedInput == focusPass, true)
	realmField := renderRealmField(m.realmInput.Value(), m.focusedInput == focusRealm)

	var submitButton string
	if m.focusedInput == focusSubmit {
		submitButton = focusedButtonStyle.Render("Continue")
	} else {
		submitButton = buttonStyle.Render("Continue")
	}

	form := formStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			hostField,
			"",
			portField,
			"",
			userField,
			"",
			passField,
			"",
			realmField,
			"",
			submitButton,
		),
	)

	var status string
	var statusColor lipgloss.Color

	if m.statusMsg != "" {
		status = m.statusMsg
		if m.state == stateError {
			statusColor = theme.Catppuccin.Red
		} else {
			statusColor = theme.Catppuccin.Blue
		}
	} else {
		switch m.state {
		case stateForm:
			if m.isFormValid() {
				if m.focusedInput == focusSubmit {
					status = "Ready! Press Enter to start setup"
					statusColor = theme.Catppuccin.Green
				} else {
					status = "Navigate to Continue button and press Enter"
					statusColor = theme.Catppuccin.Blue
				}
			} else {
				status = "Fill in all fields to continue"
				statusColor = theme.Catppuccin.Yellow
			}
		case stateConnecting:
			status = "Connecting to Proxmox..."
			statusColor = theme.Catppuccin.Yellow
		case stateCreatingToken:
			status = "Creating API token..."
			statusColor = theme.Catppuccin.Blue
		case stateSaving:
			status = "Saving configuration..."
			statusColor = theme.Catppuccin.Mauve
		case stateComplete:
			status = "Setup complete! Press Enter to continue to pvetop"
			statusColor = theme.Catppuccin.Green
		case stateError:
			status = "Setup failed. Press Enter to try again"
			statusColor = theme.Catppuccin.Red
		}
	}

	statusLine := lipgloss.NewStyle().
		Foreground(statusColor).
		Align(lipgloss.Center).
		Width(60).
		MarginBottom(2).
		Render(status)

	var help string
	if m.state == stateForm {
		if m.focusedInput == focusRealm {
			help = "Space/Enter: Toggle realm • Tab: Navigate • ↑↓: Navigate • Ctrl+C: Quit"
		} else {
			help = "Tab/Enter: Navigate • ↑↓: Navigate • Ctrl+C: Quit"
		}
	} else if m.state == stateComplete || m.state == stateError {
		help = "Enter: Continue • Ctrl+C: Quit"
	} else {
		help = "Please wait..."
	}

	helpText := lipgloss.NewStyle().
		Foreground(theme.Catppuccin.Overlay0).
		Align(lipgloss.Center).
		Width(60).
		Render(help)

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		subtitle,
		form,
		statusLine,
		helpText,
	)

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)
}
