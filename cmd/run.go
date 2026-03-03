package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/davidberget/cctask-go/internal/claude"
	"github.com/davidberget/cctask-go/internal/prompt"
	"github.com/davidberget/cctask-go/internal/store"
)

var runCmd = &cobra.Command{
	Use:   "run <id>",
	Short: "Run a task or project with Claude Code",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		root := store.FindProjectRoot("")
		ensureInit(root)

		id := args[0]
		s, err := store.LoadStore(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if task := store.FindTask(s, id); task != nil {
			workDir := store.ResolveWorkDir(root, s, task)
			p := prompt.BuildTaskPrompt(root, task)
			if err := claude.RunInteractive(workDir, p); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if group := store.FindGroup(s, id); group != nil {
			workDir := store.ResolveGroupWorkDir(root, s, group)
			p := prompt.BuildGroupPrompt(root, group, s)
			if err := claude.RunInteractive(workDir, p); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		fmt.Fprintf(os.Stderr, "No task or project found with id: %s\n", id)
		os.Exit(1)
	},
}
