package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/davidberget/cctask-go/internal/server"
	"github.com/davidberget/cctask-go/internal/store"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the cctask webhook server",
	Long:  "Start an HTTP server for programmatic task creation via webhooks and plugins.",
	Run:   runServe,
}

var (
	servePort          int
	serveToken         string
	serveGenerateToken bool
)

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntVar(&servePort, "port", 0, "port to listen on (default: config or 8080)")
	serveCmd.Flags().StringVar(&serveToken, "token", "", "bearer token for authentication")
	serveCmd.Flags().BoolVar(&serveGenerateToken, "generate-token", false, "generate a random auth token and save to config")
}

func runServe(cmd *cobra.Command, args []string) {
	root := store.FindProjectRoot("")
	if !store.IsInitialized(root) {
		fmt.Fprintln(os.Stderr, "error: .cctask not initialized. Run 'cctask init' first.")
		os.Exit(1)
	}

	cfg := store.LoadConfig(root)

	// Handle --generate-token
	if serveGenerateToken {
		token := generateToken()
		cfg.Server.AuthToken = token
		store.SaveConfig(root, cfg)
		fmt.Printf("Generated auth token: %s\n", token)
		fmt.Println("Token saved to .cctask/config.json")
	}

	// Apply flag overrides
	if servePort != 0 {
		cfg.Server.Port = servePort
	}
	if serveToken != "" {
		cfg.Server.AuthToken = serveToken
	}

	srv := server.New(root, cfg.Server)

	// Load plugins
	if err := srv.LoadPlugins(); err != nil {
		log.Printf("warning: plugin loading: %v", err)
	}

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down...")
		srv.Stop(cmd.Context())
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func generateToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
