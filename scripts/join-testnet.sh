#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════
# Zerone — Join Testnet
# ═══════════════════════════════════════════════════════════════════════════
#
# Configures a Zerone node to join zerone-testnet-1.
#
# Usage:
#   scripts/join-testnet.sh [OPTIONS]
#
# Options:
#   --moniker NAME        Node moniker (default: hostname)
#   --home DIR            zeroned home directory (default: $HOME/.zeroned)
#   --seeds FILE          Path to seeds.txt (default: seeds.txt in repo root)
#   --genesis URL         Genesis file URL (or local path)
#   --cosmovisor          Set up Cosmovisor process manager
#   --systemd             Generate systemd service file
#   --reset               Reset existing state before joining (DESTRUCTIVE)
#   --help                Show this help message
#
# Supports: Ubuntu 22.04+, macOS 13+
# Requires: zeroned (in PATH or built via make install), jq, curl
# ═══════════════════════════════════════════════════════════════════════════

set -euo pipefail

# ── Constants ────────────────────────────────────────────────────────────

CHAIN_ID="zerone-testnet-1"
DENOM="uzrn"
MIN_GAS_PRICES="0.025${DENOM}"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Defaults
MONIKER="$(hostname)"
ZERONED_HOME="${HOME}/.zeroned"
SEEDS_FILE="${PROJECT_ROOT}/seeds.txt"
GENESIS_URL=""
SETUP_COSMOVISOR=false
SETUP_SYSTEMD=false
RESET_STATE=false

# ── Helpers ──────────────────────────────────────────────────────────────

die()  { echo -e "\033[1;31mERROR:\033[0m $*" >&2; exit 1; }
info() { echo -e "\033[1;34m  ->\033[0m $*"; }
ok()   { echo -e "\033[1;32m  OK\033[0m $*"; }
warn() { echo -e "\033[1;33m  !!\033[0m $*"; }

usage() {
  head -27 "$0" | tail -18
  exit 0
}

# ── Parse arguments ─────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
  case "$1" in
    --moniker)    MONIKER="$2"; shift 2 ;;
    --home)       ZERONED_HOME="$2"; shift 2 ;;
    --seeds)      SEEDS_FILE="$2"; shift 2 ;;
    --genesis)    GENESIS_URL="$2"; shift 2 ;;
    --cosmovisor) SETUP_COSMOVISOR=true; shift ;;
    --systemd)    SETUP_SYSTEMD=true; shift ;;
    --reset)      RESET_STATE=true; shift ;;
    --help|-h)    usage ;;
    *) die "Unknown option: $1. Run with --help for usage." ;;
  esac
done

# ── Dependency checks ───────────────────────────────────────────────────

check_deps() {
  command -v zeroned >/dev/null 2>&1 || die "zeroned not found in PATH. Run: make install"
  command -v jq >/dev/null 2>&1      || die "jq required. Install: brew install jq (macOS) or apt install jq (Ubuntu)"
  command -v curl >/dev/null 2>&1    || die "curl required."
}

# ── Detect OS ────────────────────────────────────────────────────────────

detect_os() {
  case "$(uname -s)" in
    Linux*)  OS="linux" ;;
    Darwin*) OS="macos" ;;
    *) die "Unsupported operating system: $(uname -s)" ;;
  esac
}

# ── Read seeds ───────────────────────────────────────────────────────────

read_seeds() {
  if [[ ! -f "${SEEDS_FILE}" ]]; then
    warn "Seeds file not found: ${SEEDS_FILE}"
    warn "Node will start without seeds. Add seeds to config.toml manually."
    SEEDS=""
    return
  fi

  # Read non-comment, non-empty lines and join with commas
  SEEDS="$(grep -v '^\s*#' "${SEEDS_FILE}" | grep -v '^\s*$' | tr '\n' ',' | sed 's/,$//')"

  if [[ -z "${SEEDS}" ]]; then
    warn "No seed entries found in ${SEEDS_FILE}. Add seeds after genesis ceremony."
  else
    ok "Loaded seeds: ${SEEDS}"
  fi
}

# ── Initialize node ─────────────────────────────────────────────────────

init_node() {
  if [[ -d "${ZERONED_HOME}/config" ]] && [[ "${RESET_STATE}" != true ]]; then
    info "Node already initialized at ${ZERONED_HOME}"
    info "Using existing configuration. Pass --reset to reinitialize."
    return
  fi

  if [[ "${RESET_STATE}" == true ]] && [[ -d "${ZERONED_HOME}" ]]; then
    warn "Resetting existing state at ${ZERONED_HOME}"
    rm -rf "${ZERONED_HOME}/data"
  fi

  info "Initializing node: moniker=${MONIKER} chain-id=${CHAIN_ID}"
  zeroned init "${MONIKER}" --chain-id "${CHAIN_ID}" --home "${ZERONED_HOME}" 2>/dev/null
  ok "Node initialized at ${ZERONED_HOME}"
}

# ── Fetch genesis ────────────────────────────────────────────────────────

fetch_genesis() {
  local genesis_path="${ZERONED_HOME}/config/genesis.json"

  if [[ -z "${GENESIS_URL}" ]]; then
    info "No --genesis URL provided."
    info "Copy the official genesis.json to: ${genesis_path}"
    return
  fi

  if [[ -f "${GENESIS_URL}" ]]; then
    # Local file
    info "Copying genesis from local file: ${GENESIS_URL}"
    cp "${GENESIS_URL}" "${genesis_path}"
  else
    # Remote URL
    info "Downloading genesis from: ${GENESIS_URL}"
    curl -sSfL "${GENESIS_URL}" -o "${genesis_path}" || die "Failed to download genesis file"
  fi

  # Validate (SDK v0.50 uses 'genesis validate', older uses 'validate-genesis')
  if ! zeroned genesis validate --home "${ZERONED_HOME}" 2>/dev/null; then
    zeroned validate-genesis --home "${ZERONED_HOME}" 2>/dev/null || die "Genesis file validation failed"
  fi

  ok "Genesis file installed and validated"
}

# ── Configure node ───────────────────────────────────────────────────────

configure_node() {
  # Delegate to configure-node.sh for standardised mode-based configuration
  if [ -f "${PROJECT_ROOT}/scripts/configure-node.sh" ]; then
    info "Applying recommended node configuration..."
    bash "${PROJECT_ROOT}/scripts/configure-node.sh" \
        --home "${ZERONED_HOME}" \
        --mode validator \
        --moniker "${MONIKER}"
  else
    warn "configure-node.sh not found; applying minimal config..."

    local config_toml="${ZERONED_HOME}/config/config.toml"
    local app_toml="${ZERONED_HOME}/config/app.toml"

    # Portable in-place sed
    sedi() {
      if [[ "${OS}" == "macos" ]]; then
        sed -i '' "$@"
      else
        sed -i "$@"
      fi
    }

    sedi "s/^moniker = .*/moniker = \"${MONIKER}\"/" "${config_toml}"
    sedi "s/^timeout_commit = .*/timeout_commit = \"2521ms\"/" "${config_toml}"
    sedi "s/^minimum-gas-prices = .*/minimum-gas-prices = \"${MIN_GAS_PRICES}\"/" "${app_toml}"
    ok "Minimal configuration applied"
  fi

  # Patch seeds separately (join-testnet reads them from seeds.txt)
  if [[ -n "${SEEDS}" ]]; then
    local config_toml="${ZERONED_HOME}/config/config.toml"
    sedi_join() {
      if [[ "${OS}" == "macos" ]]; then
        sed -i '' "$@"
      else
        sed -i "$@"
      fi
    }
    sedi_join "s/^seeds = .*/seeds = \"${SEEDS}\"/" "${config_toml}"
    ok "Seeds configured"
  fi
}

# ── Cosmovisor setup ─────────────────────────────────────────────────────

setup_cosmovisor() {
  if [[ "${SETUP_COSMOVISOR}" != true ]]; then
    return
  fi

  info "Setting up Cosmovisor..."

  # Install cosmovisor if not present
  if ! command -v cosmovisor >/dev/null 2>&1; then
    info "Installing cosmovisor..."
    go install cosmossdk.io/tools/cosmovisor/cmd/cosmovisor@latest || die "Failed to install cosmovisor"
  fi

  # Create directory structure
  local cv_dir="${ZERONED_HOME}/cosmovisor"
  mkdir -p "${cv_dir}/genesis/bin"
  mkdir -p "${cv_dir}/upgrades"

  # Copy current binary
  local zeroned_path
  zeroned_path="$(command -v zeroned)"
  cp "${zeroned_path}" "${cv_dir}/genesis/bin/zeroned"

  # Write environment file
  local env_file="${ZERONED_HOME}/cosmovisor.env"
  cat > "${env_file}" <<EOF
DAEMON_NAME=zeroned
DAEMON_HOME=${ZERONED_HOME}
DAEMON_ALLOW_DOWNLOAD_BINARIES=false
DAEMON_RESTART_AFTER_UPGRADE=true
DAEMON_LOG_BUFFER_SIZE=512
EOF

  ok "Cosmovisor configured at ${cv_dir}"
  info "Start with: source ${env_file} && cosmovisor run start"
}

# ── systemd service ──────────────────────────────────────────────────────

setup_systemd() {
  if [[ "${SETUP_SYSTEMD}" != true ]]; then
    return
  fi

  if [[ "${OS}" != "linux" ]]; then
    warn "systemd service only supported on Linux. Skipping."
    return
  fi

  local service_file="${ZERONED_HOME}/zeroned.service"
  local exec_start

  if [[ "${SETUP_COSMOVISOR}" == true ]]; then
    exec_start="$(command -v cosmovisor) run start --home ${ZERONED_HOME}"
    local env_file="${ZERONED_HOME}/cosmovisor.env"
    cat > "${service_file}" <<EOF
[Unit]
Description=Zerone Node (zerone-testnet-1)
After=network-online.target
Wants=network-online.target

[Service]
User=${USER}
ExecStart=${exec_start}
Restart=always
RestartSec=3
LimitNOFILE=65535
EnvironmentFile=${env_file}

[Install]
WantedBy=multi-user.target
EOF
  else
    exec_start="$(command -v zeroned) start --home ${ZERONED_HOME} --minimum-gas-prices ${MIN_GAS_PRICES}"
    cat > "${service_file}" <<EOF
[Unit]
Description=Zerone Node (zerone-testnet-1)
After=network-online.target
Wants=network-online.target

[Service]
User=${USER}
ExecStart=${exec_start}
Restart=always
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
  fi

  ok "systemd service file written to: ${service_file}"
  echo ""
  info "To install the service:"
  info "  sudo cp ${service_file} /etc/systemd/system/zeroned.service"
  info "  sudo systemctl daemon-reload"
  info "  sudo systemctl enable zeroned"
  info "  sudo systemctl start zeroned"
  info "  sudo journalctl -u zeroned -f"
}

# ── Main ─────────────────────────────────────────────────────────────────

main() {
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Join Testnet"
  echo "  Chain: ${CHAIN_ID} | Moniker: ${MONIKER}"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  check_deps
  detect_os
  read_seeds
  init_node
  fetch_genesis
  configure_node
  setup_cosmovisor
  setup_systemd

  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  ok "Node configured for ${CHAIN_ID}"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  info "Next steps:"
  info "  1. Obtain the official genesis.json (if not already provided)"
  info "  2. Add seed nodes to ${ZERONED_HOME}/config/config.toml"
  info "  3. Start your node:"
  echo ""

  if [[ "${SETUP_COSMOVISOR}" == true ]]; then
    info "     source ${ZERONED_HOME}/cosmovisor.env"
    info "     cosmovisor run start"
  elif [[ "${SETUP_SYSTEMD}" == true ]]; then
    info "     sudo systemctl start zeroned"
  else
    info "     zeroned start --home ${ZERONED_HOME} --minimum-gas-prices ${MIN_GAS_PRICES}"
  fi

  echo ""
  info "  4. Wait for node to sync (check: zeroned status | jq .sync_info)"
  info "  5. Register as validator (see docs/VALIDATOR-GUIDE.md)"
  echo ""
}

main "$@"
