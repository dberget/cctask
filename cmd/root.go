package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/davidberget/cctask-go/internal/store"
	"github.com/davidberget/cctask-go/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "cctask",
	Short: "Interactive TUI task manager for Claude Code",
	Run: func(cmd *cobra.Command, args []string) {
		root := store.FindProjectRoot("")
		if !store.IsInitialized(root) {
			store.Init(root)
		}
		tui.Run(root)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(runCmd)
}
