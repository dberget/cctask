package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/davidberget/cctask-go/internal/store"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tasks",
	Run: func(cmd *cobra.Command, args []string) {
		root := store.FindProjectRoot("")
		ensureInit(root)

		s, err := store.LoadStore(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(s.Tasks) == 0 {
			fmt.Println("No tasks. Add one with: cctask add \"My task\"")
			return
		}
		for _, t := range s.Tasks {
			status := "●"
			switch t.Status {
			case "in-progress":
				status = "◉"
			case "done":
				status = "✓"
			}
			group := ""
			if t.Group != "" {
				// Show full group path
				path := store.GetGroupPath(s, t.Group)
				if len(path) > 0 {
					var names []string
					for _, g := range path {
						names = append(names, g.Name)
					}
					group = fmt.Sprintf(" [%s]", strings.Join(names, " > "))
				} else {
					group = fmt.Sprintf(" [%s]", t.Group)
				}
			}
			fmt.Printf("  %-5s %s %s%s\n", t.ID, status, t.Title, group)
		}
	},
}
