package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/davidberget/cctask-go/internal/store"
)

var addCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Add a new task",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		root := store.FindProjectRoot("")
		ensureInit(root)

		title := args[0]
		desc, _ := cmd.Flags().GetString("description")
		tagsStr, _ := cmd.Flags().GetString("tags")
		group, _ := cmd.Flags().GetString("group")

		var tags []string
		if tagsStr != "" {
			for _, t := range strings.Split(tagsStr, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}

		task, err := store.AddTask(root, title, desc, tags, group)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Added task %s: %s\n", task.ID, task.Title)
	},
}

func init() {
	addCmd.Flags().StringP("description", "d", "", "Task description")
	addCmd.Flags().StringP("tags", "t", "", "Comma-separated tags")
	addCmd.Flags().StringP("group", "g", "", "Assign to project")
}

func ensureInit(root string) {
	if !store.IsInitialized(root) {
		fmt.Fprintln(os.Stderr, "Not a cctask project. Run `cctask init` first.")
		os.Exit(1)
	}
}
