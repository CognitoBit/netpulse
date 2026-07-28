package main

import "github.com/PLACEHOLDER/netpulse/cmd"

// version is injected at release time via -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	cmd.Execute(version)
}
