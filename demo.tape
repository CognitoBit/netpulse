# VHS demo for netpulse — render with: vhs demo.tape
# https://github.com/charmbracelet/vhs

Output demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1120
Set Height 560
Set Theme "Dracula"
Set WindowBar Colorful
Set Framerate 20
Set TypingSpeed 60ms
Set Padding 16

# Renders locally (netpulse on PATH) or via the VHS Docker image with the
# repo mounted at /vhs after `goreleaser release --snapshot`:
#   docker run --rm -v .:/vhs ghcr.io/charmbracelet/vhs demo.tape
Hide
Type "export PATH=$HOME/.local/bin:/vhs/dist/netpulse_linux_amd64_v1:$PATH && clear"
Enter
Show

# Main menu
Type "netpulse"
Sleep 500ms
Enter
Sleep 2.5s

# Packet loss test (menu item 3)
Type "3"
Sleep 7s
Escape
Sleep 1.5s

# MTR (menu item 4)
Type "4"
Sleep 1s
Type "cloudflare.com"
Sleep 500ms
Enter
Sleep 9s
Escape
Sleep 1.5s

# Speed test (menu item 1) — show the backend picker and the live phases
Type "1"
Sleep 1.5s
Enter
Sleep 11s
Escape
Sleep 1s

Type "q"
Sleep 500ms
