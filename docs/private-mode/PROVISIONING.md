# Private Mode — VPS provisioning runbook

End-to-end steps to stand up the self-hosted AmneziaWG endpoint that Private
Mode connects to. You run these on a VPS you control; nothing here is automated
by the app. Personal VPN use is legal in Turkey.

**Goal of this phase (4):** a hardened AmneziaWG server outside Turkey + a
client `.conf` (with QUIC obfuscation) that the **official Amnezia client** on
Windows can connect through, proven by an external-IP check. App integration is
Phase 5 — do not touch app code until the raw tunnel works.

---

## 1. Pick a provider + region (outside Turkey)

Optimise for: (a) **not Hetzner**, (b) low latency to Turkey, (c) a provider
that isn't itself a well-known VPN farm on ISP blocklists.

> **Hetzner caveat:** Turkish ISPs have, at times, throttled or AS-level
> blocked Hetzner ranges (AS24940) because it's such a common VPN origin.
> Hetzner is cheap and fast *when it works*, but it's a gamble for TR. Prefer a
> different AS as the safe default.

| Pick | Provider / region | Why |
|---|---|---|
| **Safe default** | **Netcup** — Nuremberg (DE) or Vienna (AT) | Non-Hetzner, own AS, strong EU routing, cheap KVM. |
| **Lowest latency** | A **Bulgaria (Sofia)** host, e.g. Neterra/AEZA | Geographically closest good-transit EU country to TR. |
| **Alternates** | Netherlands (Amsterdam) or Romania (Bucharest) KVM VPS | Good routes, diverse ASNs. |
| Avoid for TR | Hetzner (AS block risk), oversold budget hosts | Throttling / packet loss. |

Spec: smallest KVM (1 vCPU / 1–2 GB RAM) is plenty for one user. **Ubuntu 24.04
LTS** (or Debian 12). Note the **public IPv4** — it's the tunnel endpoint.

Add your SSH public key at creation time (or `ssh-copy-id` immediately after).

---

## 2. First login + base update

```bash
ssh root@<VPS_IP>
apt update && apt full-upgrade -y && reboot     # reconnect after reboot
```

---

## 3. Install AmneziaWG (`amneziawg-installer`)

Use the community AmneziaWG installer (an AmneziaWG-adapted fork of
`angristan/wireguard-install`). It builds the kernel module via **DKMS** —
**this can trigger a reboot / take a few minutes; the script resumes and
survives it.** Let it finish; don't interrupt DKMS.

> **Security (match this repo's provenance discipline):** do **not** blind-pipe
> `curl | bash`. Download, read, then run.

```bash
# Fetch the installer (pick the maintained AmneziaWG fork you trust), e.g.:
curl -fsSL -o awg-install.sh https://raw.githubusercontent.com/<maintained-awg-installer>/main/install.sh
less awg-install.sh            # actually read it
chmod +x awg-install.sh
./awg-install.sh
```

The installer prompts for:

- **Public IPv4 / endpoint** — accept the detected VPS IP.
- **Port (UDP)** — accept the random high port or set one. **Write it down**
  (this is `AWG_PORT`, needed for the firewall). Using **443/udp** can help it
  blend with QUIC, but any high port is fine.
- **DNS for clients** — `1.1.1.1` (Cloudflare) is a good default.
- **First client name** — e.g. `windows-laptop`.

On finish it generates the server config and a **first client `.conf`** (often
under `/root/` or `~`, and/or shown as a QR). It also enables IP forwarding and
NAT (MASQUERADE) for the VPN subnet.

Verify the server is up:

```bash
sudo awg show                        # shows the awg0 interface + listen port
sudo systemctl status awg-quick@awg0 --no-pager
awg --version                        # note 1.0 vs 1.5+ (drives obfuscation set)
```

---

## 4. Harden the box

Copy `server/harden.sh` from this repo to the VPS and run it. It sets UFW to
**deny-all except SSH + the AmneziaWG UDP port**, installs **Fail2Ban**, and
switches SSH to **key-only** (with a guard that won't lock you out if no key is
present).

```bash
# from your workstation:
scp server/harden.sh root@<VPS_IP>:/root/

# on the VPS:
chmod +x /root/harden.sh
./harden.sh --awg-port <AWG_PORT>        # --ssh-port <N> if you changed SSH
```

> UFW defaults the FORWARD chain to DROP, which would silently kill the VPN's
> internet access — the script sets `DEFAULT_FORWARD_POLICY=ACCEPT` so routed
> peer traffic still gets NAT'd out. See comments in the script.

**Before logging out:** open a *second* SSH session with your key to confirm
key-only login works. Then verify the firewall:

```bash
sudo ufw status verbose               # only SSH/tcp + AWG/udp allowed
sudo fail2ban-client status sshd
```

---

## 5. Generate / export the client peer

The installer already made one client in step 3. To add more, re-run the
installer and choose "add a new client" (it appends a `[Peer]` to the server
and emits a fresh client `.conf` with new keys + PresharedKey).

The exported client `.conf` already contains the server's obfuscation params
(`Jc/Jmin/Jmax/S1/S2/H1–H4`) plus this client's keys.

**Add the QUIC obfuscation** (installers don't set `I1`): follow
[OBFUSCATION.md](./OBFUSCATION.md) to add the `I1` line (and `S3/S4` if 1.5+)
to **both** the server `[Interface]` and the client `[Interface]`, then
`sudo systemctl restart awg-quick@awg0`. Cross-check the client against
[client.conf.template](./client.conf.template).

---

## 6. Store the client config securely (never commit it)

Pull the client config to your workstation over SSH and store it under the
git-ignored `configs/` directory:

```bash
# from your workstation (repo root):
mkdir -p configs
scp root@<VPS_IP>:/root/windows-laptop.conf configs/private-mode.conf
```

`configs/`, `*.conf`, and key files are already in [.gitignore](../../.gitignore)
— confirm the secret is invisible to git before doing anything else:

```bash
git status --porcelain configs/       # must print NOTHING
git check-ignore -v configs/private-mode.conf   # must show it's ignored
```

Never paste real keys into commits, issues, or chats. If a private key or PSK
leaks, rotate it (regenerate the peer on the server).

---

## 7. Verify raw connectivity

Proceed to [VERIFICATION.md](./VERIFICATION.md): import `configs/private-mode.conf`
into the **official Amnezia client** on Windows, connect, and confirm an
external IP check shows the **VPS IP**. That green check is the "Done when" for
this phase.
