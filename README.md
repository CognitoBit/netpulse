# netpulse

A terminal network performance toolkit — one static binary, no dependencies.

![netpulse demo](demo.gif)

- **Speed test** — internet download/upload/latency via speedtest.net (Ookla) or Cloudflare
- **LAN speed server** — host a browser speed test page; any phone/laptop on your network can test against your machine
- **Packet loss** — ICMP burst testing with loss %, latency distribution and jitter (packetlosstest.com style)
- **MTR** — traceroute with live per-hop loss and latency stats

Everything runs in the terminal: an interactive TUI when run bare, plain/JSON output for scripts.

## Install

**Windows** (PowerShell):

```powershell
irm https://raw.githubusercontent.com/CognitoBit/netpulse/main/install.ps1 | iex
```

**macOS / Linux**:

```bash
curl -fsSL https://raw.githubusercontent.com/CognitoBit/netpulse/main/install.sh | sh
```

Or grab a binary from [releases](https://github.com/CognitoBit/netpulse/releases).

## Use

```
netpulse                  # interactive menu
netpulse speed            # speed test (--backend ookla|cloudflare)
netpulse serve            # LAN speed test server on :7333
netpulse loss             # packet loss test (defaults: 8.8.8.8, 1.1.1.1, 10 pings/sec, 30s)
netpulse mtr google.com   # live traceroute stats
```

Every test command accepts `--plain` (line output) or `--json` (machine-readable final document), and detects non-TTY output automatically.

Examples:

```
netpulse loss --targets 8.8.8.8,1.1.1.1 --interval 50ms --duration 1m
netpulse mtr cloudflare.com --cycles 20 --json
netpulse speed --backend cloudflare --plain
netpulse serve --port 8080
```

## Platform notes

- **Windows**: ICMP works without elevation. The LAN server triggers a one-time Windows Firewall prompt — allow it on Private networks.
- **macOS**: everything works unprivileged.
- **Linux**: the packet loss test works unprivileged if `net.ipv4.ping_group_range` allows it (most distros). MTR needs raw sockets — run with `sudo`, or grant the binary `cap_net_raw` once:
  ```
  sudo setcap cap_net_raw+ep "$(command -v netpulse)"
  ```
  (Real `mtr` has the same requirement.)
- Mid-path hops showing high loss while the final hop shows 0% is normal — routers deprioritize ICMP replies.

## Build from source

```
go build -o netpulse .
```

Releases are built by GoReleaser via GitHub Actions on version tags (`v*`).

## License

[MIT](LICENSE)

Future: Homebrew tap (`brews:`) and Scoop bucket (`scoops:`) can be added to `.goreleaser.yaml` once tap/bucket repos exist.
