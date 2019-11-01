package aes

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"math/rand"
)

func Encrypt(key, data []byte) (iv, output []byte, err error) {
	var block cipher.Block
	block, err = aes.NewCipher(key)
	if err != nil {
		return
	}

	iv = make([]byte, 16)
	_, err = rand.Read(iv)
	if err != nil {
		return
	}

	stream := cipher.NewCBCEncrypter(block, iv)
	bufferSize := calcEncryptedSize(uint32(len(data)))
	output = make([]byte, bufferSize)
	binary.LittleEndian.PutUint32(output, uint32(len(data)))
	copy(output[4:], data)
	stream.CryptBlocks(output, output)
	return
}

func Decrypt(key, iv, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return []byte{}, err
	}

	stream := cipher.NewCBCDecrypter(block, iv)
	output := make([]byte, len(data))
	stream.CryptBlocks(output, data)
	size := binary.LittleEndian.Uint32(output[:4])
	return output[4 : 4+size], nil
}

func calcEncryptedSize(length uint32) uint32 {
	length += 4
	bufferSize := (length / 16) * 16 // block_size = 16
	if bufferSize < length {
		bufferSize = bufferSize + 16
	}
	return bufferSize
}
