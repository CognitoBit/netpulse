package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/PLACEHOLDER/netpulse/internal/netutil"
	"github.com/PLACEHOLDER/netpulse/internal/speed"
	"github.com/PLACEHOLDER/netpulse/internal/tui"
)

func init() {
	var backend string
	var serverID int
	var jsonOut, plainOut bool

	c := &cobra.Command{
		Use:   "speed",
		Short: "Internet speed test (download, upload, latency)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, ok := speed.New(backend, serverID); !ok {
				return fmt.Errorf("unknown backend %q (want ookla or cloudflare)", backend)
			}
			mode := resolveMode(jsonOut, plainOut)
			if mode == modeTUI {
				return tui.RunStandalone(tui.NewSpeedView(backend, serverID))
			}
			return runSpeedPlain(backend, serverID, mode)
		},
	}
	c.Flags().StringVar(&backend, "backend", "ookla", "test backend: ookla or cloudflare")
	c.Flags().IntVar(&serverID, "server-id", 0, "specific speedtest.net server ID (ookla only)")
	c.Flags().BoolVar(&jsonOut, "json", false, "print a final JSON document")
	c.Flags().BoolVar(&plainOut, "plain", false, "line output instead of the TUI")
	rootCmd.AddCommand(c)
}

func runSpeedPlain(backend string, serverID int, mode outputMode) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	b, _ := speed.New(backend, serverID)
	events := make(chan speed.Event, 32)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range events {
			if mode == modePlain && e.Msg != "" {
				fmt.Println(e.Msg)
			}
		}
	}()

	res, err := b.Run(ctx, events)
	close(events)
	<-done
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}

	if mode == modeJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	fmt.Printf("download: %s\n", netutil.FormatBitRate(res.DownloadBps))
	fmt.Printf("upload:   %s\n", netutil.FormatBitRate(res.UploadBps))
	fmt.Printf("latency:  %s (jitter %s)\n", netutil.FormatLatency(res.Latency), netutil.FormatLatency(res.Jitter))
	fmt.Printf("server:   %s\n", res.Server)
	return nil
}
