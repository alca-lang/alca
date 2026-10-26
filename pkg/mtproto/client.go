// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package mtproto

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// The container types of the protocol.
const (
	// ContainerMessage carries the body of a request.
	ContainerMessage uint32 = 0x5f2ab7ee
	// ContainerRPCResult carries the result of a request.
	ContainerRPCResult uint32 = 0x3072cfa1
	// ContainerRPCError carries the error of a request.
	ContainerRPCError uint32 = 0x9760a3d2
	// ContainerGzipPacked carries a compressed body.
	ContainerGzipPacked uint32 = 0x73f1f8dc
)

// Endpoint is the address of a data center.
type Endpoint struct {
	// DC is the number of the data center.
	DC int
	// IP is the address of the data center.
	IP string
	// Port is the port of the data center.
	Port int
}

// Address returns the address of the endpoint.
func (e Endpoint) Address() string { return net.JoinHostPort(e.IP, fmt.Sprint(e.Port)) }

// KnownEndpoints returns the production data centers of the network. The
// addresses change over time, so a program can pass its own endpoint instead.
func KnownEndpoints() []Endpoint {
	return []Endpoint{
		{DC: 1, IP: "149.154.175.50", Port: 443},
		{DC: 2, IP: "149.154.167.51", Port: 443},
		{DC: 3, IP: "149.154.175.100", Port: 443},
		{DC: 4, IP: "149.154.167.91", Port: 443},
		{DC: 5, IP: "91.108.56.130", Port: 443},
	}
}

// ErrNoAuthKey is returned when a message needs an authorization key and the
// session has none.
var ErrNoAuthKey = errors.New("mtproto: the session has no authorization key")

// Client is a connection to a data center.
type Client struct {
	mu        sync.Mutex
	endpoint  Endpoint
	conn      net.Conn
	key       AuthKey
	hasKey    bool
	sessionID int64
	salt      int64
	seqNo     int32
	msgID     int64
}

// Connect opens a connection to a data center and sends the obfuscated header
// the transport expects.
func Connect(endpoint Endpoint, timeout time.Duration) (*Client, error) {
	if endpoint.IP == "" {
		return nil, errors.New("mtproto: the endpoint has no address")
	}
	conn, err := net.DialTimeout("tcp", endpoint.Address(), timeout)
	if err != nil {
		return nil, err
	}
	client := &Client{endpoint: endpoint, conn: conn}
	if err := client.sendHeader(); err != nil {
		conn.Close()
		return nil, err
	}
	return client, nil
}

// Endpoint returns the address of the data center the client is connected to.
func (c *Client) Endpoint() Endpoint { return c.endpoint }

// SetAuthKey installs the authorization key of the session.
func (c *Client) SetAuthKey(key AuthKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.key = key
	c.hasKey = true
}

// AuthKey returns the authorization key of the session.
func (c *Client) AuthKey() (AuthKey, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.key, c.hasKey
}

// Close closes the connection.
func (c *Client) Close() error { return c.conn.Close() }

// sendHeader sends the obfuscated header of the connection.
func (c *Client) sendHeader() error {
	salt, err := randomInt64()
	if err != nil {
		return err
	}
	sessionID, err := randomInt64()
	if err != nil {
		return err
	}
	header, err := NewObfuscatedHeader(TagIntermediate, salt, sessionID, 0, 0, 0)
	if err != nil {
		return err
	}
	encoded, err := header.Encode()
	if err != nil {
		return err
	}
	if _, err := c.conn.Write(encoded); err != nil {
		return err
	}
	c.salt = salt
	c.sessionID = sessionID
	return nil
}

// Send writes a request body to the server. The body is wrapped in the
// message container and encrypted with the authorization key of the session.
func (c *Client) Send(body []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendLocked(body)
}

// SendUnencrypted writes a request body without an authorization key, which
// the key exchange needs.
func (c *Client) SendUnencrypted(body []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendUnencryptedLocked(body)
}

// sendLocked writes an encrypted request.
func (c *Client) sendLocked(body []byte) error {
	if !c.hasKey {
		return ErrNoAuthKey
	}
	payload := make([]byte, 0, len(body)+8)
	payload = append(payload, byte(len(body)+4), byte((len(body)+4)>>8), byte((len(body)+4)>>16), byte((len(body)+4)>>24))
	payload = binary.LittleEndian.AppendUint32(payload, ContainerMessage)
	payload = append(payload, body...)

	plain := PlainMessage{
		Salt:      c.salt,
		SessionID: c.sessionID,
		MsgID:     c.nextMsgID(len(payload)),
		SeqNo:     c.seqNo,
		Payload:   payload,
	}
	c.seqNo++

	encrypted, err := EncryptMessage(c.key, plain)
	if err != nil {
		return err
	}
	return c.writeMessage(encrypted)
}

// sendUnencryptedLocked writes a request without an authorization key.
func (c *Client) sendUnencryptedLocked(body []byte) error {
	payload := make([]byte, 0, len(body)+8)
	payload = append(payload, byte(len(body)+4), byte((len(body)+4)>>8), byte((len(body)+4)>>16), byte((len(body)+4)>>24))
	payload = binary.LittleEndian.AppendUint32(payload, ContainerMessage)
	payload = append(payload, body...)

	msgID := c.nextMsgID(len(payload))
	c.seqNo++
	return c.writeMessage(Message{MsgID: msgID, Data: payload})
}

// writeMessage writes a message in the framing of the transport: the length of
// the message, then the message itself.
func (c *Client) writeMessage(msg Message) error {
	body := EncodeMessage(msg)
	var length [4]byte
	binary.LittleEndian.PutUint32(length[:], uint32(len(body)))
	if _, err := c.conn.Write(length[:]); err != nil {
		return err
	}
	_, err := c.conn.Write(body)
	return err
}

// Receive reads the next body from the server. The result of a request is
// returned as it is; an error of the server is returned as an error.
func (c *Client) Receive() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var length [4]byte
	if _, err := io.ReadFull(c.conn, length[:]); err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint32(length[:])
	if size > 1<<24 {
		return nil, fmt.Errorf("mtproto: the message of %d bytes is too large", size)
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(c.conn, body); err != nil {
		return nil, err
	}
	msg, err := DecodeMessage(body)
	if err != nil {
		return nil, err
	}
	var plain PlainMessage
	if c.hasKey && msg.AuthKeyID == c.key.ID {
		plain, err = DecryptMessage(c.key, ToClient, msg)
		if err != nil {
			return nil, err
		}
	} else {
		plain = PlainMessage{MsgID: msg.MsgID, Payload: msg.Data}
	}
	return parseResponseContainer(plain.Payload)
}

// parseResponseContainer reads the container of a response.
func parseResponseContainer(payload []byte) ([]byte, error) {
	reader := NewReader(payload)
	for {
		length, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		if length < 4 {
			return nil, ErrTLFormat
		}
		typeID, err := reader.ReadUInt32()
		if err != nil {
			return nil, err
		}
		body := reader.Remaining()[:length-4]
		switch typeID {
		case ContainerRPCResult:
			return body, nil
		case ContainerRPCError:
			code, err := NewReader(body).ReadInt32()
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("mtproto: the server returned the error %d", code)
		case ContainerGzipPacked:
			// The compressed body is read as it is; the caller decodes it.
			return body, nil
		default:
			reader.pos += int(length) - 4
		}
	}
}

// nextMsgID returns the identifier of the next message.
func (c *Client) nextMsgID(payloadLen int) int64 {
	if c.msgID == 0 {
		c.msgID = NewMsgID(payloadLen, time.Now())
		return c.msgID
	}
	c.msgID += 4
	return c.msgID
}

// randomInt64 returns a random 64-bit number.
func randomInt64() (int64, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(buf[:])), nil
}
