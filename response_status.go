package satel

import "fmt"

type ResponseStatus byte

const (
	Ok                        ResponseStatus = 0x00
	ReqUsercodeNotFound       ResponseStatus = 0x01
	NoAccess                  ResponseStatus = 0x02
	SelectedUserNotExist      ResponseStatus = 0x03
	SelectedUserAlreadyExists ResponseStatus = 0x04
	WrongOrDuplicateCode      ResponseStatus = 0x05
	TelephoneCodeExists       ResponseStatus = 0x06
	ChangedCodeSame           ResponseStatus = 0x07
	OtherError                ResponseStatus = 0x08
	CannotArmButForceArm      ResponseStatus = 0x11
	CannotArm                 ResponseStatus = 0x12
	OtherErrors               ResponseStatus = 0x80 // Placeholder for other errors with 8 as the prefix
	CommandAccepted           ResponseStatus = 0xFF
)

var ResponseStatusStrings = map[ResponseStatus]string{
	Ok:                        "ok",
	ReqUsercodeNotFound:       "requesting user code not found",
	NoAccess:                  "no access",
	SelectedUserNotExist:      "selected user does not exist",
	SelectedUserAlreadyExists: "selected user already exists",
	WrongOrDuplicateCode:      "wrong code or code already exists",
	TelephoneCodeExists:       "telephone code already exists",
	ChangedCodeSame:           "changed code is the same",
	OtherError:                "other error",
	CannotArmButForceArm:      "can not arm, but can use force arm",
	CannotArm:                 "can not arm",
	OtherErrors:               "other errors",
	CommandAccepted:           "command accepted (data length and CRC OK), will be processed",
}

func (r ResponseStatus) String() string {
	if r >= 0x80 && r <= 0x8F {
		return fmt.Sprintf("other errors 0x%02X", byte(r))
	}

	if ResponseStatusStrings[r] == "" {
		return "invalid response code"
	}

	return ResponseStatusStrings[r]
}

func (r ResponseStatus) IsError() bool {
	return (r != Ok && r != CommandAccepted)
}
