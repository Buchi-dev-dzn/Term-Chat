# termchat

`termchat` is a single-command LAN chat for the terminal.

Start the app, choose `Host` or `Join`, and complete the rest inside the interactive setup flow. No config file, no required CLI flags, no external server.

## Features

- Single public entrypoint: `termchat`
- Interactive setup for hosting and joining
- LAN room discovery via mDNS (`_termchat._tcp`)
- TCP host-relay model
- Room list includes both open and password-protected rooms
- App-layer message encryption with `Argon2id` + `XChaCha20-Poly1305`
- In-memory rooms and in-memory message flow only
- Cross-platform target: macOS, Linux, Windows

## Current Model

`termchat` is designed for people on the same local network.

- A host creates a room and relays encrypted messages
- Joiners discover rooms on the LAN, or fall back to manual `IP:port`
- Password-protected rooms require the passcode during setup
- Chat history is not persisted

## Install

### From source

Requires Go `1.26+`.

```bash
git clone https://github.com/your-name/termchat.git
cd termchat
go build -o termchat ./cmd/termchat
```

## Run

```bash
./termchat
```

The app will guide you through setup:

1. Choose `Host a room` or `Join a room`
2. Enter your nickname
3. If hosting, set the public room name and whether the room is password-protected
4. If joining, pick a discovered LAN room or enter `IP:port` manually
5. Enter the room passcode when required
6. Start chatting

When hosting, `termchat` shows the room name, local IP, and port after startup.

## Security Notes

- Message bodies are encrypted end-to-end at the application layer
- The host relays ciphertext, not plaintext message bodies
- Discovery metadata is public on the LAN by design
- This project does not currently provide strong participant identity verification

## Limitations

- LAN-only discovery
- No persistence
- No file transfer
- No direct messages
- No non-interactive CLI mode yet
- The current interactive host flow starts one room per process

## Development

Run tests:

```bash
go test ./...
```

Run without building:

```bash
go run ./cmd/termchat
```

## Project Layout

```text
cmd/termchat         application entrypoint
internal/tui         interactive setup and chat views
internal/discovery   mDNS advertisement and browsing
internal/protocol    TCP transport, framing, rooms, encryption
```

## Roadmap

- Better multi-room host management in one process
- Stronger UX around join failures and protected rooms
- Improved terminal UI beyond the current line-based flow

## License

Licensed under the terms in [LICENSE](./LICENSE).
