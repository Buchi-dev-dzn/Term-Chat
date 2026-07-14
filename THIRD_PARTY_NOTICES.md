# Third-Party Notices

This project uses third-party open source software.

## Runtime and UI dependencies

| Dependency | Purpose | License |
| --- | --- | --- |
| `github.com/charmbracelet/bubbletea` | full-screen terminal UI runtime | MIT |
| `github.com/charmbracelet/bubbles` | text input and TUI components | MIT |
| `github.com/charmbracelet/lipgloss` | terminal layout and styling | MIT |
| `github.com/grandcat/zeroconf` | mDNS-based LAN room discovery | MIT |
| `golang.org/x/crypto` | Argon2id and XChaCha20-Poly1305 | BSD-3-Clause |

## Transitive dependencies used by shipped features

| Dependency | Used through | License |
| --- | --- | --- |
| `github.com/miekg/dns` | `github.com/grandcat/zeroconf` | BSD-3-Clause |
| `golang.org/x/net` | `github.com/grandcat/zeroconf` | BSD-3-Clause |
| `golang.org/x/sys` | terminal and networking dependencies | BSD-3-Clause |
| `golang.org/x/text` | terminal UI dependencies | BSD-3-Clause |

## Source references

License texts are available from each upstream project. The versions currently referenced by this repository are listed in [go.mod](./go.mod) and [go.sum](./go.sum).

For local development, the downloaded module cache used during setup contained these license files:

- `github.com/charmbracelet/bubbletea@v1.3.10/LICENSE`
- `github.com/charmbracelet/bubbles@v1.0.0/LICENSE`
- `github.com/charmbracelet/lipgloss@v1.1.0/LICENSE`
- `github.com/grandcat/zeroconf@v1.0.0/LICENSE`
- `github.com/miekg/dns@v1.1.27/LICENSE`
- `golang.org/x/crypto@v0.43.0/LICENSE`
- `golang.org/x/net@v0.45.0/LICENSE`
- `golang.org/x/sys@v0.38.0/LICENSE`
- `golang.org/x/text@v0.30.0/LICENSE`
