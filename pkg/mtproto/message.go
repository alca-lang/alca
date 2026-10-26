// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package mtproto

import (
	"crypto/aes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// Direction selects which half of the authorization key protects a message.
type Direction int

const (
	// ToServer is the direction of a message the client sends.
	ToServer Direction = 0
	// ToClient is the direction of a message the server sends.
	ToClient Direction = 8
)

// Message is an encrypted protocol message.
type Message struct {
	// AuthKeyID identifies the key the message is encrypted with. It is zero
	// for an unencrypted message.
	AuthKeyID [8]byte
	// MsgID is the identifier of the message.
	MsgID int64
	// MsgKey is the message key: the lower 128 bits of the hash of the key
	// part and the plain text.
	MsgKey [16]byte
	// Data is the encrypted body.
	Data []byte
}

// PlainMessage is the plain form of a message: the header of the session and
// the payload the application sent.
type PlainMessage struct {
	// Salt is the salt of the session.
	Salt int64
	// SessionID identifies the session.
	SessionID int64
	// MsgID is the identifier of the message.
	MsgID int64
	// SeqNo is the sequence number of the message.
	SeqNo int32
	// Payload is the body of the message.
	Payload []byte
}

// NewMsgID returns a message identifier for a payload of the given length.
// The identifier carries the time, the length and a random part, and the
// client side of the protocol requires it to be divisible by four.
func NewMsgID(payloadLen int, now time.Time) int64 {
	var randomPart [2]byte
	if _, err := rand.Read(randomPart[:]); err != nil {
		randomPart = [2]byte{0, 0}
	}
	id := (now.Unix() << 32) | (int64(payloadLen) << 16) | int64(binary.LittleEndian.Uint16(randomPart[:]))
	return id &^ 3
}

// ErrMessageFormat is returned when a message does not have the expected
// layout.
var ErrMessageFormat = errors.New("mtproto: the message has an unexpected format")

// EncryptMessage encrypts a plain message with an authorization key.
//
// The key is used from the offset that belongs to the direction: the client
// uses the bytes 88 to 119, the server the bytes 96 to 127. The message key is
// the lower 128 bits of the SHA-256 hash of that key part and the plain text,
// and it is the first half of the initialization vector.
func EncryptMessage(key AuthKey, plain PlainMessage) (Message, error) {
	if len(key.Value) != AuthKeySize {
		return Message{}, fmt.Errorf("mtproto: the authorization key must be %d bytes", AuthKeySize)
	}
	header, err := EncodeHeader(plain)
	if err != nil {
		return Message{}, err
	}
	body := append(append([]byte{}, header...), plain.Payload...)

	// The padding is at least 12 bytes and makes the message a multiple of the
	// block size of the cipher.
	paddingSize := 12
	if remainder := (len(body) + paddingSize) % aes.BlockSize; remainder != 0 {
		paddingSize += aes.BlockSize - remainder
	}
	padding := make([]byte, paddingSize)
	if _, err := rand.Read(padding); err != nil {
		return Message{}, err
	}
	encrypted := append(body, padding...)

	keyPart := key.Value[88:120]
	hasher := sha256.New()
	hasher.Write(keyPart)
	hasher.Write(encrypted)
	sum := hasher.Sum(nil)
	var msgKey [16]byte
	copy(msgKey[:], sum[8:24])

	iv := make([]byte, 32)
	copy(iv[:16], msgKey[:])
	copy(iv[16:], key.Value[88:96])
	data, err := EncryptIGE(keyPart, encrypted, iv)
	if err != nil {
		return Message{}, err
	}
	return Message{AuthKeyID: key.ID, MsgID: plain.MsgID, MsgKey: msgKey, Data: data}, nil
}

// DecryptMessage decrypts a message with an authorization key and returns its
// header and payload.
func DecryptMessage(key AuthKey, direction Direction, msg Message) (PlainMessage, error) {
	if len(key.Value) != AuthKeySize {
		return PlainMessage{}, fmt.Errorf("mtproto: the authorization key must be %d bytes", AuthKeySize)
	}
	if direction != ToServer && direction != ToClient {
		return PlainMessage{}, errors.New("mtproto: the direction must be to the server or to the client")
	}
	keyPart := key.Value[88+int(direction) : 120+int(direction)]

	iv := make([]byte, 32)
	copy(iv[:16], msg.MsgKey[:])
	copy(iv[16:], key.Value[88+int(direction):96+int(direction)])
	plain, err := DecryptIGE(keyPart, msg.Data, iv)
	if err != nil {
		return PlainMessage{}, err
	}
	// The message key is checked against the plain text.
	hasher := sha256.New()
	hasher.Write(keyPart)
	hasher.Write(plain)
	sum := hasher.Sum(nil)
	if !equalBytes(sum[8:24], msg.MsgKey[:]) {
		return PlainMessage{}, errors.New("mtproto: the message key does not match the message")
	}
	// The padding is at least 12 bytes.
	if len(plain) < 12 {
		return PlainMessage{}, ErrMessageFormat
	}
	header, payload, err := DecodeHeader(plain)
	if err != nil {
		return PlainMessage{}, err
	}
	return PlainMessage{
		Salt:      header.Salt,
		SessionID: header.SessionID,
		MsgID:     header.MsgID,
		SeqNo:     header.SeqNo,
		Payload:   payload,
	}, nil
}

// Header is the plain header of a message: the state of the session.
type Header struct {
	Salt      int64
	SessionID int64
	MsgID     int64
	SeqNo     int32
}

// EncodeHeader serializes a message header: the salt, the session, the
// message identifier, the sequence number and the length of the payload.
func EncodeHeader(plain PlainMessage) ([]byte, error) {
	header := make([]byte, 32)
	binary.LittleEndian.PutUint64(header[0:8], uint64(plain.Salt))
	binary.LittleEndian.PutUint64(header[8:16], uint64(plain.SessionID))
	binary.LittleEndian.PutUint64(header[16:24], uint64(plain.MsgID))
	binary.LittleEndian.PutUint32(header[24:28], uint32(plain.SeqNo))
	binary.LittleEndian.PutUint32(header[28:32], uint32(len(plain.Payload)))
	return header, nil
}

// DecodeHeader reads a message header and the payload that follows it. The
// bytes after the payload are the padding of the message.
func DecodeHeader(data []byte) (Header, []byte, error) {
	if len(data) < 32 {
		return Header{}, nil, ErrMessageFormat
	}
	length := int(binary.LittleEndian.Uint32(data[28:32]))
	if length < 0 || 32+length > len(data) {
		return Header{}, nil, ErrMessageFormat
	}
	header := Header{
		Salt:      int64(binary.LittleEndian.Uint64(data[0:8])),
		SessionID: int64(binary.LittleEndian.Uint64(data[8:16])),
		MsgID:     int64(binary.LittleEndian.Uint64(data[16:24])),
		SeqNo:     int32(binary.LittleEndian.Uint32(data[24:28])),
	}
	return header, data[32 : 32+length], nil
}

// EncodeMessage writes a message in the wire format: the key identifier, the
// message identifier, the message key and the encrypted body.
func EncodeMessage(msg Message) []byte {
	out := make([]byte, 0, 32+len(msg.Data))
	out = append(out, msg.AuthKeyID[:]...)
	var id [8]byte
	binary.LittleEndian.PutUint64(id[:], uint64(msg.MsgID))
	out = append(out, id[:]...)
	out = append(out, msg.MsgKey[:]...)
	out = append(out, msg.Data...)
	return out
}

// DecodeMessage reads a message in the wire format.
func DecodeMessage(data []byte) (Message, error) {
	if len(data) < 32 {
		return Message{}, ErrMessageFormat
	}
	msg := Message{Data: append([]byte(nil), data[32:]...)}
	copy(msg.AuthKeyID[:], data[0:8])
	msg.MsgID = int64(binary.LittleEndian.Uint64(data[8:16]))
	copy(msg.MsgKey[:], data[16:32])
	return msg, nil
}
