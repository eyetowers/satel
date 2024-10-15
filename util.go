package satel

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

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

func decomposePayload(bytes ...byte) (byte, []byte, error) {
	const minByteLength = 3
	if len(bytes) < minByteLength {
		return 0, nil, ErrCorruptedResponse
	}
	crcIndex := len(bytes) - 2
	crc := bytes[crcIndex:]
	cmd := bytes[0]
	dataWithCmd := bytes[:crcIndex]
	data := dataWithCmd[1:]

	if !isCrcValid(dataWithCmd, crc) {
		return 0, nil, ErrCrcNotMatch
	}

	if cmd == 0xFE {
		return 0, nil, ErrForbiddenCommand
	}
	return cmd, data, nil
}

func decodePartition(data []byte, lang language) (byte, uint64, string) {
	deviceType := data[0]
	partitionID := data[1]
	// Note: We are currently not handling the function byte.
	name := decodeString(data[3:], lang)
	return deviceType, uint64(partitionID), name
}

func decodeZone(data []byte, lang language) (byte, uint64, string, uint64) {
	deviceType := data[0]
	zoneID := data[1]
	// Note: We are currently not handling the function byte.
	name := decodeString(data[3:len(data)-1], lang)
	partition := data[len(data)-1]
	return deviceType, uint64(zoneID), name, uint64(partition)
}

func decodeOutput(data []byte, lang language) (byte, uint64, OutputFunction, string) {
	deviceType := data[0]
	outputID := data[1]
	outputFunction := OutputFunction(data[2])
	name := decodeString(data[3:], lang)
	return deviceType, uint64(outputID), outputFunction, name
}

func decodeString(data []byte, language language) string {
	reader := transformReader(bytes.NewReader(data), language)
	decodedData, err := io.ReadAll(reader)
	if err != nil {
		// If we encounter an error, force the string into Ascii only characters.
		decodedData = toAscii(data)
	}

	return strings.TrimSpace(string(decodedData))
}

var languangeEncodings = map[language]encoding.Encoding{
	// Central european languages.
	Czech:     charmap.Windows1250,
	Slovakian: charmap.Windows1250,
	Polish:    charmap.Windows1250,

	// Western european languages.
	German:  charmap.Windows1252,
	French:  charmap.Windows1252,
	Spanish: charmap.Windows1252,

	// TODO: List default encodings for other languages.
}

func transformReader(r io.Reader, l language) io.Reader {
	enc, ok := languangeEncodings[l]
	if !ok {
		return unicode.UTF8.NewDecoder().Reader(r)
	}
	return enc.NewDecoder().Reader(r)
}

// toAscii returns bytes with all non standard ASCII characters replaced with '?'.
func toAscii(data []byte) []byte {
	result := make([]byte, len(data))
	for i, b := range data {
		if b < 0x20 || b > 0x7E {
			data[i] = '?'
		}
	}
	return result
}
