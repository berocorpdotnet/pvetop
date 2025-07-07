package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucaswoodzy/pvetop/internal/api"
	"github.com/lucaswoodzy/pvetop/internal/config"
	"github.com/lucaswoodzy/pvetop/internal/setup"
	"github.com/lucaswoodzy/pvetop/internal/ui"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	client := api.NewClientWithToken(cfg.Host, cfg.Port, cfg.Token)
	
	_, err = client.GetNodes()
	if err != nil {
		fmt.Printf("Failed to connect to Proxmox: %v\n", err)
		fmt.Printf("Config details - Host: %s, Port: %s, Username: %s, Token length: %d\n", 
			cfg.Host, cfg.Port, cfg.Username, len(cfg.Token))
		fmt.Println("Your configuration may be invalid. Run 'pvetop --setup' to reconfigure.")
		os.Exit(1)
	}
	
	p := tea.NewProgram(ui.NewModel(client), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig() (*config.Config, error) {
	for _, arg := range os.Args[1:] {
		if arg == "--setup" || arg == "--configure" {
			return runSetup(true)
		}
	}

	if config.Exists() {
		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("Failed to load configuration: %v\n", err)
			fmt.Println("Configuration may be corrupted. Running setup wizard...")
			return runSetup(false)
		}
		return cfg, nil
	}

	fmt.Println("No configuration found. Running initial setup...")
	return runSetup(false)
}

func runSetup(force bool) (*config.Config, error) {
	if !force && config.Exists() {
		reconfigure, err := setup.ShowReconfigurePrompt()
		if err != nil {
			return nil, err
		}
		if !reconfigure {
			fmt.Println("Keeping existing configuration.")
			return config.Load()
		}
	}

	cfg, err := setup.RunSetupWizard()
	if err != nil {
		return nil, fmt.Errorf("setup failed: %w", err)
	}

	return cfg, nil
}
