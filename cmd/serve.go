package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/CognitoBit/netpulse/internal/lanserve"
	"github.com/CognitoBit/netpulse/internal/netutil"
	"github.com/CognitoBit/netpulse/internal/tui"
)

func init() {
	var port int

	c := &cobra.Command{
		Use:   "serve",
		Short: "Host a LAN speed test server with a browser page",
		RunE: func(cmd *cobra.Command, args []string) error {
			explicit := cmd.Flags().Changed("port")
			if resolveMode(false, false) == modeTUI {
				return tui.RunStandalone(tui.NewServeView(port, explicit))
			}
			return runServePlain(port, explicit)
		},
	}
	c.Flags().IntVar(&port, "port", lanserve.DefaultPort, "port to listen on")
	rootCmd.AddCommand(c)
}

func runServePlain(port int, explicit bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	srv, err := lanserve.Start(ctx, port, explicit)
	if err != nil {
		return err
	}
	fmt.Println("LAN speed test server running. Open on any device on this network:")
	for _, u := range srv.URLs {
		fmt.Println("  " + u)
	}
	fmt.Println("Ctrl+C to stop.")

	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-srv.Events:
			rate := ""
			if e.RateBps > 0 {
				rate = "  " + netutil.FormatBitRate(e.RateBps)
			}
			fmt.Printf("%s  %-16s %-10s %10s%s\n",
				e.Time.Format("15:04:05"), e.ClientIP, e.Path, netutil.FormatBytes(e.Bytes), rate)
		}
	}
}
