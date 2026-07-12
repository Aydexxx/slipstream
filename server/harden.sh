#!/usr/bin/env bash
#
# harden.sh — Slipstream Private Mode VPS hardening (Ubuntu 24.04 / Debian 12+)
#
# Run this AFTER amneziawg-installer has created the AmneziaWG interface, on a
# fresh VPS you control. It is idempotent — safe to re-run.
#
#   Firewall (UFW): default deny inbound, allow ONLY SSH + the AmneziaWG UDP
#                   port, and permit routed VPN traffic to reach the internet.
#   Fail2Ban:       ban brute-force SSH sources (systemd/journald backend).
#   SSH:            key-only login (password auth disabled) — with a guard that
#                   refuses to lock you out if no authorized key is present.
#
# Usage (as root):
#   ./harden.sh --awg-port <UDP_PORT> [--ssh-port <PORT>]
#
# If --awg-port is omitted the script tries to read ListenPort from the
# AmneziaWG server config. --ssh-port defaults to 22.
#
# SAFETY: before this script disables SSH password auth it verifies that at
# least one SSH public key is installed for the login user(s). Still, KEEP YOUR
# CURRENT SSH SESSION OPEN and confirm you can open a SECOND session with your
# key before you log out.

set -euo pipefail

SSH_PORT=22
AWG_PORT=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --awg-port) AWG_PORT="${2:?--awg-port needs a value}"; shift 2 ;;
    --ssh-port) SSH_PORT="${2:?--ssh-port needs a value}"; shift 2 ;;
    -h|--help)  grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

log()  { printf '\033[1;36m[harden]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[harden] WARNING:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[harden] ERROR:\033[0m %s\n' "$*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "must run as root (try: sudo $0 ...)"
command -v apt-get >/dev/null || die "this script targets Debian/Ubuntu (apt not found)"

# --- Resolve the AmneziaWG UDP port ------------------------------------------
if [[ -z "$AWG_PORT" ]]; then
  for cfg in /etc/amnezia/amneziawg/*.conf /etc/amneziawg/*.conf /etc/wireguard/*.conf; do
    [[ -f "$cfg" ]] || continue
    p="$(grep -iE '^\s*ListenPort\s*=' "$cfg" | head -n1 | grep -oE '[0-9]+' || true)"
    if [[ -n "${p:-}" ]]; then AWG_PORT="$p"; log "detected AmneziaWG ListenPort=$AWG_PORT (from $cfg)"; break; fi
  done
fi
[[ -n "$AWG_PORT" ]] || die "could not determine the AmneziaWG UDP port; pass --awg-port <PORT>"
[[ "$AWG_PORT" =~ ^[0-9]+$ && "$AWG_PORT" -ge 1 && "$AWG_PORT" -le 65535 ]] || die "invalid AWG port: $AWG_PORT"
[[ "$SSH_PORT" =~ ^[0-9]+$ && "$SSH_PORT" -ge 1 && "$SSH_PORT" -le 65535 ]] || die "invalid SSH port: $SSH_PORT"

export DEBIAN_FRONTEND=noninteractive

# --- Packages ----------------------------------------------------------------
log "installing ufw + fail2ban"
apt-get update -qq
apt-get install -y -qq ufw fail2ban >/dev/null

# --- UFW ---------------------------------------------------------------------
# VPN routing needs FORWARD permitted; UFW defaults FORWARD to DROP, which
# silently breaks the tunnel's internet access. Set it to ACCEPT (single-user
# VPN box) so packets from AWG peers can be NAT'd out by the installer's rules.
log "configuring UFW (allow SSH/$SSH_PORT + AmneziaWG udp/$AWG_PORT, deny the rest)"
sed -i 's/^DEFAULT_FORWARD_POLICY=.*/DEFAULT_FORWARD_POLICY="ACCEPT"/' /etc/default/ufw

ufw --force reset >/dev/null
ufw default deny incoming
ufw default allow outgoing
ufw allow "${SSH_PORT}/tcp"  comment 'SSH'
ufw allow "${AWG_PORT}/udp"  comment 'AmneziaWG'
ufw --force enable
ufw status verbose

# --- Fail2Ban ----------------------------------------------------------------
log "configuring Fail2Ban sshd jail"
cat > /etc/fail2ban/jail.local <<EOF
# Managed by Slipstream server/harden.sh
[DEFAULT]
backend  = systemd
bantime  = 1h
findtime = 10m
maxretry = 5

[sshd]
enabled = true
port    = ${SSH_PORT}
EOF
systemctl enable --now fail2ban >/dev/null 2>&1 || true
systemctl restart fail2ban
sleep 1
fail2ban-client status sshd || warn "fail2ban sshd jail not reporting yet (check 'journalctl -u fail2ban')"

# --- SSH: key-only, with a lockout guard -------------------------------------
log "checking for installed SSH public keys before disabling password auth"
have_keys=0
shopt -s nullglob
for ak in /root/.ssh/authorized_keys /home/*/.ssh/authorized_keys; do
  [[ -s "$ak" ]] && grep -qE '^(ssh-|ecdsa-|sk-)' "$ak" && have_keys=1
done
shopt -u nullglob

if [[ "$have_keys" -eq 1 ]]; then
  install -d -m 0755 /etc/ssh/sshd_config.d
  cat > /etc/ssh/sshd_config.d/99-slipstream-hardening.conf <<EOF
# Managed by Slipstream server/harden.sh
Port ${SSH_PORT}
PasswordAuthentication no
KbdInteractiveAuthentication no
ChallengeResponseAuthentication no
PubkeyAuthentication yes
PermitRootLogin prohibit-password
EOF
  if sshd -t; then
    systemctl reload ssh 2>/dev/null || systemctl reload sshd
    log "SSH hardened: password auth DISABLED, key-only login enforced."
    warn "Open a SECOND SSH session NOW with your key to confirm access before logging out."
  else
    rm -f /etc/ssh/sshd_config.d/99-slipstream-hardening.conf
    die "sshd config test failed; reverted SSH hardening (no changes applied)."
  fi
else
  warn "No SSH public key found for root or any user — SKIPPING password-auth disable to avoid locking you out."
  warn "Install your key (ssh-copy-id / paste into ~/.ssh/authorized_keys), then re-run this script."
fi

log "done. Firewall + Fail2Ban active; SSH is key-only if a key was present."
