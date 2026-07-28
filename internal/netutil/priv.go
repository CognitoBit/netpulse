package netutil

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// PrivilegeError wraps an ICMP socket permission failure with actionable,
// per-OS remediation text. TUI and CLI render Remediation() instead of the
// raw error.
type PrivilegeError struct {
	Underlying error
}

func (e *PrivilegeError) Error() string {
	return fmt.Sprintf("ICMP requires elevated privileges on this system: %v", e.Underlying)
}

func (e *PrivilegeError) Unwrap() error { return e.Underlying }

// Remediation returns copy-pasteable fix instructions for the current OS.
func (e *PrivilegeError) Remediation() string {
	switch runtime.GOOS {
	case "linux":
		exe, _ := os.Executable()
		if exe == "" {
			exe = "$(command -v netpulse)"
		}
		return strings.Join([]string{
			"Raw ICMP is not permitted for your user. Fix one of:",
			fmt.Sprintf("  sudo setcap cap_net_raw+ep %s   # grant raw sockets to the binary", exe),
			"  sudo netpulse ...                          # or run elevated",
			`  sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"   # helps ping tests only (persist in /etc/sysctl.conf)`,
		}, "\n")
	case "darwin":
		return "ICMP raw sockets need elevation here; try:  sudo netpulse ..."
	case "windows":
		return "ICMP was blocked. Check that no security software is blocking raw ICMP sockets, or run from an elevated terminal."
	default:
		return "run with elevated privileges"
	}
}

// IsPermissionErr reports whether err looks like a socket permission failure.
func IsPermissionErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "operation not permitted") ||
		strings.Contains(s, "permission denied")
}
