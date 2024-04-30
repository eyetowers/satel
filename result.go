package satel

import "fmt"

type Result byte

const (
	Ok                        Result = 0x00
	ReqUserCodeNotFound       Result = 0x01
	NoAccess                  Result = 0x02
	SelectedUserNotExist      Result = 0x03
	SelectedUserAlreadyExists Result = 0x04
	WrongOrDuplicateCode      Result = 0x05
	TelephoneCodeExists       Result = 0x06
	ChangedCodeSame           Result = 0x07
	OtherError                Result = 0x08
	CannotArmButForceArm      Result = 0x11
	CannotArm                 Result = 0x12
	OtherErrors               Result = 0x80 // Placeholder for other errors with 8 as the prefix
	CommandAccepted           Result = 0xFF
)

func (r Result) String() string {
	if r >= 0x80 && r <= 0x8F {
		return fmt.Sprintf("Other errors 0x%02X", byte(r))
	}

	strings := map[Result]string{
		Ok:                        "OK",
		ReqUserCodeNotFound:       "Requesting user code not found",
		NoAccess:                  "No access",
		SelectedUserNotExist:      "Selected user does not exist",
		SelectedUserAlreadyExists: "Selected user already exists",
		WrongOrDuplicateCode:      "Wrong code or code already exists",
		TelephoneCodeExists:       "Telephone code already exists",
		ChangedCodeSame:           "Changed code is the same",
		OtherError:                "Other error",
		CannotArmButForceArm:      "Can not arm, but can use force arm",
		CannotArm:                 "Can not arm",
		OtherErrors:               "Other errors",
		CommandAccepted:           "Command accepted (data length and CRC OK), will be processed",
	}
	return strings[r]
}

func (r Result) IsError() bool {
	return (r != Ok && r != CommandAccepted)
}
