package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/CognitoBit/netpulse/internal/netutil"
	"github.com/CognitoBit/netpulse/internal/trace"
	"github.com/CognitoBit/netpulse/internal/tui"
)

func init() {
	cfg := trace.DefaultConfig("")
	var jsonOut, plainOut bool

	c := &cobra.Command{
		Use:   "mtr <host>",
		Short: "Traceroute with per-hop loss and latency (like mtr)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Host = args[0]
			mode := resolveMode(jsonOut, plainOut)
			if mode == modeTUI {
				return tui.RunStandalone(tui.NewMTRView(cfg))
			}
			if cfg.Cycles == 0 {
				cfg.Cycles = 10 // don't run forever without a UI
			}
			return runMTRPlain(cfg, mode)
		},
	}
	c.Flags().IntVar(&cfg.MaxHops, "max-hops", cfg.MaxHops, "TTL limit")
	c.Flags().DurationVar(&cfg.Interval, "interval", cfg.Interval, "time between cycles")
	c.Flags().DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "per-probe reply deadline")
	c.Flags().IntVar(&cfg.Cycles, "cycles", 0, "stop after N cycles (0 = endless in TUI, 10 otherwise)")
	c.Flags().BoolVar(&cfg.NoDNS, "no-dns", false, "skip reverse DNS lookups")
	c.Flags().BoolVar(&jsonOut, "json", false, "print a final JSON document")
	c.Flags().BoolVar(&plainOut, "plain", false, "line output instead of the TUI")
	rootCmd.AddCommand(c)
}

func runMTRPlain(cfg trace.Config, mode outputMode) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ch, err := trace.Run(ctx, cfg)
	if err != nil {
		return remediate(err)
	}

	var last trace.Snapshot
	for snap := range ch {
		last = snap
	}

	if mode == modeJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(last)
	}

	fmt.Printf("mtr to %s (%s), %d cycles\n", last.Target, last.TargetIP, last.Cycles)
	fmt.Printf("%3s  %-40s %7s %5s %9s %9s %9s %9s\n", "HOP", "HOST", "LOSS", "SNT", "LAST", "AVG", "BEST", "WORST")
	for _, h := range last.Hops {
		name := h.Addr
		if h.Host != "" {
			name = h.Host
		}
		if name == "" {
			name = "???"
		}
		fmt.Printf("%3d  %-40.40s %6.1f%% %5d %9s %9s %9s %9s\n",
			h.TTL, name, h.LossPct, h.Sent,
			netutil.FormatLatency(h.Last), netutil.FormatLatency(h.Avg),
			netutil.FormatLatency(h.Best), netutil.FormatLatency(h.Worst))
	}
	return nil
}

// remediate upgrades privilege errors to include their fix instructions.
func remediate(err error) error {
	var priv *netutil.PrivilegeError
	if errors.As(err, &priv) {
		return fmt.Errorf("%w\n\n%s", err, priv.Remediation())
	}
	return err
}
