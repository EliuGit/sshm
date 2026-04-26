package main

import (
	"fmt"
	"os"
	"sshm/internal/app"
	"sshm/internal/cli"
	"sshm/internal/config"
	"sshm/internal/i18n"
	"sshm/internal/security"
	"sshm/internal/store/sqlite"
	sshtransport "sshm/internal/transport/ssh"
	"sshm/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		os.Exit(cli.New(nil, nil, os.Stdout, os.Stderr).Run(os.Args[1:]))
	}

	runtimeConfig, err := config.Load()
	if err != nil {
		fatal(nil, err)
	}
	translator, err := i18n.New(runtimeConfig.Language)
	if err != nil {
		fatal(nil, err)
	}
	if err := config.EnsurePaths(runtimeConfig); err != nil {
		fatal(translator, err)
	}

	startupDir := detectStartupDir()

	crypto, err := security.LoadOrCreateKey(runtimeConfig.KeyPath)
	if err != nil {
		fatal(translator, err)
	}

	repo, err := sqlite.Open(runtimeConfig.DatabasePath)
	if err != nil {
		fatal(translator, err)
	}
	defer repo.Close()

	remote := sshtransport.NewClient(runtimeConfig.KnownHostsPath, runtimeConfig.DefaultPrivateKeyPath)
	services := app.NewServices(repo, crypto, remote, runtimeConfig.DefaultPrivateKeyPath)

	if len(os.Args) > 1 {
		os.Exit(cli.New(services, translator, os.Stdout, os.Stderr).Run(os.Args[1:]))
	}

	for {
		model := ui.NewModel(services, translator, startupDir, runtimeConfig.DefaultPrivateKeyPath)
		program := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := program.Run(); err != nil {
			fatal(translator, err)
		}

		result := model.Result()
		if result.ShellSession == nil {
			return
		}
		err = result.ShellSession.OpenShell()
		_ = result.ShellSession.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, translator.T("app.ssh_session_failed"), translator.Error(err))
		}
	}
}

func detectStartupDir() string {
	wd, err := os.Getwd()
	if err == nil {
		return wd
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return home
	}
	return "."
}

func fatal(translator *i18n.Translator, err error) {
	if translator == nil {
		translator, _ = i18n.New("en")
	}
	fmt.Fprintf(os.Stderr, translator.T("app.error_prefix"), translator.Error(err))
	os.Exit(1)
}
