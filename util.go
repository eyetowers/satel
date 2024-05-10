package satel

import "errors"

var ErrInvalidChar = errors.New("usercode contains invalid character")
var ErrInvalidLength = errors.New("usercode does not match the expected length")

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
