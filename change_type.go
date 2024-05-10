package satel

type ChangeType byte

const (
	ZoneViolation ChangeType = iota
	ZoneTamper
	ZoneAlarm
	ZoneTamperAlarm
	ZoneAlarmMemory
	ZoneTamperAlarmMemory
	ZoneBypass
	ZoneNoViolationTrouble
	ZoneLongViolationTrouble
	ArmedPartitionSuppressed
	ArmedPartition
	PartitionArmedInMode2
	PartitionArmedInMode3
	PartitionWith1stCodeEntered
	PartitionEntryTime
	PartitionExitTimeOver10s
	PartitionExitTimeUnder10s
	PartitionTemporaryBlocked
	PartitionBlockedForGuardRound
	PartitionAlarm
	PartitionFireAlarm
	PartitionAlarmMemory
	PartitionFireAlarmMemory
	Output
	DoorOpened
	DoorOpenedLong
	StatusBit
	TroublePart1
	TroublePart2
	TroublePart3
	TroublePart4
	TroublePart5
	TroubleMemoryPart1
	TroubleMemoryPart2
	TroubleMemoryPart3
	TroubleMemoryPart4
	TroubleMemoryPart5
	PartitionWithViolatedZones
	ZoneIsolate
)

func (c ChangeType) String() string {
	strings := [...]string{
		"zone-violation",
		"zone-tamper",
		"zone-alarm",
		"zone-tamper-alarm",
		"zone-alarm-memory",
		"zone-tamper-alarm-memory",
		"zone-bypass",
		"zone-no-violation-trouble",
		"zone-long-violation-trouble",
		"armed-partition-suppressed",
		"armed-partition",
		"partition-armed-mode-2",
		"partition-armed-mode-3",
		"partition-with-1st-code-entered",
		"partition-entry-time",
		"partition-exit-time-over-10s",
		"partition-exit-time-under-10s",
		"partition-temporary-blocked",
		"partition-blocked-guard-round",
		"partition-alarm",
		"partition-fire-alarm",
		"partition-alarm-memory",
		"partition-fire-alarm-memory",
		"output",
		"doors-opened",
		"doors-opened-long",
		"status-bit",
		"trouble-part-1",
		"trouble-part-2",
		"trouble-part-3",
		"trouble-part-4",
		"trouble-part-5",
		"trouble-memory-part-1",
		"trouble-memory-part-2",
		"trouble-memory-part-3",
		"trouble-memory-part-4",
		"trouble-memory-part-5",
		"partition-with-violated-zones",
		"zone-isolate",
	}
	return strings[c]
}
