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

func subscribe(bytes ...StateType) []byte {
	subs := make([]byte, 6)

	for _, b := range bytes {
		pos := b / 8
		subs[pos] = subs[pos] | (1 << (b % 8))
	}

	subs2 := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	return append([]byte{0x7F}, append(subs, subs2...)...)
}

func subsStates() []StateType {
	// first 23 states and Trouble Part 3.
	states := []StateType{
		ZoneViolation,
		ZoneTamper,
		ZoneAlarm,
		ZoneTamperAlarm,
		ZoneAlarmMemory,
		ZoneTamperAlarmMemory,
		ZoneBypass,
		ZoneNoViolationTrouble,
		ZoneLongViolationTrouble,
		ArmedPartitionSuppressed,
		ArmedPartition,
		PartitionArmedInMode2,
		PartitionArmedInMode3,
		PartitionWith1stCodeEntered,
		PartitionEntryTime,
		PartitionExitTimeOver10s,
		PartitionExitTimeUnder10s,
		PartitionTemporaryBlocked,
		PartitionBlockedForGuardRound,
		PartitionAlarm,
		PartitionFireAlarm,
		PartitionAlarmMemory,
		PartitionFireAlarmMemory,
		Output,
		TroublePart3,
	}
	return states
}
