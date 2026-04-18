#!/usr/bin/env bash
#
# ccbootstrap — installer for macOS Apple Silicon
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/abdessama-cto/ccb/master/install.sh | bash
#
# Environment variables:
#   CCBOOTSTRAP_VERSION   Target version (default: latest)
#   CCBOOTSTRAP_NO_BREW   If set to "1", skip Homebrew/deps bootstrap
#   CCBOOTSTRAP_PREFIX    Install prefix (default: $HOME/.local/bin)
#
# License: MIT

set -euo pipefail

# ─── Constants ────────────────────────────────────────────────────────────────
readonly INSTALLER_VERSION="3.0.0"
readonly GITHUB_REPO="abdessama-cto/ccb"
readonly BINARY_NAME="ccbootstrap"
readonly BIN_DIR="${CCBOOTSTRAP_PREFIX:-${HOME}/.local/bin}"
readonly CONFIG_DIR="${HOME}/.ccbootstrap"
readonly TARGET_VERSION="${CCBOOTSTRAP_VERSION:-latest}"

# ─── Colors & logging ────────────────────────────────────────────────────────
if [[ -t 1 ]] && [[ "${TERM:-dumb}" != "dumb" ]]; then
  readonly C_RESET=$'\033[0m'
  readonly C_RED=$'\033[0;31m'
  readonly C_GREEN=$'\033[0;32m'
  readonly C_YELLOW=$'\033[0;33m'
  readonly C_BLUE=$'\033[0;34m'
  readonly C_DIM=$'\033[2m'
  readonly C_BOLD=$'\033[1m'
else
  readonly C_RESET='' C_RED='' C_GREEN='' C_YELLOW='' C_BLUE='' C_DIM='' C_BOLD=''
fi

info()    { printf "%s▸%s %s\n"    "$C_BLUE"   "$C_RESET" "$*"; }
success() { printf "%s✓%s %s\n"    "$C_GREEN"  "$C_RESET" "$*"; }
warn()    { printf "%s⚠%s  %s\n"   "$C_YELLOW" "$C_RESET" "$*" >&2; }
err()     { printf "%s✗%s %s\n"    "$C_RED"    "$C_RESET" "$*" >&2; }
fatal()   { err "$*"; exit 1; }

# ─── Cleanup trap ────────────────────────────────────────────────────────────
TMPDIR_LOCAL=""
cleanup() {
  local exit_code=$?
  if [[ -n "$TMPDIR_LOCAL" && -d "$TMPDIR_LOCAL" ]]; then
    rm -rf "$TMPDIR_LOCAL"
  fi
  if (( exit_code != 0 )); then
    err "Installation failed (exit $exit_code). See output above."
    err "Report issues at https://github.com/${GITHUB_REPO}/issues"
  fi
}
trap cleanup EXIT
trap 'fatal "Aborted by user"' INT TERM

# ─── Banner ──────────────────────────────────────────────────────────────────
print_banner() {
  cat <<EOF

   ${C_GREEN}🌱 ccbootstrap${C_RESET} ${C_DIM}— Claude Code Project Bootstrapper${C_RESET}
   ${C_DIM}installer v${INSTALLER_VERSION} · macOS Apple Silicon · $(date +%Y-%m-%d)${C_RESET}

EOF
}

# ─── Step 1: platform check ──────────────────────────────────────────────────
check_platform() {
  info "Checking platform..."

  if [[ "$(uname -s)" != "Darwin" ]]; then
    fatal "ccbootstrap v3 requires macOS (detected: $(uname -s)).
  Linux/Windows support is on the v1.5 roadmap.
  See https://ccbootstrap.dev/other-platforms"
  fi

  if [[ "$(uname -m)" != "arm64" ]]; then
    fatal "ccbootstrap v3 requires Apple Silicon (M1/M2/M3/M4).
  Detected architecture: $(uname -m)
  Intel Mac support is on the v1.5 roadmap.
  See https://ccbootstrap.dev/other-platforms"
  fi

  local macos_major
  macos_major=$(sw_vers -productVersion 2>/dev/null | cut -d. -f1 || echo "0")
  if (( macos_major < 13 )); then
    warn "macOS $(sw_vers -productVersion 2>/dev/null) detected — 13+ recommended (Gatekeeper/notarization)"
  fi

  local cpu_brand
  cpu_brand=$(sysctl -n machdep.cpu.brand_string 2>/dev/null || echo "Apple Silicon")
  success "macOS $(sw_vers -productVersion 2>/dev/null) · ${cpu_brand}"
}

# ─── Step 2: Homebrew ────────────────────────────────────────────────────────
install_homebrew() {
  if [[ "${CCBOOTSTRAP_NO_BREW:-0}" == "1" ]]; then
    warn "CCBOOTSTRAP_NO_BREW=1 — skipping Homebrew step"
    return 0
  fi

  # Check common Homebrew locations if not in PATH
  if ! command -v brew >/dev/null 2>&1; then
    if [[ -x /opt/homebrew/bin/brew ]]; then
      eval "$(/opt/homebrew/bin/brew shellenv)"
    elif [[ -x /usr/local/bin/brew ]]; then
      eval "$(/usr/local/bin/brew shellenv)"
    fi
  fi

  if command -v brew >/dev/null 2>&1; then
    success "Homebrew present ($(brew --version 2>/dev/null | head -1))"
    return 0
  fi

  info "Installing Homebrew..."
  NONINTERACTIVE=1 /bin/bash -c \
    "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)" \
    || fatal "Homebrew installation failed"

  # Apple Silicon default location
  if [[ -x /opt/homebrew/bin/brew ]]; then
    eval "$(/opt/homebrew/bin/brew shellenv)"
  fi

  command -v brew >/dev/null 2>&1 || fatal "brew not on PATH after install"
  success "Homebrew installed"
}

# ─── Step 3: runtime dependencies ────────────────────────────────────────────
install_dependencies() {
  if [[ "${CCBOOTSTRAP_NO_BREW:-0}" == "1" ]]; then
    return 0
  fi

  local deps=(git gh jq node)
  info "Checking dependencies: ${deps[*]}"

  for dep in "${deps[@]}"; do
    if command -v "$dep" >/dev/null 2>&1 || brew list "$dep" >/dev/null 2>&1; then
      printf "  %s·%s %s %s(already installed)%s\n" \
        "$C_DIM" "$C_RESET" "$dep" "$C_DIM" "$C_RESET"
    else
      printf "  %s▸%s installing %s...\n" "$C_BLUE" "$C_RESET" "$dep"
      brew install "$dep" >/dev/null 2>&1 \
        || fatal "Failed to install $dep via Homebrew"
      printf "  %s✓%s %s\n" "$C_GREEN" "$C_RESET" "$dep"
    fi
  done

  # Minimum versions
  local node_major
  node_major=$(node -v 2>/dev/null | sed -E 's/^v([0-9]+).*/\1/' || echo "0")
  if (( node_major < 20 )); then
    warn "Node $(node -v 2>/dev/null) is below the recommended 20 LTS. Run: brew upgrade node"
  fi
}

# ─── Step 4: resolve version ─────────────────────────────────────────────────
resolve_version() {
  if [[ "$TARGET_VERSION" == "latest" ]]; then
    info "Resolving latest release from GitHub..."
    local api_url redirect_url api_json api_exit fallback_tag
    api_url="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
    redirect_url="https://github.com/${GITHUB_REPO}/releases/latest"
    api_exit=0

    # Try GitHub API first (most reliable source for tag_name).
    api_json=""
    local http_code
    set +e
    # Use -w to get HTTP status code and -s (silent) to avoid progress bar
    # We remove -f to handle the error manually based on http_code
    api_json=$(curl -sSL --http1.1 --retry 3 --retry-delay 2 --connect-timeout 15 \
      -H "Accept: application/vnd.github+json" \
      -w "%{http_code}" \
      "$api_url")
    api_exit=$?
    set -e

    if [[ "$api_exit" -eq 0 ]]; then
      http_code="${api_json: -3}"
      api_json="${api_json%???}"
      
      if [[ "$http_code" -eq 200 ]]; then
        if command -v jq >/dev/null 2>&1; then
          RESOLVED_VERSION=$(printf '%s' "$api_json" | jq -r '.tag_name // empty' 2>/dev/null || true)
        else
          RESOLVED_VERSION=$(printf '%s' "$api_json" \
            | tr -d '\n' \
            | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
        fi
      elif [[ "$http_code" -eq 404 ]]; then
        info "Repository found, but no releases are published yet."
      elif [[ "$http_code" -eq 403 ]]; then
        warn "GitHub API rate limit exceeded or access denied (HTTP 403)."
      fi
    fi

    # Fallback resolver (scraping redirect)
    if [[ -z "${RESOLVED_VERSION:-}" ]]; then
      info "Trying fallback resolver..."
      fallback_tag=$(
        curl -fsSI --http1.1 --retry 2 --retry-delay 1 --connect-timeout 10 \
          "$redirect_url" 2>/dev/null \
        | awk -F'/' 'BEGIN{IGNORECASE=1} /^location:/ {print $NF}' \
        | tr -d '\r' \
        | tail -1
      ) || true
      
      # If fallback_tag is "releases", it means the redirect was to the general releases page
      if [[ "$fallback_tag" == "releases" ]]; then
        fallback_tag=""
      fi
      RESOLVED_VERSION="${fallback_tag:-}"
    fi

    if [[ -z "${RESOLVED_VERSION:-}" ]]; then
      fatal "Could not resolve 'latest' tag.
  Possible causes:
    - No releases published at https://github.com/${GITHUB_REPO}/releases
    - Firewall/proxy or no internet connection
    - GitHub API rate limit
  
  Try pinning a version: CCBOOTSTRAP_VERSION=v1.0.0 curl -fsSL ... | bash"
    fi
  else
    RESOLVED_VERSION="$TARGET_VERSION"
  fi
  readonly RESOLVED_VERSION
  success "Target release: ${C_BOLD}${RESOLVED_VERSION}${C_RESET}"
}

# ─── Step 5: existing install check ──────────────────────────────────────────
is_already_installed() {
  [[ ! -x "${BIN_DIR}/${BINARY_NAME}" ]] && return 1

  local current
  current=$("${BIN_DIR}/${BINARY_NAME}" --version 2>/dev/null \
    | head -1 | awk '{print $NF}' || echo "unknown")

  # Normalise v-prefix for comparison
  local target="${RESOLVED_VERSION#v}"
  local have="${current#v}"

  if [[ "$target" == "$have" ]]; then
    success "ccbootstrap ${current} already installed and up to date"
    return 0
  fi

  info "Upgrading: ${current} → ${RESOLVED_VERSION}"
  return 1
}

# ─── Step 6: download, verify, install ───────────────────────────────────────
download_and_install() {
  TMPDIR_LOCAL=$(mktemp -d -t ccbootstrap-install-XXXXXX)

  local base_url="https://github.com/${GITHUB_REPO}/releases/download/${RESOLVED_VERSION}"
  local binary_url="${base_url}/${BINARY_NAME}-darwin-arm64"
  local sha_url="${binary_url}.sha256"
  local tmp_bin="${TMPDIR_LOCAL}/${BINARY_NAME}"
  local tmp_sha="${TMPDIR_LOCAL}/${BINARY_NAME}.sha256"

  info "Downloading binary (arm64-darwin)..."
  if ! curl -fsSL --retry 3 --retry-delay 2 --connect-timeout 15 \
         -o "$tmp_bin" "$binary_url"; then
    fatal "Failed to download binary.
  URL: $binary_url
  Check https://github.com/${GITHUB_REPO}/releases"
  fi

  local size_mb
  size_mb=$(du -m "$tmp_bin" | awk '{print $1}')
  success "Downloaded (${size_mb} MB)"

  # SHA256 verification — REQUIRED for supply-chain safety
  info "Verifying SHA256 integrity..."
  if curl -fsSL --retry 2 --connect-timeout 10 \
       -o "$tmp_sha" "$sha_url" 2>/dev/null; then
    local expected actual
    expected=$(awk '{print $1}' "$tmp_sha" | tr -d '[:space:]')
    actual=$(shasum -a 256 "$tmp_bin" | awk '{print $1}')

    if [[ -z "$expected" ]]; then
      fatal "Checksum file empty or malformed: $sha_url"
    fi

    if [[ "$expected" != "$actual" ]]; then
      fatal "SHA256 mismatch — binary may be corrupted or tampered!
  Expected: $expected
  Got:      $actual
  Aborting install. Report at https://github.com/${GITHUB_REPO}/issues"
    fi
    success "Checksum matches (${actual:0:12}...)"
  else
    warn "Checksum file missing at ${sha_url}
  Proceeding WITHOUT integrity verification (acceptable only for dev builds)."
  fi

  # Gatekeeper / notarization probe
  info "Checking Apple notarization..."
  if /usr/sbin/spctl --assess --type execute "$tmp_bin" 2>/dev/null; then
    success "Notarization OK (Gatekeeper will accept)"
  else
    warn "Binary not notarized for Gatekeeper. If macOS blocks it at first run:
       xattr -d com.apple.quarantine \"${BIN_DIR}/${BINARY_NAME}\""
  fi

  # Atomic install
  mkdir -p "$BIN_DIR"
  install -m 0755 "$tmp_bin" "${BIN_DIR}/${BINARY_NAME}"
  success "Installed to ${C_BOLD}${BIN_DIR}/${BINARY_NAME}${C_RESET}"
}

# ─── Step 7: PATH setup ──────────────────────────────────────────────────────
setup_path() {
  if [[ ":${PATH}:" == *":${BIN_DIR}:"* ]]; then
    success "$BIN_DIR already in PATH"
    return 0
  fi

  # Detect user's shell (SHELL env var reflects login shell on macOS)
  local user_shell shell_rc path_line marker
  user_shell=$(basename "${SHELL:-/bin/zsh}")

  case "$user_shell" in
    zsh)
      shell_rc="${HOME}/.zshrc"
      path_line="export PATH=\"${BIN_DIR}:\$PATH\""
      ;;
    bash)
      # macOS: .bash_profile is sourced by login shells; .bashrc by interactive non-login
      shell_rc="${HOME}/.bash_profile"
      [[ -f "${HOME}/.bashrc" && ! -f "${HOME}/.bash_profile" ]] && shell_rc="${HOME}/.bashrc"
      path_line="export PATH=\"${BIN_DIR}:\$PATH\""
      ;;
    fish)
      shell_rc="${HOME}/.config/fish/config.fish"
      mkdir -p "$(dirname "$shell_rc")"
      path_line="fish_add_path ${BIN_DIR}"
      ;;
    *)
      shell_rc="${HOME}/.profile"
      path_line="export PATH=\"${BIN_DIR}:\$PATH\""
      warn "Unknown shell '$user_shell' — falling back to ~/.profile"
      ;;
  esac

  marker="# ccbootstrap installer — PATH entry"

  if [[ -f "$shell_rc" ]] && grep -qF "$BIN_DIR" "$shell_rc" 2>/dev/null; then
    success "$BIN_DIR already referenced in $shell_rc"
  else
    {
      printf '\n%s (added %s)\n' "$marker" "$(date -Iseconds 2>/dev/null || date)"
      printf '%s\n' "$path_line"
    } >> "$shell_rc"
    success "Added $BIN_DIR to PATH in $shell_rc"
    warn "Reload your shell: ${C_BOLD}source $shell_rc${C_RESET}   (or open a new terminal)"
  fi
}

# ─── Step 8: config directory ────────────────────────────────────────────────
setup_config() {
  mkdir -p \
    "${CONFIG_DIR}/cache/templates" \
    "${CONFIG_DIR}/projects" \
    "${CONFIG_DIR}/logs"

  if [[ ! -f "${CONFIG_DIR}/config.yaml" ]]; then
    cat > "${CONFIG_DIR}/config.yaml" <<EOF
# ccbootstrap config — edit via: ccbootstrap settings
version: "${RESOLVED_VERSION}"
installed_at: "$(date -Iseconds 2>/dev/null || date)"
installer_version: "${INSTALLER_VERSION}"

ai:
  enabled: true
  provider: openai
  model: gpt-5.4-nano
  monthly_budget_usd: 5.00

ui:
  language: auto
  color_scheme: auto
  verbosity: normal

defaults:
  profile: balanced
  auto_pr: true
  auto_run_tests: true
  skip_confirmations: false
EOF
    success "Created ${CONFIG_DIR}/config.yaml (defaults)"
  else
    success "Existing config preserved: ${CONFIG_DIR}/config.yaml"
  fi
}

# ─── Step 9: sanity check the install ────────────────────────────────────────
verify_install() {
  if ! "${BIN_DIR}/${BINARY_NAME}" --version >/dev/null 2>&1; then
    warn "Binary installed but --version check failed.
  This may be a Gatekeeper quarantine issue on first run.
  Try: xattr -d com.apple.quarantine ${BIN_DIR}/${BINARY_NAME}"
    return 0
  fi
  local v
  v=$("${BIN_DIR}/${BINARY_NAME}" --version 2>/dev/null | head -1)
  success "Binary responds: ${v}"
}

# ─── Next steps ──────────────────────────────────────────────────────────────
print_next_steps() {
  cat <<EOF

${C_GREEN}${C_BOLD}🎉 ccbootstrap ${RESOLVED_VERSION} installed${C_RESET}

${C_BOLD}Next steps${C_RESET}

  ${C_DIM}# 1. Reload your shell (or open a new terminal)${C_RESET}
  source ~/.zshrc

  ${C_DIM}# 2. Configure credentials (OpenAI, GitHub auth)${C_RESET}
  ccbootstrap settings

  ${C_DIM}# 3. Bootstrap your first repo${C_RESET}
  ccbootstrap init https://github.com/<owner>/<repo>

${C_BOLD}Docs${C_RESET}   https://ccbootstrap.dev/docs
${C_BOLD}Repo${C_RESET}   https://github.com/${GITHUB_REPO}
${C_BOLD}Issues${C_RESET} https://github.com/${GITHUB_REPO}/issues

EOF
}

# ─── Main ────────────────────────────────────────────────────────────────────
main() {
  print_banner
  check_platform
  install_homebrew
  install_dependencies
  resolve_version
  if ! is_already_installed; then
    download_and_install
  fi
  setup_path
  setup_config
  verify_install
  print_next_steps
}

main "$@"
