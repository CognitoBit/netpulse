package cmd

import (
	"fmt"
	"os"

	"github.com/PLACEHOLDER/netpulse/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var noColor bool

var rootCmd = &cobra.Command{
	Use:   "netpulse",
	Short: "Network performance toolkit: speed test, LAN speed server, packet loss, MTR",
	Long: `netpulse is a terminal network performance toolkit.

Run with no arguments for the interactive menu, or use a subcommand directly:

  netpulse speed    internet speed test (Ookla or Cloudflare)
  netpulse serve    host a LAN speed test server with a browser page
  netpulse loss     ICMP packet loss / jitter test
  netpulse mtr      traceroute with per-hop loss and latency stats`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			return fmt.Errorf("no TTY detected; use a subcommand (speed, serve, loss, mtr) with --plain or --json")
		}
		return tui.RunMenu(version)
	},
}

var version = "dev"

// Execute is the CLI entry point, called from main with the release version.
func Execute(v string) {
	version = v
	rootCmd.Version = v
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// outputMode resolves how a subcommand should render: interactive TUI, plain
// line output, or a final JSON document.
type outputMode int

const (
	modeTUI outputMode = iota
	modePlain
	modeJSON
)

func resolveMode(jsonFlag, plainFlag bool) outputMode {
	switch {
	case jsonFlag:
		return modeJSON
	case plainFlag:
		return modePlain
	case !term.IsTerminal(int(os.Stdout.Fd())):
		return modePlain
	default:
		return modeTUI
	}
}
