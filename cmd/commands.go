package cmd

import (
	core "goodin/internal"

	"github.com/spf13/cobra"
)

func NewCommands(app core.App) []*cobra.Command {
	rootCmd := []*cobra.Command{
		// NewAllCommand(app),
		// NewMessageBrokerCommand(app),
		NewMasterCommand(app),
		NewWorkerCommand(app),
		// NewHttpCommand(app),
		// NewMigration(app),
		// newVersionCmd(app),
	}
	return rootCmd
}
