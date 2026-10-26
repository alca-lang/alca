// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package stdlib provides the MTProto module for the Alca programming
// language.
//
// The module exposes the protocol of the Telegram network: the AES-IGE cipher,
// the authorization key exchange, the message framing, the obfuscated
// transport and the serialization layer of the protocol language.
//
// The API of the network itself is not part of the protocol: a program passes
// the identifier of the method it calls and the serialized arguments, so the
// module stays independent of the API schema.
package stdlib

import (
	"math/big"
	"sync"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/mtproto"
	"github.com/alca-lang/alca/pkg/sandbox"
)

var (
	mtprotoMu        sync.Mutex
	mtprotoExchanges = map[int64]*mtproto.DHExchange{}
	mtprotoSessions  = map[int64]*mtproto.Client{}
	mtprotoNextID    int64
)

// RegisterMTProtoBuiltins registers "std.mtproto" and "mtproto" into the interpreter environment.
func RegisterMTProtoBuiltins(env *interpreter.Environment) {
	mtModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		mtModule.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}

	// mtproto.ige_encrypt(key, data, iv) -> blob
	register("ige_encrypt", func(args ...interpreter.Value) interpreter.Value {
		key, ok := blobArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.ige_encrypt requires a key")
		}
		data, ok := blobArg(args, 1)
		if !ok {
			return interpreter.NewError(400, "mtproto.ige_encrypt requires data")
		}
		iv, ok := blobArg(args, 2)
		if !ok {
			return interpreter.NewError(400, "mtproto.ige_encrypt requires an initialization vector")
		}
		out, err := mtproto.EncryptIGE(key, data, iv)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		return &interpreter.BlobValue{Data: out}
	})

	// mtproto.ige_decrypt(key, data, iv) -> blob
	register("ige_decrypt", func(args ...interpreter.Value) interpreter.Value {
		key, ok := blobArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.ige_decrypt requires a key")
		}
		data, ok := blobArg(args, 1)
		if !ok {
			return interpreter.NewError(400, "mtproto.ige_decrypt requires data")
		}
		iv, ok := blobArg(args, 2)
		if !ok {
			return interpreter.NewError(400, "mtproto.ige_decrypt requires an initialization vector")
		}
		out, err := mtproto.DecryptIGE(key, data, iv)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		return &interpreter.BlobValue{Data: out}
	})

	// mtproto.key_id(key) -> blob
	register("key_id", func(args ...interpreter.Value) interpreter.Value {
		key, ok := blobArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.key_id requires a key")
		}
		id, err := mtproto.AuthKeyID(key)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		return &interpreter.BlobValue{Data: append([]byte(nil), id[:]...)}
	})

	// mtproto.fingerprint(key) -> blob
	register("fingerprint", func(args ...interpreter.Value) interpreter.Value {
		key, ok := blobArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.fingerprint requires a key")
		}
		authKey, err := mtproto.NewAuthKey(key)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		fingerprint := authKey.Fingerprint()
		return &interpreter.BlobValue{Data: append([]byte(nil), fingerprint[:]...)}
	})

	// mtproto.parse_key(hex) -> blob
	register("parse_key", func(args ...interpreter.Value) interpreter.Value {
		key, err := mtproto.ParseAuthKey(interpreter.CoerceToString(args[0]))
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		return &interpreter.BlobValue{Data: key.Value}
	})

	// mtproto.key_hex(key) -> string
	register("key_hex", func(args ...interpreter.Value) interpreter.Value {
		key, ok := blobArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.key_hex requires a key")
		}
		authKey, err := mtproto.NewAuthKey(key)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		return &interpreter.StringValue{Val: authKey.Hex()}
	})

	// mtproto.dh_start(modulus_base64) -> map: the request of a key exchange.
	// The modulus is the modulus of the server's RSA key.
	register("dh_start", func(args ...interpreter.Value) interpreter.Value {
		text := interpreter.CoerceToString(args[0])
		raw, err := mtproto.DecodeBase64(text)
		if err != nil {
			return interpreter.NewError(400, "mtproto.dh_start: the modulus is not base64")
		}
		modulus := new(big.Int).SetBytes(raw)
		if modulus.Sign() <= 0 {
			return interpreter.NewError(400, "mtproto.dh_start: the modulus is empty")
		}
		exchange, err := mtproto.NewDHExchange(modulus, time.Now().Unix())
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		request := exchange.Request()
		id := mtprotoNextIdentifier()
		mtprotoMu.Lock()
		mtprotoExchanges[id] = exchange
		mtprotoMu.Unlock()

		result := interpreter.NewMap()
		result.Set("id", &interpreter.IntegerValue{Val: id})
		result.Set("prime", &interpreter.BlobValue{Data: request.Prime.Bytes()})
		result.Set("generator", &interpreter.IntegerValue{Val: request.Generator.Int64()})
		result.Set("ga", &interpreter.BlobValue{Data: request.GA.Bytes()})
		result.Set("time", &interpreter.IntegerValue{Val: request.TimeA})
		return result
	})

	// mtproto.dh_derive(id, server_nonce, gb, new_nonce, aux_hash) -> blob
	register("dh_derive", func(args ...interpreter.Value) interpreter.Value {
		id, ok := intArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.dh_derive requires the identifier of the exchange")
		}
		mtprotoMu.Lock()
		exchange := mtprotoExchanges[id]
		mtprotoMu.Unlock()
		if exchange == nil {
			return interpreter.NewError(400, "mtproto.dh_derive: the exchange is unknown")
		}
		serverNonce, ok := blobArg(args, 1)
		if !ok {
			return interpreter.NewError(400, "mtproto.dh_derive requires the nonce of the server")
		}
		gbRaw, ok := blobArg(args, 2)
		if !ok {
			return interpreter.NewError(400, "mtproto.dh_derive requires the public value of the server")
		}
		newNonce, ok := blobArg(args, 3)
		if !ok {
			return interpreter.NewError(400, "mtproto.dh_derive requires the new nonce")
		}
		auxHash, ok := blobArg(args, 4)
		if !ok {
			return interpreter.NewError(400, "mtproto.dh_derive requires the auxiliary hash")
		}
		key, err := exchange.Derive(mtproto.DHResponse{
			ServerNonce: serverNonce,
			GB:          new(big.Int).SetBytes(gbRaw),
		}, newNonce, auxHash)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		return &interpreter.BlobValue{Data: key.Value}
	})

	// mtproto.rsa_encrypt(public_key_pem, nonce, data) -> blob
	register("rsa_encrypt", func(args ...interpreter.Value) interpreter.Value {
		public, err := mtproto.ParseRSAPublicKey(interpreter.CoerceToString(args[0]))
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		nonce, ok := blobArg(args, 1)
		if !ok {
			return interpreter.NewError(400, "mtproto.rsa_encrypt requires a nonce")
		}
		data, ok := blobArg(args, 2)
		if !ok {
			return interpreter.NewError(400, "mtproto.rsa_encrypt requires data")
		}
		out, err := mtproto.EncryptRSAData(public, nonce, data)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		return &interpreter.BlobValue{Data: out}
	})

	// mtproto.encrypt_message(key, salt, session_id, msg_id, seq_no, payload) -> blob
	register("encrypt_message", func(args ...interpreter.Value) interpreter.Value {
		keyRaw, ok := blobArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.encrypt_message requires a key")
		}
		key, err := mtproto.NewAuthKey(keyRaw)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		salt, _ := intArg(args, 1)
		sessionID, _ := intArg(args, 2)
		msgID, _ := intArg(args, 3)
		seqNo, _ := intArg(args, 4)
		payload, ok := blobArg(args, 5)
		if !ok {
			return interpreter.NewError(400, "mtproto.encrypt_message requires a payload")
		}
		encrypted, err := mtproto.EncryptMessage(key, mtproto.PlainMessage{
			Salt:      salt,
			SessionID: sessionID,
			MsgID:     msgID,
			SeqNo:     int32(seqNo),
			Payload:   payload,
		})
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.BlobValue{Data: mtproto.EncodeMessage(encrypted)}
	})

	// mtproto.decrypt_message(key, direction, wire) -> map
	register("decrypt_message", func(args ...interpreter.Value) interpreter.Value {
		keyRaw, ok := blobArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.decrypt_message requires a key")
		}
		key, err := mtproto.NewAuthKey(keyRaw)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		direction := mtproto.ToServer
		if len(args) > 1 && interpreter.CoerceToString(args[1]) == "to_client" {
			direction = mtproto.ToClient
		}
		wire, ok := blobArg(args, 2)
		if !ok {
			return interpreter.NewError(400, "mtproto.decrypt_message requires the message")
		}
		message, err := mtproto.DecodeMessage(wire)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		plain, err := mtproto.DecryptMessage(key, direction, message)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		result := interpreter.NewMap()
		result.Set("salt", &interpreter.IntegerValue{Val: plain.Salt})
		result.Set("session_id", &interpreter.IntegerValue{Val: plain.SessionID})
		result.Set("msg_id", &interpreter.IntegerValue{Val: plain.MsgID})
		result.Set("seq_no", &interpreter.IntegerValue{Val: int64(plain.SeqNo)})
		result.Set("payload", &interpreter.BlobValue{Data: plain.Payload})
		return result
	})

	// mtproto.new_msg_id(length) -> int
	register("new_msg_id", func(args ...interpreter.Value) interpreter.Value {
		length, _ := intArg(args, 0)
		return &interpreter.IntegerValue{Val: mtproto.NewMsgID(int(length), time.Now())}
	})

	// The serialization layer of the protocol language.
	register("tl_int32", func(args ...interpreter.Value) interpreter.Value {
		value, _ := intArg(args, 0)
		writer := mtproto.NewWriter()
		writer.WriteInt32(int32(value))
		return &interpreter.BlobValue{Data: writer.Bytes()}
	})
	register("tl_int64", func(args ...interpreter.Value) interpreter.Value {
		value, _ := intArg(args, 0)
		writer := mtproto.NewWriter()
		writer.WriteInt64(value)
		return &interpreter.BlobValue{Data: writer.Bytes()}
	})
	register("tl_bool", func(args ...interpreter.Value) interpreter.Value {
		value := len(args) > 0 && interpreter.IsTruthy(args[0])
		writer := mtproto.NewWriter()
		writer.WriteBool(value)
		return &interpreter.BlobValue{Data: writer.Bytes()}
	})
	register("tl_string", func(args ...interpreter.Value) interpreter.Value {
		writer := mtproto.NewWriter()
		writer.WriteString(interpreter.CoerceToString(args[0]))
		return &interpreter.BlobValue{Data: writer.Bytes()}
	})
	register("tl_bytes", func(args ...interpreter.Value) interpreter.Value {
		data, ok := blobArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.tl_bytes requires data")
		}
		writer := mtproto.NewWriter()
		writer.WriteBytes(data)
		return &interpreter.BlobValue{Data: writer.Bytes()}
	})
	register("tl_read_int32", func(args ...interpreter.Value) interpreter.Value {
		data, ok := blobArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.tl_read_int32 requires data")
		}
		value, err := mtproto.NewReader(data).ReadInt32()
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		return &interpreter.IntegerValue{Val: int64(value)}
	})
	register("tl_read_int64", func(args ...interpreter.Value) interpreter.Value {
		data, ok := blobArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.tl_read_int64 requires data")
		}
		value, err := mtproto.NewReader(data).ReadInt64()
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		return &interpreter.IntegerValue{Val: value}
	})
	register("tl_read_bool", func(args ...interpreter.Value) interpreter.Value {
		data, ok := blobArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.tl_read_bool requires data")
		}
		value, err := mtproto.NewReader(data).ReadBool()
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		if value {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})
	register("tl_read_string", func(args ...interpreter.Value) interpreter.Value {
		data, ok := blobArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.tl_read_string requires data")
		}
		value, err := mtproto.NewReader(data).ReadString()
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		return &interpreter.StringValue{Val: value}
	})
	register("tl_read_bytes", func(args ...interpreter.Value) interpreter.Value {
		data, ok := blobArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.tl_read_bytes requires data")
		}
		value, err := mtproto.NewReader(data).ReadBytes()
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		return &interpreter.BlobValue{Data: value}
	})

	// mtproto.dc_list() -> list[map]: the data centers of the network.
	register("dc_list", func(args ...interpreter.Value) interpreter.Value {
		endpoints := mtproto.KnownEndpoints()
		list := make([]interpreter.Value, 0, len(endpoints))
		for _, endpoint := range endpoints {
			entry := interpreter.NewMap()
			entry.Set("dc", &interpreter.IntegerValue{Val: int64(endpoint.DC)})
			entry.Set("ip", &interpreter.StringValue{Val: endpoint.IP})
			entry.Set("port", &interpreter.IntegerValue{Val: int64(endpoint.Port)})
			list = append(list, entry)
		}
		return interpreter.NewList(list...)
	})

	// mtproto.connect(dc) -> map: a session with a data center.
	register("connect", func(args ...interpreter.Value) interpreter.Value {
		number, ok := intArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.connect requires the number of a data center")
		}
		var endpoint mtproto.Endpoint
		found := false
		for _, candidate := range mtproto.KnownEndpoints() {
			if int64(candidate.DC) == number {
				endpoint = candidate
				found = true
				break
			}
		}
		if !found {
			return interpreter.NewError(400, "mtproto.connect: the data center is unknown")
		}
		if err := sandbox.RequireNet(endpoint.Address()); err != nil {
			return interpreter.NewError(403, "access denied: "+err.Error())
		}
		client, err := mtproto.Connect(endpoint, 10*time.Second)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		id := mtprotoNextIdentifier()
		mtprotoMu.Lock()
		mtprotoSessions[id] = client
		mtprotoMu.Unlock()
		result := interpreter.NewMap()
		result.Set("id", &interpreter.IntegerValue{Val: id})
		result.Set("dc", &interpreter.IntegerValue{Val: number})
		result.Set("address", &interpreter.StringValue{Val: endpoint.Address()})
		return result
	})

	// mtproto.use_key(session, key) -> bool: installs the authorization key.
	register("use_key", func(args ...interpreter.Value) interpreter.Value {
		client, ok := mtprotoSession(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.use_key requires a session")
		}
		keyRaw, ok := blobArg(args, 1)
		if !ok {
			return interpreter.NewError(400, "mtproto.use_key requires a key")
		}
		key, err := mtproto.NewAuthKey(keyRaw)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		client.SetAuthKey(key)
		return interpreter.TRUE
	})

	// mtproto.call(session, method_id, body) -> blob: calls a method of the
	// API. The method identifier and the serialized arguments come from the
	// schema of the API.
	register("call", func(args ...interpreter.Value) interpreter.Value {
		client, ok := mtprotoSession(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.call requires a session")
		}
		method, ok := intArg(args, 1)
		if !ok {
			return interpreter.NewError(400, "mtproto.call requires the identifier of the method")
		}
		body, ok := blobArg(args, 2)
		if !ok {
			return interpreter.NewError(400, "mtproto.call requires the arguments of the method")
		}
		request := mtproto.NewWriter()
		request.WriteUInt32(uint32(method))
		request.WriteRaw(body)
		if err := client.Send(request.Bytes()); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		result, err := client.Receive()
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.BlobValue{Data: result}
	})

	// mtproto.close(session) -> bool
	register("close", func(args ...interpreter.Value) interpreter.Value {
		client, ok := mtprotoSession(args, 0)
		if !ok {
			return interpreter.NewError(400, "mtproto.close requires a session")
		}
		mtprotoMu.Lock()
		for id, session := range mtprotoSessions {
			if session == client {
				delete(mtprotoSessions, id)
			}
		}
		mtprotoMu.Unlock()
		if err := client.Close(); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	})

	env.Set("mtproto", mtModule)
	env.Set("std.mtproto", mtModule)
}

// mtprotoNextIdentifier returns the identifier of the next session or exchange.
func mtprotoNextIdentifier() int64 {
	mtprotoMu.Lock()
	defer mtprotoMu.Unlock()
	mtprotoNextID++
	return mtprotoNextID
}

// mtprotoSession reads the session of an argument.
func mtprotoSession(args []interpreter.Value, index int) (*mtproto.Client, bool) {
	if index >= len(args) {
		return nil, false
	}
	id, ok := intArg(args, index)
	if !ok {
		return nil, false
	}
	mtprotoMu.Lock()
	defer mtprotoMu.Unlock()
	client := mtprotoSessions[id]
	return client, client != nil
}
