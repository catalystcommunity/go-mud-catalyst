package message

import (
	"fmt"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/fxamacker/cbor/v2"
)

// Message type constants for common message types
const (
	// System messages
	MessageTypeHeartbeat int64 = iota
	MessageTypeConnect
	MessageTypeDisconnect
	MessageTypeError
	MessageTypeAck
	MessageTypeNack
	
	// Authentication messages
	MessageTypeAuth
	MessageTypeAuthSuccess
	MessageTypeAuthFailure
	
	// Chat messages
	MessageTypeChat
	MessageTypeWhisper
	MessageTypeBroadcast
	MessageTypeJoinRoom
	MessageTypeLeaveRoom
	
	// Game messages
	MessageTypeMove
	MessageTypeAction
	MessageTypeStatus
	MessageTypeInventory
	
	// Custom messages start here
	MessageTypeCustom = 1000
)

var (
	em cbor.EncMode
)

func init() {
	opts := cbor.CoreDetEncOptions()
	opts.Time = cbor.TimeUnix
	var err error
	em, err = opts.EncMode()
	if err != nil {
		logging.Fatal("Encoding options failed for cbor", "error", err)
	}
}

// Messages are constrained to be an int64 and can be mapped to an enum or whatnot,
// but is designed for fast switching of handlers
// Contents can be anything, but we provide helpers for CBOR messages, dual layered
type Message struct {
	Type         int64
	Contents     []byte
	ID           string        // Unique message ID for acknowledgment tracking
	RequiresAck  bool         // Whether this message requires acknowledgment
	AckTimeout   time.Duration // How long to wait for acknowledgment before retry/fail
}

// AckMessage represents an acknowledgment message
type AckMessage struct {
	MessageID string
	Success   bool
	Error     string // Optional error message for NACK
}

// PendingAck tracks messages waiting for acknowledgment
type PendingAck struct {
	Message   *Message
	SentAt    time.Time
	RetryCount int
}

type MessageValidator func(msg *Message) bool
type MessageHandler func(msg *Message) error

// MessageRouter handles routing messages to appropriate handlers
type MessageRouter struct {
	handlers   map[int64]MessageHandler
	validators map[int64]MessageValidator
}

// NewMessageRouter creates a new message router
func NewMessageRouter() *MessageRouter {
	return &MessageRouter{
		handlers:   make(map[int64]MessageHandler),
		validators: make(map[int64]MessageValidator),
	}
}

// RegisterHandler registers a handler for a specific message type
func (r *MessageRouter) RegisterHandler(messageType int64, handler MessageHandler) {
	r.handlers[messageType] = handler
}

// RegisterValidator registers a validator for a specific message type
func (r *MessageRouter) RegisterValidator(messageType int64, validator MessageValidator) {
	r.validators[messageType] = validator
}

// RouteMessage routes a message to the appropriate handler
func (r *MessageRouter) RouteMessage(msg *Message) error {
	// First validate the message if a validator is registered
	if validator, exists := r.validators[msg.Type]; exists {
		if !validator(msg) {
			return &MessageError{Type: msg.Type, Err: "message validation failed"}
		}
	}
	
	// Route to handler if one is registered
	if handler, exists := r.handlers[msg.Type]; exists {
		return handler(msg)
	}
	
	return &MessageError{Type: msg.Type, Err: "no handler registered for message type"}
}

// MessageError represents an error in message processing
type MessageError struct {
	Type int64
	Err  string
}

func (e *MessageError) Error() string {
	return e.Err
}

func MessageUnwrap(raw []byte) (*Message, error) {
	msg := &Message{}
	err := cbor.Unmarshal(raw, msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func MessageWrap(msg *Message) ([]byte, error) {
	b, err := em.Marshal(msg)
	if err != nil {
		return []byte{}, err
	}
	return b, nil
}

// CreateAckMessage creates an acknowledgment message for the given message ID
func CreateAckMessage(messageID string, success bool, errorMsg string) *Message {
	ackData := AckMessage{
		MessageID: messageID,
		Success:   success,
		Error:     errorMsg,
	}
	
	contents, err := em.Marshal(ackData)
	if err != nil {
		logging.Error("Failed to marshal ack message", "error", err)
		return nil
	}
	
	msgType := MessageTypeAck
	if !success {
		msgType = MessageTypeNack
	}
	
	return &Message{
		Type:        msgType,
		Contents:    contents,
		RequiresAck: false, // Acks themselves don't require acks
	}
}

// ParseAckMessage parses the contents of an acknowledgment message
func ParseAckMessage(msg *Message) (*AckMessage, error) {
	if msg.Type != MessageTypeAck && msg.Type != MessageTypeNack {
		return nil, &MessageError{Type: msg.Type, Err: "message is not an acknowledgment"}
	}
	
	var ackData AckMessage
	err := cbor.Unmarshal(msg.Contents, &ackData)
	if err != nil {
		return nil, err
	}
	
	return &ackData, nil
}

// RequiresAcknowledgment returns true if the given message type typically requires acknowledgment
func RequiresAcknowledgment(messageType int64) bool {
	switch messageType {
	case MessageTypeAuth, MessageTypeAuthSuccess, MessageTypeAuthFailure:
		return true
	case MessageTypeMove, MessageTypeAction, MessageTypeInventory:
		return true
	case MessageTypeJoinRoom, MessageTypeLeaveRoom:
		return true
	case MessageTypeHeartbeat, MessageTypeChat, MessageTypeWhisper, MessageTypeBroadcast:
		return false
	default:
		return false // Custom messages can set RequiresAck explicitly
	}
}

// MessageTypeToString returns a human-readable string for a message type
func MessageTypeToString(messageType int64) string {
	switch messageType {
	case MessageTypeHeartbeat:
		return "HEARTBEAT"
	case MessageTypeConnect:
		return "CONNECT"
	case MessageTypeDisconnect:
		return "DISCONNECT"
	case MessageTypeError:
		return "ERROR"
	case MessageTypeAck:
		return "ACK"
	case MessageTypeNack:
		return "NACK"
	case MessageTypeAuth:
		return "AUTH"
	case MessageTypeAuthSuccess:
		return "AUTH_SUCCESS"
	case MessageTypeAuthFailure:
		return "AUTH_FAILURE"
	case MessageTypeChat:
		return "CHAT"
	case MessageTypeWhisper:
		return "WHISPER"
	case MessageTypeBroadcast:
		return "BROADCAST"
	case MessageTypeJoinRoom:
		return "JOIN_ROOM"
	case MessageTypeLeaveRoom:
		return "LEAVE_ROOM"
	case MessageTypeMove:
		return "MOVE"
	case MessageTypeAction:
		return "ACTION"
	case MessageTypeStatus:
		return "STATUS"
	case MessageTypeInventory:
		return "INVENTORY"
	default:
		if messageType >= MessageTypeCustom {
			return fmt.Sprintf("CUSTOM_%d", messageType)
		}
		return fmt.Sprintf("UNKNOWN_%d", messageType)
	}
}
