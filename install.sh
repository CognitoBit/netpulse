#!/bin/sh
# netpulse installer for macOS and Linux.
#   curl -fsSL https://raw.githubusercontent.com/CognitoBit/netpulse/main/install.sh | sh
set -eu

REPO="CognitoBit/netpulse"

main() {
    os=$(uname -s)
    case "$os" in
        Linux)  os=linux ;;
        Darwin) os=darwin ;;
        *) err "unsupported OS: $os (netpulse supports Linux and macOS; on Windows use install.ps1)" ;;
    esac

    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)  arch=amd64 ;;
        aarch64|arm64) arch=arm64 ;;
        *) err "unsupported architecture: $arch (need x86_64 or arm64)" ;;
    esac

    asset="netpulse_${os}_${arch}.tar.gz"
    url="https://github.com/${REPO}/releases/latest/download/${asset}"

    if [ -n "${NETPULSE_INSTALL_DIR:-}" ]; then
        install_dir="$NETPULSE_INSTALL_DIR"
    elif [ -n "${XDG_BIN_HOME:-}" ]; then
        install_dir="$XDG_BIN_HOME"
    else
        install_dir="$HOME/.local/bin"
    fi

    say "downloading $asset ..."
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT
    curl -fsSL "$url" -o "$tmp/$asset" || err "download failed: $url"
    tar -xzf "$tmp/$asset" -C "$tmp"

    mkdir -p "$install_dir"
    install -m 755 "$tmp/netpulse" "$install_dir/netpulse" 2>/dev/null \
        || { cp "$tmp/netpulse" "$install_dir/netpulse" && chmod 755 "$install_dir/netpulse"; }

    say "installed to $install_dir/netpulse"

    case ":$PATH:" in
        *":$install_dir:"*)
            say "run: netpulse"
            ;;
        *)
            say ""
            say "NOTE: $install_dir is not on your PATH. Add it with:"
            case "${SHELL:-}" in
                */zsh)  say "  echo 'export PATH=\"$install_dir:\$PATH\"' >> ~/.zshrc && source ~/.zshrc" ;;
                */fish) say "  fish_add_path $install_dir" ;;
                *)      say "  echo 'export PATH=\"$install_dir:\$PATH\"' >> ~/.bashrc && source ~/.bashrc" ;;
            esac
            ;;
    esac

    if command -v "$install_dir/netpulse" >/dev/null 2>&1; then
        "$install_dir/netpulse" --version
    fi
}

say() { printf '%s\n' "$1"; }
err() { printf 'error: %s\n' "$1" >&2; exit 1; }

main
