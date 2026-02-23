package network

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/amalg/go-bomberman/internal/game"
)

// MsgType identifies the type of network message.
type MsgType string

const (
	MsgJoin    MsgType = "join"
	MsgWelcome MsgType = "welcome"
	MsgAction  MsgType = "action"
	MsgState   MsgType = "state"
	MsgError   MsgType = "error"
	MsgStart   MsgType = "start"
)

// Envelope wraps all messages with a type discriminator for deserialization.
type Envelope struct {
	Type    MsgType         `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// --- Client → Server Messages ---

// JoinMsg is sent by a client to join the game.
type JoinMsg struct {
	Name string `json:"name"`
}

// ActionMsg is sent by a client to perform an action.
type ActionMsg struct {
	ActionType game.ActionType `json:"action_type"`
	Direction  game.Direction  `json:"direction,omitempty"`
}

// --- Server → Client Messages ---

// WelcomeMsg is sent to a client after joining.
type WelcomeMsg struct {
	PlayerID string          `json:"player_id"`
	Config   game.GameConfig `json:"config"`
}

// StateMsg is the full game state broadcast to all clients.
type StateMsg struct {
	State game.GameState `json:"state"`
}

// ErrorMsg notifies a client of an error.
type ErrorMsg struct {
	Message string `json:"message"`
}

// Encode serializes a message and writes it to the writer.
// Format: [4-byte big-endian length][JSON body]
func Encode(w io.Writer, msgType MsgType, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	env := Envelope{
		Type:    msgType,
		Payload: json.RawMessage(payloadBytes),
	}

	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	// Write 4-byte length header
	length := uint32(len(body))
	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return fmt.Errorf("write length: %w", err)
	}

	// Write body
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("write body: %w", err)
	}

	return nil
}

// Decode reads a length-prefixed JSON message from the reader.
func Decode(r io.Reader) (*Envelope, error) {
	// Read 4-byte length header
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("read length: %w", err)
	}

	// Sanity check on message size (max 1MB)
	if length > 1<<20 {
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}

	// Read body
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}

	return &env, nil
}

// DecodePayload unmarshals the payload from an envelope into the target struct.
func DecodePayload(env *Envelope, target interface{}) error {
	return json.Unmarshal(env.Payload, target)
}
