package rsa

import (
	"math"
	"math/big"
	"math/rand"
	"strings"
)

func Encrypt(key *PublicKey, data []byte) ([]byte, error) {
	encodedData, err := encode(data)
	if err != nil {
		return []byte{}, err
	}
	buffer := make([]string, len(encodedData))
	for idx, d := range encodedData {
		buffer[idx] = new(big.Int).Exp(big.NewInt(int64(d)), key.E, key.N).Text(10) // (msg ^ E) mod N
	}

	content := []byte(strings.Join(buffer, ","))
	return content, nil
}

func encode(data []byte) ([]byte, error) {
	size := len(data)
	mask := make([]byte, 32)
	sizeMask := byte(math.Min(float64(size), 32))
	buffer := make([]byte, int(sizeMask)+size+1)

	// Setup mask
	buffer[0] = sizeMask
	_, err := rand.Read(mask)
	if err != nil {
		return []byte{}, err
	}
	for iMask := byte(0); iMask < sizeMask; iMask++ {
		buffer[iMask+1] = mask[iMask]
	}

	// Setup output
	for iByte := int(0); iByte < size; iByte++ {
		buffer[int(sizeMask)+iByte+1] = data[iByte] ^ mask[iByte%int(sizeMask)]
	}
	return buffer, nil
}
