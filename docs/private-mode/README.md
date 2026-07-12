# Private Mode — self-hosted AmneziaWG endpoint (Phase 4)

Private Mode routes traffic through a **self-hosted AmneziaWG** (obfuscated
WireGuard) endpoint on a VPS outside Turkey. Same crypto as WireGuard, but the
wire format is masked (junk/decoy packets, randomized headers) so Turkey's DPI
can't fingerprint-and-throttle it. Turkey is a lighter DPI zone, so AmneziaWG
alone is enough — no Xray/REALITY.

This directory is the **operator runbook** for standing up that server and
producing the client config. It is documentation + scripts you run on a VPS you
control; **the app does not automate any of it.** Personal VPN use is legal in
Turkey.

## Read in this order

1. **[PROVISIONING.md](./PROVISIONING.md)** — pick a provider/region (non-Hetzner
   EU default), install AmneziaWG via `amneziawg-installer`, harden the box, and
   export the client peer.
2. **[OBFUSCATION.md](./OBFUSCATION.md)** — the exact obfuscation parameters and
   the **QUIC (`I1`) preset** for Turkish ISPs, with the validation rules and
   version-compat notes.
3. **[VERIFICATION.md](./VERIFICATION.md)** — connect with the **official Amnezia
   client** and confirm the exit IP is the VPS. This is the phase's "Done when".

Supporting files:
- **[client.conf.template](./client.conf.template)** — annotated client config
  (placeholders only, no secrets).
- **[`server/harden.sh`](../../server/harden.sh)** — UFW + Fail2Ban + key-only
  SSH hardening, idempotent, with a lockout guard.

## Done when

The Windows machine establishes an AmneziaWG tunnel to the VPS via the official
Amnezia client, and an external IP check (`api.ipify.org`) shows the **VPS IP**.

## Secrets rule (do not skip)

The real client config and any keys are **secrets** and must never be committed.

- Store the filled config at **`configs/private-mode.conf`** (git-ignored).
- `configs/`, `*.conf`, and key patterns are in [.gitignore](../../.gitignore).
- Before/after saving it, confirm git can't see it:

  ```bash
  git check-ignore -v configs/private-mode.conf   # must show it's ignored
  git status --porcelain configs/                 # must print nothing
  ```

- If a private key or PresharedKey ever leaks, **rotate the peer** on the server.
