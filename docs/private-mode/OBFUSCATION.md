# AmneziaWG Obfuscation — Turkey QUIC preset

AmneziaWG is WireGuard with the **same crypto** plus a wire-format obfuscation
layer that hides the WireGuard signature from DPI. Turkey is a *lighter* DPI
zone (SNI/protocol-fingerprint blocking, not deep active probing), so this
obfuscation is sufficient on its own — no Xray/REALITY wrapper is needed.

These parameters live in the `[Interface]` section of **both** the server and
the client config. They are **not secrets** and they are **not per-peer** — they
are the obfuscation protocol itself and must be **byte-for-byte identical on both
ends**, or the handshake never completes. (Keys and the PresharedKey *are*
per-peer secrets; the params below are not.)

---

## Version compatibility (read this first)

There are two generations of AmneziaWG parameters:

| Generation | Params | Supported by |
|---|---|---|
| **AWG 1.0** | `Jc Jmin Jmax S1 S2 H1 H2 H3 H4` | all AmneziaWG builds |
| **AWG 1.5+** | adds `S3 S4` and the signature packets `I1`–`I5` | amneziawg-go/tools **1.5+ / 2.x** |

Our Windows client is **amneziawg-go 2.0.1** (see [ENGINES.md](../../ENGINES.md)),
which supports the 1.5+ set — so the QUIC `I1` mimic below is usable **only if
the server's AmneziaWG is also 1.5+**. Confirm on the VPS after install:

```bash
awg --version        # or: modinfo amneziawg | grep -i version
```

- Server is **1.5+** → use the full preset including `I1` (recommended for TR).
- Server is **1.0** → drop `S3 S4 I1`; keep `Jc Jmin Jmax S1 S2 H1–H4`. Still
  defeats plain-WireGuard fingerprinting, just without the QUIC decoy.

Whatever set you use, the client config must contain the **same** lines.

---

## Parameter reference

| Param | Meaning | Value / rule | Notes |
|---|---|---|---|
| `Jc` | Junk packet **count** sent before the handshake | 3–10 (use **4**) | more = more cover traffic + overhead |
| `Jmin` | Junk packet **min** size (bytes) | `Jmin < Jmax` | |
| `Jmax` | Junk packet **max** size (bytes) | `≤ 1280` | keep under path MTU |
| `S1` | Junk **prepended to handshake INIT** | small, e.g. 15–150 | **`S1 + 56 ≠ S2`** (see below) |
| `S2` | Junk **prepended to handshake RESPONSE** | small, e.g. 15–150 | **`S1 + 56 ≠ S2`** |
| `S3` | (1.5+) Junk for the cookie/underload packet | small | omit on AWG 1.0 |
| `S4` | (1.5+) Junk for the transport packet | small | omit on AWG 1.0 |
| `H1` | Magic header replacing msg type **1** (INIT) | distinct uint32 | see H-rule below |
| `H2` | Magic header replacing msg type **2** (RESPONSE) | distinct uint32 | |
| `H3` | Magic header replacing msg type **3** (COOKIE) | distinct uint32 | |
| `H4` | Magic header replacing msg type **4** (TRANSPORT) | distinct uint32 | |
| `I1`–`I5` | (1.5+) **Signature/decoy packets** sent at connect time to imitate another protocol | tag grammar | `I1` = our QUIC decoy |

**The two hard validation rules** (AmneziaWG rejects configs that break them):

1. **`S1 + 56 ≠ S2`.** WireGuard's INIT is 148 B and RESPONSE is 92 B
   (148 − 92 = 56). If `S1 + 56 == S2`, the obfuscated INIT and RESPONSE end up
   the same length — a fingerprint. Keep them unequal.
2. **`H1, H2, H3, H4` must be four *distinct* values, each `> 4`** (1–4 are the
   real WireGuard message types). Use large random 32-bit numbers. Keep them
   within signed-int32 range (`5 … 2147483647`) for maximum tooling
   compatibility.

`amneziawg-installer` **auto-generates valid** `Jc Jmin Jmax S1 S2 H1–H4` for
you. Read them off the server rather than inventing them:

```bash
sudo awg showconf awg0     # prints the live [Interface] incl. jc/jmin/.../h4
# or:
sudo grep -E '^(Jc|Jmin|Jmax|S1|S2|S3|S4|H1|H2|H3|H4|I1)' /etc/amnezia/amneziawg/awg0.conf
```

Copy those exact values into the client config. Then **add** the QUIC `I1`
below to **both** ends (installers don't set `I1`), and restart the interface.

---

## The QUIC (`I1`) decoy — why and what

Turkish ISPs pass **QUIC/HTTP-3 (UDP/443)** freely — it's ordinary browser
traffic. A signature packet that looks like a **QUIC Initial** at connection
time makes the flow's opening bytes classify as QUIC, so the DPI never reaches
"unknown UDP → throttle". `I1` is sent once when the tunnel connects.

### Signature-packet grammar (amneziawg-tools 1.5+)

A signature packet is a string of tags:

| Tag | Emits |
|---|---|
| `<b 0x..>` | literal **bytes** (hex) |
| `<r N>` | `N` **random** bytes |
| `<c>` | a rolling **counter** |
| `<t>` | a **timestamp** |

### Our QUIC Initial `I1` (annotated)

```
I1 = <b 0xc300000001><b 0x08><r 8><b 0x08><r 8><b 0x00><b 0x449e><r 118>
```

Byte-by-byte, this reproduces the header of a **QUIC v1 Initial (long header)**:

| Bytes | QUIC field | Value |
|---|---|---|
| `c3` | first byte: long header, fixed bit, Initial type, 4-byte PN len | `0xC3` |
| `00000001` | **Version** | QUIC v1 |
| `08` | Destination Connection ID **length** | 8 |
| `<r 8>` | Destination Connection ID | random (per-connection, like a real client) |
| `08` | Source Connection ID **length** | 8 |
| `<r 8>` | Source Connection ID | random |
| `00` | Token length (varint) | 0 (client Initial has no token) |
| `449e` | Payload **length** (2-byte varint) | ~0x049e |
| `<r 118>` | protected payload + padding | random |

Total ≈ 158 bytes. Real Chrome/Firefox pad the first Initial to ~1200 B; if a
particular ISP fingerprints on that, raise the tail to `<r 1150>`. Keep `I1`
**identical on server and client**.

> Two ways to obtain a QUIC preset, in order of confidence:
>
> 1. **Preferred:** open the official **Amnezia GUI** client, pick the
>    protocol-mimic masking preset, and copy its `I1…I5` lines verbatim into
>    both configs. This is guaranteed valid for the installed tools version.
> 2. **Manual:** use the annotated `I1` above. If your `awg`/`amneziawg-tools`
>    version rejects it, its grammar has drifted — fall back to (1). Verify the
>    current grammar in the amneziawg-tools README before hand-authoring.

---

## Final `[Interface]` obfuscation block (example placeholders)

Put this on the **server** (`awg0.conf`) and the **same** lines in the client
`[Interface]`. Replace the numeric values with the server's actual
installer-generated ones; `H*` shown are illustrative distinct values.

```ini
Jc   = 4
Jmin = 40
Jmax = 70
S1   = 86
S2   = 574
S3   = 0
S4   = 0
H1   = 1148643509
H2   = 1637895918
H3   = 2059136377
H4   = 1051215432
I1   = <b 0xc300000001><b 0x08><r 8><b 0x08><r 8><b 0x00><b 0x449e><r 118>
```

> `86 + 56 = 142 ≠ 574` ✓ (satisfies rule 1). `H1–H4` are four distinct
> in-range values ✓ (rule 2). These are **example** numbers — use the server's
> own generated `H*`/`S*`, then mirror them to the client.

After editing the server config, apply and confirm no validation error:

```bash
sudo systemctl restart awg-quick@awg0
sudo awg show awg0            # interface should be up, listening
```
