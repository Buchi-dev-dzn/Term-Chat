package protocol

import "time"

const (
	Version         = "v1"
	ServiceName     = "_termchat._tcp"
	FrameTypeHello  = "hello"
	FrameTypeJoined = "joined"
	FrameTypeLeft   = "left"
	FrameTypeMsg    = "message"
	FrameTypeError  = "error"
)

type Envelope struct {
	Version    string    `json:"version"`
	Room       string    `json:"room"`
	Sender     string    `json:"sender"`
	SentAt     time.Time `json:"sent_at"`
	Nonce      []byte    `json:"nonce"`
	Ciphertext []byte    `json:"ciphertext"`
}

type ControlFrame struct {
	Type    string `json:"type"`
	Room    string `json:"room"`
	Sender  string `json:"sender"`
	Payload []byte `json:"payload,omitempty"`
}

type ChatMessage struct {
	Body string `json:"body"`
}

type HelloPayload struct {
	Nickname string `json:"nickname"`
	Verifier []byte `json:"verifier"`
}

type RoomConfig struct {
	Name      string
	Protected bool
	Passcode  string
}
