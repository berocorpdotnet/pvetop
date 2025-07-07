package setup

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
	"github.com/lucaswoodzy/pvetop/internal/config"
)

func RunSetupWizard() (*config.Config, error) {
	model := NewInstallerModel()
	
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("setup failed: %w", err)
	}

	installerModel := finalModel.(installerModel)
	if installerModel.state == stateComplete {
		return installerModel.config, nil
	}

	return nil, fmt.Errorf("setup was cancelled or failed")
}


func readPassword() (string, error) {
	fd := int(syscall.Stdin)

	if !term.IsTerminal(fd) {
		reader := bufio.NewReader(os.Stdin)
		password, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(password), nil
	}

	bytePassword, err := term.ReadPassword(fd)
	if err != nil {
		return "", err
	}

	fmt.Println() 
	return string(bytePassword), nil
}


func ValidateHost(host string) (string, error) {
	if host == "" {
		return "", fmt.Errorf("host cannot be empty")
	}

	
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")

	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	host = strings.TrimSuffix(host, "/")

	if strings.Contains(host, "/") || strings.Contains(host, " ") {
		return "", fmt.Errorf("invalid host format")
	}

	return host, nil
}

func ValidateToken(token string) error {
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	if !strings.Contains(token, "=") {
		return fmt.Errorf("token must contain '=' (missing secret part)")
	}

	if !strings.Contains(token, "!") {
		return fmt.Errorf("token must contain '!' (missing token ID)")
	}

	parts := strings.Split(token, "=")
	if len(parts) != 2 {
		return fmt.Errorf("token format should be USER@REALM!TOKENID=SECRET")
	}

	userTokenPart := parts[0]
	secret := parts[1]

	if secret == "" {
		return fmt.Errorf("token secret cannot be empty")
	}

	userParts := strings.Split(userTokenPart, "!")
	if len(userParts) != 2 {
		return fmt.Errorf("token format should be USER@REALM!TOKENID=SECRET")
	}

	user := userParts[0]
	tokenID := userParts[1]

	if user == "" {
		return fmt.Errorf("user part cannot be empty")
	}

	if tokenID == "" {
		return fmt.Errorf("token ID cannot be empty")
	}

	if !strings.Contains(user, "@") {
		return fmt.Errorf("user must include realm (e.g., user@pam)")
	}

	return nil
}

func ShowReconfigurePrompt() (bool, error) {
	fmt.Println()
	fmt.Println(" Configuration already exists.")
	
	configPath, _ := config.GetConfigLocation()
	fmt.Printf("Current config location: %s\n", configPath)
	fmt.Println()
	
	reader := bufio.NewReader(os.Stdin)
	
	for {
		fmt.Print("Do you want to reconfigure? (y/n): ")
		response, err := reader.ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("failed to read response: %w", err)
		}
		
		response = strings.ToLower(strings.TrimSpace(response))
		
		switch response {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Println("Please enter 'y' for yes or 'n' for no.")
		}
	}
}
