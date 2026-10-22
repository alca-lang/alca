// Package mtproto implements the MTProto 2.0 protocol used by Telegram.
//
// The package covers the protocol itself: the AES-IGE cipher, the
// authorization key generation and storage, the message framing with its
// message keys and sequence numbers, the obfuscated2 transport and the
// serialization layer of the protocol language.
//
// The Telegram API (the method identifiers and the type schemas) is not part
// of the protocol layer: a program supplies the method identifier and the
// serialized arguments, so the module stays independent of the API schema.
package mtproto

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

// ErrIGEInput is returned when the data of an IGE operation has a length that
// is not a multiple of the block size.
var ErrIGEInput = errors.New("mtproto: the IGE input must be a multiple of the block size")

// igeCipher encrypts and decrypts in Infinite Garble Extension mode. IGE is
// the mode MTProto uses for the message body: every block depends on the
// previous cipher and plain block, so a message cannot be cut or reordered.
type igeCipher struct {
	block cipher.Block
}

// newIGE returns an IGE cipher for the given 256-bit key.
func newIGE(key []byte) (*igeCipher, error) {
	if len(key) != 32 {
		return nil, errors.New("mtproto: the IGE key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return &igeCipher{block: block}, nil
}

// Encrypt encrypts data in IGE mode. The IV is 32 bytes: the first half
// initializes the cipher chain, the second half the plain chain.
//
//	C_i = E(M_i XOR C_{i-1}) XOR M_{i-1}
func (c *igeCipher) Encrypt(data, iv []byte) ([]byte, error) {
	if len(iv) != 32 {
		return nil, errors.New("mtproto: the IGE IV must be 32 bytes")
	}
	if len(data)%aes.BlockSize != 0 {
		return nil, ErrIGEInput
	}
	out := make([]byte, len(data))
	cipherBlock := make([]byte, aes.BlockSize)
	plainBlock := make([]byte, aes.BlockSize)
	copy(cipherBlock, iv[:aes.BlockSize])
	copy(plainBlock, iv[aes.BlockSize:])

	tmp := make([]byte, aes.BlockSize)
	encrypted := make([]byte, aes.BlockSize)
	for offset := 0; offset < len(data); offset += aes.BlockSize {
		block := data[offset : offset+aes.BlockSize]
		for i := range tmp {
			tmp[i] = block[i] ^ cipherBlock[i]
		}
		c.block.Encrypt(encrypted, tmp)
		for i := range encrypted {
			encrypted[i] ^= plainBlock[i]
		}
		copy(out[offset:offset+aes.BlockSize], encrypted)
		copy(cipherBlock, encrypted)
		copy(plainBlock, block)
	}
	return out, nil
}

// Decrypt decrypts data in IGE mode.
//
//	M_i = D(C_i XOR M_{i-1}) XOR C_{i-1}
func (c *igeCipher) Decrypt(data, iv []byte) ([]byte, error) {
	if len(iv) != 32 {
		return nil, errors.New("mtproto: the IGE IV must be 32 bytes")
	}
	if len(data)%aes.BlockSize != 0 {
		return nil, ErrIGEInput
	}
	out := make([]byte, len(data))
	cipherBlock := make([]byte, aes.BlockSize)
	plainBlock := make([]byte, aes.BlockSize)
	copy(cipherBlock, iv[:aes.BlockSize])
	copy(plainBlock, iv[aes.BlockSize:])

	tmp := make([]byte, aes.BlockSize)
	decrypted := make([]byte, aes.BlockSize)
	for offset := 0; offset < len(data); offset += aes.BlockSize {
		block := data[offset : offset+aes.BlockSize]
		for i := range tmp {
			tmp[i] = block[i] ^ plainBlock[i]
		}
		c.block.Decrypt(decrypted, tmp)
		for i := range decrypted {
			decrypted[i] ^= cipherBlock[i]
		}
		copy(out[offset:offset+aes.BlockSize], decrypted)
		copy(cipherBlock, block)
		copy(plainBlock, decrypted)
	}
	return out, nil
}

// EncryptIGE encrypts data with AES-256-IGE.
func EncryptIGE(key, data, iv []byte) ([]byte, error) {
	cipher, err := newIGE(key)
	if err != nil {
		return nil, err
	}
	return cipher.Encrypt(data, iv)
}

// DecryptIGE decrypts data with AES-256-IGE.
func DecryptIGE(key, data, iv []byte) ([]byte, error) {
	cipher, err := newIGE(key)
	if err != nil {
		return nil, err
	}
	return cipher.Decrypt(data, iv)
}
