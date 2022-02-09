package hex

import (
	"encoding/hex"
	"fmt"
	"math/big"
)

func RemovePrefix(str string) string {
	if has0xPrefix(str) {
		return str[2:]
	}
	return str
}

func WithPrefix(str string) string {
	return "0x" + str
}

func has0xPrefix(input string) bool {
	return len(input) >= 2 && input[0] == '0' && (input[1] == 'x' || input[1] == 'X')
}

func Str2Big(str string) (*big.Int, error) {
	str = RemovePrefix(str)
	count, ok := new(big.Int).SetString(str, 16)
	if !ok {
		return nil, fmt.Errorf("Can't parse string to big int")
	}
	// Count should be positive
	if count.Sign() != 1 {
		return nil, fmt.Errorf("Count should be positive")
	}
	return count, nil
}

// Encode encodes b as a hex string with 0x prefix.
func Encode(b []byte) string {
	enc := make([]byte, len(b)*2+2)
	copy(enc, "0x")
	hex.Encode(enc[2:], b)
	return string(enc)
}