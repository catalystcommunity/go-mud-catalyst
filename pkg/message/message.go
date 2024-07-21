package message

import (
	"log"

	"github.com/fxamacker/cbor/v2"
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
		log.Fatal("Encoding options somehow failed for cbor: ", err)
	}
}

// Messages are constrained to be an int64 and can be mapped to an enum or whatnot,
// but is designed for fast switching of handlers
// Contents can be anything, but we provide helpers for CBOR messages, dual layered
type Message struct {
	Type     int64
	Contents []byte
}

type MessageValidator func(msg *Message) bool

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
