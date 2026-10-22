package stdlib_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/stdlib"
)

func mtprotoModule(t *testing.T) *interpreter.MapValue {
	t.Helper()
	env := interpreter.NewEnvironment()
	stdlib.RegisterMTProtoBuiltins(env)
	value, ok := env.Get("std.mtproto")
	if !ok {
		t.Fatal("expected std.mtproto to be registered")
	}
	return value.(*interpreter.MapValue)
}

func mtprotoCall(t *testing.T, module *interpreter.MapValue, name string, args ...interpreter.Value) interpreter.Value {
	t.Helper()
	fn, ok := module.Get(name)
	if !ok {
		t.Fatalf("std.mtproto has no member %q", name)
	}
	return fn.(*interpreter.BuiltinFunction).Fn(args...)
}

func TestMTProtoModuleIGERoundTrip(t *testing.T) {
	module := mtprotoModule(t)

	key := make([]byte, 32)
	iv := make([]byte, 32)
	data := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
		iv[i] = byte(100 + i)
		data[i] = byte(200 + i)
	}

	encrypted := mtprotoCall(t, module, "ige_encrypt",
		&interpreter.BlobValue{Data: key},
		&interpreter.BlobValue{Data: data},
		&interpreter.BlobValue{Data: iv})
	if encrypted.Type() == interpreter.ERROR_VAL {
		t.Fatalf("ige_encrypt failed: %s", encrypted.(*interpreter.ErrorValue).Message)
	}
	decrypted := mtprotoCall(t, module, "ige_decrypt",
		&interpreter.BlobValue{Data: key},
		encrypted,
		&interpreter.BlobValue{Data: iv})
	if decrypted.Type() == interpreter.ERROR_VAL {
		t.Fatalf("ige_decrypt failed: %s", decrypted.(*interpreter.ErrorValue).Message)
	}
	got := decrypted.(*interpreter.BlobValue).Data
	if len(got) != len(data) {
		t.Fatalf("decrypted %d bytes, want %d", len(got), len(data))
	}
	for i := range data {
		if got[i] != data[i] {
			t.Fatalf("the decrypted data differs at %d", i)
		}
	}
}

func TestMTProtoModuleMessageRoundTrip(t *testing.T) {
	module := mtprotoModule(t)

	key := make([]byte, 256)
	for i := range key {
		key[i] = byte(i)
	}
	payload := []byte("the body of a request")

	encrypted := mtprotoCall(t, module, "encrypt_message",
		&interpreter.BlobValue{Data: key},
		&interpreter.IntegerValue{Val: 11},
		&interpreter.IntegerValue{Val: 22},
		&interpreter.IntegerValue{Val: 33},
		&interpreter.IntegerValue{Val: 44},
		&interpreter.BlobValue{Data: payload})
	if encrypted.Type() == interpreter.ERROR_VAL {
		t.Fatalf("encrypt_message failed: %s", encrypted.(*interpreter.ErrorValue).Message)
	}
	decrypted := mtprotoCall(t, module, "decrypt_message",
		&interpreter.BlobValue{Data: key},
		&interpreter.StringValue{Val: "to_server"},
		encrypted)
	if decrypted.Type() == interpreter.ERROR_VAL {
		t.Fatalf("decrypt_message failed: %s", decrypted.(*interpreter.ErrorValue).Message)
	}
	result := decrypted.(*interpreter.MapValue)
	if salt, _ := result.Get("salt"); salt.(*interpreter.IntegerValue).Val != 11 {
		t.Fatalf("salt = %v, want 11", salt)
	}
	if seq, _ := result.Get("seq_no"); seq.(*interpreter.IntegerValue).Val != 44 {
		t.Fatalf("seq_no = %v, want 44", seq)
	}
	body, _ := result.Get("payload")
	if string(body.(*interpreter.BlobValue).Data) != string(payload) {
		t.Fatalf("payload = %q, want %q", body.(*interpreter.BlobValue).Data, payload)
	}
}

func TestMTProtoModuleKeyIdentifier(t *testing.T) {
	module := mtprotoModule(t)
	key := make([]byte, 256)
	for i := range key {
		key[i] = byte(i * 3)
	}
	id := mtprotoCall(t, module, "key_id", &interpreter.BlobValue{Data: key})
	if id.Type() == interpreter.ERROR_VAL {
		t.Fatalf("key_id failed: %s", id.(*interpreter.ErrorValue).Message)
	}
	if len(id.(*interpreter.BlobValue).Data) != 8 {
		t.Fatalf("the identifier is %d bytes, want 8", len(id.(*interpreter.BlobValue).Data))
	}
	fingerprint := mtprotoCall(t, module, "fingerprint", &interpreter.BlobValue{Data: key})
	if fingerprint.Type() == interpreter.ERROR_VAL {
		t.Fatalf("fingerprint failed: %s", fingerprint.(*interpreter.ErrorValue).Message)
	}
	if len(fingerprint.(*interpreter.BlobValue).Data) != 20 {
		t.Fatalf("the fingerprint is %d bytes, want 20", len(fingerprint.(*interpreter.BlobValue).Data))
	}
	// The hexadecimal form parses back into the same key.
	hexForm := mtprotoCall(t, module, "key_hex", &interpreter.BlobValue{Data: key})
	parsed := mtprotoCall(t, module, "parse_key", hexForm)
	if parsed.Type() == interpreter.ERROR_VAL {
		t.Fatalf("parse_key failed: %s", parsed.(*interpreter.ErrorValue).Message)
	}
	back := parsed.(*interpreter.BlobValue).Data
	if len(back) != len(key) {
		t.Fatalf("the parsed key is %d bytes, want %d", len(back), len(key))
	}
	for i := range key {
		if back[i] != key[i] {
			t.Fatalf("the parsed key differs at %d", i)
		}
	}
}

func TestMTProtoModuleSerialization(t *testing.T) {
	module := mtprotoModule(t)

	encoded := mtprotoCall(t, module, "tl_int32", &interpreter.IntegerValue{Val: 1234})
	if encoded.Type() == interpreter.ERROR_VAL {
		t.Fatalf("tl_int32 failed: %s", encoded.(*interpreter.ErrorValue).Message)
	}
	decoded := mtprotoCall(t, module, "tl_read_int32", encoded)
	if decoded.Type() == interpreter.ERROR_VAL {
		t.Fatalf("tl_read_int32 failed: %s", decoded.(*interpreter.ErrorValue).Message)
	}
	if value := decoded.(*interpreter.IntegerValue).Val; value != 1234 {
		t.Fatalf("tl_read_int32 = %d, want 1234", value)
	}

	text := mtprotoCall(t, module, "tl_string", &interpreter.StringValue{Val: "hello"})
	readText := mtprotoCall(t, module, "tl_read_string", text)
	if readText.Type() == interpreter.ERROR_VAL {
		t.Fatalf("tl_read_string failed: %s", readText.(*interpreter.ErrorValue).Message)
	}
	if value := readText.(*interpreter.StringValue).Val; value != "hello" {
		t.Fatalf("tl_read_string = %q, want hello", value)
	}
}

func TestMTProtoModuleDataCenters(t *testing.T) {
	module := mtprotoModule(t)
	list := mtprotoCall(t, module, "dc_list")
	if list.Type() == interpreter.ERROR_VAL {
		t.Fatalf("dc_list failed: %s", list.(*interpreter.ErrorValue).Message)
	}
	centers := list.(*interpreter.ListValue)
	if len(centers.Elements) == 0 {
		t.Fatal("the network must have data centers")
	}
	first := centers.Elements[0].(*interpreter.MapValue)
	ip, ok := first.Get("ip")
	if !ok || ip.(*interpreter.StringValue).Val == "" {
		t.Fatalf("the first data center has no address: %v", first)
	}
}
