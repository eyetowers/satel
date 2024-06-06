package satel

import "errors"

var ErrInvalidChar = errors.New("usercode contains invalid character")
var ErrInvalidLength = errors.New("usercode does not match the expected length")

const subscribeCmd = 0x7F

func validateUsercode(usercode string) error {
	if len(usercode) != 4 {
		return ErrInvalidLength
	}

	for _, char := range usercode {
		if char < '0' || char > '9' {
			return ErrInvalidChar
		}
	}
	return nil
}

func transformSubscription(bytes ...StateType) []byte {
	payload := make([]byte, 13)
	payload[0] = subscribeCmd
	for _, b := range bytes {
		pos := b/8 + 1
		payload[pos] |= 1 << (b % 8)
	}
	return payload
}

func transformCode(code string) []byte {
	bytes := make([]byte, 8)
	for i := 0; i < 16; i++ {
		if i < len(code) {
			digit := code[i]
			if i%2 == 0 {
				bytes[i/2] = (digit - '0') << 4
			} else {
				bytes[i/2] |= digit - '0'
			}
		} else if i%2 == 0 {
			bytes[i/2] = 0xFF
		} else if i == len(code) {
			bytes[i/2] |= 0x0F
		}
	}
	return bytes
}
