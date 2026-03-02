package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/davidberget/cctask-go/internal/store"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize .cctask/ in the current directory",
	Run: func(cmd *cobra.Command, args []string) {
		root, _ := os.Getwd()
		if store.IsInitialized(root) {
			fmt.Println(".cctask/ already exists")
			return
		}
		if err := store.Init(root); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Initialized .cctask/ in", root)
	},
}
