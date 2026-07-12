# Private Mode — raw connectivity verification

**This is the "Done when" for Phase 4.** Prove the AmneziaWG tunnel works with
the **official Amnezia client** before any app code touches it. If it fails
here, it's a server/config problem, not an app problem — fix it now.

## 0. Baseline (tunnel OFF)

On the Windows machine, note your real Turkish IP so you can see it change:

- Open <https://api.ipify.org> (or `curl https://api.ipify.org` in PowerShell).
- Record the IP and, at <https://ipinfo.io>, the ISP/country (should be TR).

## 1. Import the config into the official Amnezia client

1. Install the **official AmneziaVPN** Windows client (amnezia.org) — this is
   the *reference* client for validation. (Our bundled `amneziawg.exe` engine is
   for Phase 5; don't use the app yet.)
2. Import `configs/private-mode.conf`:
   - AmneziaVPN → **Add connection / Import** → from file → pick the `.conf`, **or**
   - use the WireGuard-style import if you're testing with the AmneziaWG tunnel
     directly.
3. Confirm the imported connection shows **AmneziaWG** (not plain WireGuard) and
   that the obfuscation fields (`Jc … H4`, `I1`) are present in advanced view.

## 2. Connect

Click **Connect**. Within a few seconds it should reach **Connected**.

Quick server-side confirmation (optional, over SSH):

```bash
sudo awg show            # your peer shows a recent 'latest handshake' + transfer
```

A non-zero **latest handshake** and rising **transfer rx/tx** = the tunnel is up.

## 3. Confirm the exit IP is the VPS  ✅

With the tunnel connected, on Windows:

```powershell
curl https://api.ipify.org      # must print the VPS public IPv4
```

- Also load <https://ipinfo.io> → country/org should now be the **VPS's**
  (Germany/Bulgaria/…), not Turkey.
- **DNS-leak check:** <https://www.dnsleaktest.com> → resolvers should be the
  VPN's DNS (`1.1.1.1`), not your Turkish ISP.

If the external IP equals the **VPS IP**, Phase 4 is **done**. 🎉

## 4. Sanity: throughput & stability

- Run a quick speed test — AmneziaWG is same-crypto WireGuard, so expect near
  line-rate minus obfuscation overhead. Large drops usually mean **MTU**: lower
  the client `MTU` (try `1280`, then `1200`) and reconnect.
- Leave it connected a few minutes; `PersistentKeepalive = 25` should keep the
  session alive through NAT.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Handshake never completes (no `latest handshake`) | UDP port blocked, or obfuscation params differ between ends | Confirm `ufw` allows `AWG_PORT/udp`; ensure `Jc…H4`/`I1` are **identical** on server & client |
| Connects but no internet / DNS | FORWARD dropped or NAT missing | `harden.sh` sets `DEFAULT_FORWARD_POLICY=ACCEPT`; check installer's MASQUERADE rule and `net.ipv4.ip_forward=1` |
| Config rejected on import/restart | `S1+56 == S2`, or duplicate/`≤4` `H*`, or `I1` grammar mismatch | See the two hard rules in [OBFUSCATION.md](./OBFUSCATION.md); use the GUI's QUIC preset for `I1` |
| Works, but throttled after a while in TR | DPI still fingerprinting | Move `AWG_PORT` to `443/udp`, keep the QUIC `I1`; try a different provider AS |
| Locked out of SSH | password auth disabled without a working key | Use provider web console/recovery to re-add your key in `~/.ssh/authorized_keys` |

When it works, save the confirming detail (VPS IP shown by `api.ipify.org`) in
your own notes — **not** in git — and hand off to Phase 5 (app integration).
