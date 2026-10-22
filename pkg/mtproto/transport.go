package mtproto

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
)

// TransportTag selects the framing of a transport connection.
type TransportTag uint32

const (
	// TagIntermediate is the framing of a full protocol message: the header
	// is encrypted with the key of the session.
	TagIntermediate TransportTag = 0xefefefef
	// TagAbridged is the framing of a short message.
	TagAbridged TransportTag = 0xeeeeeeee
	// TagFull is the framing of a message that is not compressed.
	TagFull TransportTag = 0xdddddddd
)

// ObfuscatedHeaderSize is the size of the obfuscated header of a transport
// connection.
const ObfuscatedHeaderSize = 64

// obfuscatedHeaderSize is the size of the masked part of the header: the salt,
// the session, the message identifier, the sequence number, the length and
// twelve random bytes. Together with the four byte tag and the sixteen byte
// initialization vector it fills the sixty-four bytes of the header.
const obfuscatedHeaderSize = 44

// ObfuscatedHeader is the first bytes of a transport connection.
type ObfuscatedHeader struct {
	// Tag is the framing of the connection.
	Tag TransportTag
	// Salt is the salt of the session.
	Salt int64
	// SessionID identifies the session.
	SessionID int64
	// MsgID is the identifier of the first message.
	MsgID int64
	// SeqNo is the sequence number of the first message.
	SeqNo int32
	// Length is the length of the first message.
	Length int32
	// InitVector is the initialization vector of the connection.
	InitVector []byte
}

// NewObfuscatedHeader creates the obfuscated header of a connection.
func NewObfuscatedHeader(tag TransportTag, salt, sessionID, msgID int64, seqNo, length int32) (ObfuscatedHeader, error) {
	iv := make([]byte, 16)
	if _, err := rand.Read(iv); err != nil {
		return ObfuscatedHeader{}, err
	}
	return ObfuscatedHeader{
		Tag:        tag,
		Salt:       salt,
		SessionID:  sessionID,
		MsgID:      msgID,
		SeqNo:      seqNo,
		Length:     length,
		InitVector: iv,
	}, nil
}

// Encode writes the header in the obfuscated form the transport expects: the
// tag, the masked header and the initialization vector.
//
// The header of the connection is masked with the initialization vector, read
// from its end, so that the vector itself stays readable by the receiver.
func (h ObfuscatedHeader) Encode() ([]byte, error) {
	if len(h.InitVector) != 16 {
		return nil, errors.New("mtproto: the initialization vector must be 16 bytes")
	}
	out := make([]byte, ObfuscatedHeaderSize)
	binary.LittleEndian.PutUint32(out[0:4], uint32(h.Tag))

	body := make([]byte, obfuscatedHeaderSize)
	binary.LittleEndian.PutUint64(body[0:8], uint64(h.Salt))
	binary.LittleEndian.PutUint64(body[8:16], uint64(h.SessionID))
	binary.LittleEndian.PutUint64(body[16:24], uint64(h.MsgID))
	binary.LittleEndian.PutUint32(body[24:28], uint32(h.SeqNo))
	binary.LittleEndian.PutUint32(body[28:32], uint32(h.Length))
	// The last twelve bytes stay random.

	// The body is masked with the initialization vector, read backwards.
	for i := 0; i < obfuscatedHeaderSize; i++ {
		out[4+i] = body[i] ^ h.InitVector[15-(i%16)]
	}
	// The initialization vector is the last sixteen bytes of the header.
	copy(out[4+obfuscatedHeaderSize:], h.InitVector)
	return out, nil
}

// DecodeObfuscatedHeader reads the obfuscated header of a connection.
func DecodeObfuscatedHeader(data []byte) (ObfuscatedHeader, error) {
	if len(data) < ObfuscatedHeaderSize {
		return ObfuscatedHeader{}, fmt.Errorf("mtproto: the obfuscated header must be %d bytes", ObfuscatedHeaderSize)
	}
	tag := TransportTag(binary.LittleEndian.Uint32(data[0:4]))
	iv := make([]byte, 16)
	copy(iv, data[4+obfuscatedHeaderSize:4+obfuscatedHeaderSize+16])
	body := make([]byte, obfuscatedHeaderSize)
	for i := 0; i < obfuscatedHeaderSize; i++ {
		body[i] = data[4+i] ^ iv[15-(i%16)]
	}
	header := ObfuscatedHeader{
		Tag:        tag,
		Salt:       int64(binary.LittleEndian.Uint64(body[0:8])),
		SessionID:  int64(binary.LittleEndian.Uint64(body[8:16])),
		MsgID:      int64(binary.LittleEndian.Uint64(body[16:24])),
		SeqNo:      int32(binary.LittleEndian.Uint32(body[24:28])),
		Length:     int32(binary.LittleEndian.Uint32(body[28:32])),
		InitVector: iv,
	}
	return header, nil
}
