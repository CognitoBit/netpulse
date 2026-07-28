package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	"github.com/PLACEHOLDER/netpulse/internal/netutil"
	"github.com/PLACEHOLDER/netpulse/internal/pingtest"
	"github.com/PLACEHOLDER/netpulse/internal/tui"
)

func init() {
	cfg := pingtest.DefaultConfig()
	var jsonOut, plainOut bool

	c := &cobra.Command{
		Use:   "loss",
		Short: "ICMP packet loss test (loss %, latency, jitter)",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := resolveMode(jsonOut, plainOut)
			if mode == modeTUI {
				return tui.RunStandalone(tui.NewLossView(cfg))
			}
			return runLossPlain(cfg, mode)
		},
	}
	c.Flags().StringSliceVar(&cfg.Targets, "targets", cfg.Targets, "comma-separated hosts to ping")
	c.Flags().DurationVar(&cfg.Interval, "interval", cfg.Interval, "time between pings per target")
	c.Flags().IntVar(&cfg.Size, "size", cfg.Size, "ICMP payload bytes")
	c.Flags().DurationVar(&cfg.Duration, "duration", cfg.Duration, "test length")
	c.Flags().IntVar(&cfg.Count, "count", 0, "exact pings per target (overrides --duration)")
	c.Flags().BoolVar(&jsonOut, "json", false, "print a final JSON document")
	c.Flags().BoolVar(&plainOut, "plain", false, "line output instead of the TUI")
	rootCmd.AddCommand(c)
}

func runLossPlain(cfg pingtest.Config, mode outputMode) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ch, err := pingtest.Run(ctx, cfg)
	if err != nil {
		return remediate(err)
	}

	var last pingtest.Snapshot
	lastPrint := time.Time{}
	for snap := range ch {
		last = snap
		if mode == modePlain && (snap.Done || time.Since(lastPrint) >= time.Second) {
			lastPrint = time.Now()
			for _, t := range snap.Targets {
				fmt.Printf("%-16s sent=%-4d recv=%-4d loss=%5.1f%%  last=%-9s avg=%-9s jitter=%s\n",
					t.Target, t.Sent, t.Recv, t.LossPct,
					netutil.FormatLatency(t.LastRTT), netutil.FormatLatency(t.Avg),
					netutil.FormatLatency(t.Jitter))
			}
			if !snap.Done {
				fmt.Println()
			}
		}
	}

	if mode == modeJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(last)
	}
	fmt.Println("done")
	return nil
}
