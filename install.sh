#!/bin/sh
# Installs the latest jig release (or JIG_VERSION) for this machine:
#   curl -fsSL https://raw.githubusercontent.com/goxang/jig/main/install.sh | sh
# JIG_INSTALL_DIR picks the directory (default /usr/local/bin when writable, else ~/.local/bin);
# JIG_VERSION=v0.3.0 a release.
set -eu

repo=goxang/jig
version=${JIG_VERSION:-latest}

case $(uname -s) in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) echo "jig: no release for $(uname -s); build it with: go install github.com/$repo/cmd/jig@latest" >&2; exit 1 ;;
esac
case $(uname -m) in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) echo "jig: no release for $(uname -m); build it with: go install github.com/$repo/cmd/jig@latest" >&2; exit 1 ;;
esac

# JIG_DOWNLOAD_URL points at a mirror holding the release assets
if [ -n "${JIG_DOWNLOAD_URL:-}" ]; then
  base=$JIG_DOWNLOAD_URL
elif [ "$version" = latest ]; then
  base=https://github.com/$repo/releases/latest/download
else
  base=https://github.com/$repo/releases/download/$version
fi
asset=jig_${os}_${arch}.tar.gz

dir=${JIG_INSTALL_DIR:-}
if [ -z "$dir" ]; then
  if [ -w /usr/local/bin ]; then dir=/usr/local/bin; else dir=$HOME/.local/bin; fi
fi
mkdir -p "$dir"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "jig: downloading $asset ($version)"
curl -fsSL "$base/$asset" -o "$tmp/$asset"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"

# minisign, when installed, checks checksums.txt against goxang's key (host.OfficialKey)
if command -v minisign >/dev/null 2>&1; then
  curl -fsSL "$base/checksums.txt.minisig" -o "$tmp/checksums.txt.minisig"
  minisign -Vq -P RWSO4FXV354ZdKG69JB39vUGx5l3XooSJoWp0c8T/bWN548qByjgecLr -m "$tmp/checksums.txt" -x "$tmp/checksums.txt.minisig" || {
    echo "jig: checksums.txt is not signed by goxang" >&2
    exit 1
  }
fi

want=$(grep " $asset\$" "$tmp/checksums.txt" | cut -d' ' -f1)
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$tmp/$asset" | cut -d' ' -f1)
else
  got=$(shasum -a 256 "$tmp/$asset" | cut -d' ' -f1)
fi
if [ -z "$want" ] || [ "$want" != "$got" ]; then
  echo "jig: checksum mismatch for $asset" >&2
  exit 1
fi

tar -xzf "$tmp/$asset" -C "$tmp" jig
install -m 0755 "$tmp/jig" "$dir/jig"
echo "jig: installed $("$dir/jig" --version 2>/dev/null || echo jig) to $dir/jig"

case ":$PATH:" in
  *":$dir:"*) ;;
  *)
    # the login shell's own startup file; bash login shells (macOS terminals, ssh) skip .bashrc
    line="export PATH=\"$dir:\$PATH\""
    case "$(basename "${SHELL:-}")" in
      zsh) files="${ZDOTDIR:-$HOME}/.zshrc" ;;
      bash)
        files="$HOME/.bashrc"
        if [ "$(uname -s)" = Darwin ] || { [ -f "$HOME/.bash_profile" ] && ! grep -q bashrc "$HOME/.bash_profile"; }; then
          files="$files $HOME/.bash_profile"
        fi
        ;;
      fish) files="$HOME/.config/fish/config.fish" line="fish_add_path $dir" ;;
      *) files="$HOME/.profile" ;;
    esac
    for rc in $files; do
      grep -qs "$dir" "$rc" && continue
      mkdir -p "$(dirname "$rc")"
      printf '\n%s\n' "$line" >> "$rc"
      echo "jig: added $dir to PATH in $rc"
    done
    echo "  open a new terminal, or run: $line"
    ;;
esac

# tab completion of commands, environments, services and tasks (jig task <tab>)
case "$(basename "${SHELL:-}")" in
  zsh) rc="${ZDOTDIR:-$HOME}/.zshrc" line='autoload -Uz compinit && { (( $+functions[compdef] )) || compinit -i; } && source <(jig completion zsh)' ;;
  bash) rc="$HOME/.bashrc" line='source <(jig completion bash)' ;;
  fish) rc="$HOME/.config/fish/config.fish" line='jig completion fish | source' ;;
  *) rc="" ;;
esac
if [ -n "$rc" ] && ! grep -qs "jig completion" "$rc"; then
  mkdir -p "$(dirname "$rc")"
  printf '\n%s\n' "$line" >> "$rc"
  echo "jig: added tab completion to $rc"
fi

echo "jig: next, in a project with a jig.yaml:"
echo "  jig doctor   # what this machine still needs (plugins, trust, credentials) and the command for each"
echo "  jig          # the control plane"
