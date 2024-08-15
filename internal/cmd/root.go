package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/shashimalcse/asgardeo-cli/internal/config"
	"github.com/shashimalcse/asgardeo-cli/internal/core"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const rootShort = `
    _                            _                       ____ _     ___ 
   / \   ___  __ _  __ _ _ __ __| | ___  ___            / ___| |   |_ _|
  / _ \ / __|/ _` + "`" + ` |/ _` + "`" + ` | '__/ _` + "`" + ` |/ _ \/ _ \   _____  | |   | |    | | 
 / ___ \\__ \ (_| | (_| | | | (_| |  __/ (_) | |_____| | |___| |___ | | 
/_/   \_\___/\__, |\__,_|_|  \__,_|\___|\___/           \____|_____|___|
             |___/                                                      


Build, manage and test your Asgardeo integrations from the command line.
`

func Execute() {
	logger, err := configLogger()
	cfg := config.NewConfig(logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()
	cli := core.NewCLI(cfg, logger)
	rootCmd := buildRootCmd(cli)
	addSubCommands(rootCmd, cli)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go handleSignals(cancel)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		logger.Error("Command execution failed", zap.Error(err))
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func buildRootCmd(cli *core.CLI) *cobra.Command {
	rootCommand := &cobra.Command{
		Use:           "asgardeo",
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         rootShort,
		Long:          rootShort,
		Version:       "v0.0.1",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if !commandRequiresAuthentication(cmd.CommandPath()) {
				return nil
			}
			if err := cli.SetupWithAuthentication(cmd.Context()); err != nil {
				cli.Logger.Error("Authentication setup failed", zap.Error(err))
				return fmt.Errorf("authentication failed: %w", err)
			}
			return nil
		},
	}
	return rootCommand
}

func addSubCommands(rootCmd *cobra.Command, cli *core.CLI) {
	rootCmd.AddCommand(loginCmd(cli))
	rootCmd.AddCommand(logoutCmd(cli))
	rootCmd.AddCommand(applicationsCmd(cli))
}

func commandRequiresAuthentication(invokedCommandName string) bool {
	commandsWithNoAuthRequired := map[string]bool{
		"asgardeo login":  true,
		"asgardeo logout": true,
	}
	return !commandsWithNoAuthRequired[invokedCommandName]
}

func configLogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	logDir := filepath.Join(cwd, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logFilePath := filepath.Join(logDir, "asgardeo-cli.log")

	config.OutputPaths = []string{logFilePath}
	config.ErrorOutputPaths = []string{logFilePath}

	return config.Build()
}

func handleSignals(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh
	cancel()
}
