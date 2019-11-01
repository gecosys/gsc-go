package security

import (
	"crypto/rand"
	"errors"
	"math/big"

	pb "github.com/gecosys/gsc-go/message"
	aes "github.com/gecosys/gsc-go/security/aes"
	rsa "github.com/gecosys/gsc-go/security/rsa"

	"github.com/golang/protobuf/proto"
)

// RSA public key
var publicKey *rsa.PublicKey

// AES key
var sharedKey []byte

func Setup(key []byte) error {
	// Generate shared key AES
	sharedKey = make([]byte, 32)
	_, err := rand.Read(sharedKey)
	if err != nil {
		return err
	}

	// Setup public key RSA
	eKey := pb.PublicKey{}
	err = proto.Unmarshal(key, &eKey)
	if err != nil {
		return err
	}
	e, ok := new(big.Int).SetString(eKey.E, 10)
	if ok == false {
		return errors.New("Cannot setup public key")
	}
	n, ok := new(big.Int).SetString(eKey.N, 10)
	if ok == false {
		return errors.New("Cannot setup public key")
	}
	publicKey = new(rsa.PublicKey)
	publicKey.E = e
	publicKey.N = n
	return nil
}

// GetSharedKey returns shared key encrypted by RSA
// Output:
//  output: the encrypted shared key
//  err: error occurred
func GetSharedKey() (output []byte, err error) {
	output, err = EncryptRSA(sharedKey)
	return
}

// EncryptRSA encrypts data with RSA
// Input:
//  data: content will be encrypted
// Output:
//  output: the encrypted data
//  err: error occurred
func EncryptRSA(data []byte) (output []byte, err error) {
	output, err = rsa.Encrypt(publicKey, data)
	return
}

// Encrypt encrypts data with AES
// Input:
//  data: content will be encrypted
// Output:
//  iv: vector AES
//  output: the encrypted data
//  err: error occurred
func Encrypt(data []byte) (iv, output []byte, err error) {
	iv, output, err = aes.Encrypt(sharedKey, data)
	return
}

// Decrypt decrypts data with AES
// Input:
//  iv: vector AES
//  data: encrypted content
// Output:
//  output: the decrypted data
//  err: error occurred
func Decrypt(iv, data []byte) (output []byte, err error) {
	output, err = aes.Decrypt(sharedKey, iv, data)
	return
}
