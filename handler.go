package satel

type Handler interface {
	OnZoneViolations(index int, state, initial bool)
	OnZoneTamper(index int, state, initial bool)
	OnZoneAlarm(index int, state, initial bool)
	OnZoneTamperAlarm(index int, state, initial bool)
	OnZoneAlarmMemory(index int, state, initial bool)
	OnZoneTamperAlarmMemory(index int, state, initial bool)
	OnZoneBypass(index int, state, initial bool)
	OnZoneNoViolationTrouble(index int, state, initial bool)
	OnZoneLongViolationTrouble(index int, state, initial bool)
	OnArmedPartitionSuppressed(index int, state, initial bool)
	OnArmedPartition(index int, state, initial bool)
	OnPartitionArmedInMode2(index int, state, initial bool)
	OnPartitionArmedInMode3(index int, state, initial bool)
	OnPartitionWith1stCodeEntered(index int, state, initial bool)
	OnPartitionEntryTime(index int, state, initial bool)
	OnPartitionExitTimeOver10s(index int, state, initial bool)
	OnPartitionExitTimeUnder10s(index int, state, initial bool)
	OnPartitionTemporaryBlocked(index int, state, initial bool)
	OnPartitionBlockedForGuardRound(index int, state, initial bool)
	OnPartitionAlarm(index int, state, initial bool)
	OnPartitionFireAlarm(index int, state, initial bool)
	OnPartitionAlarmMemory(index int, state, initial bool)
	OnPartitionFireAlarmMemory(index int, state, initial bool)
	OnOutput(index int, state, initial bool)
	OnDoorOpened(index int, state, initial bool)
	OnDoorOpenedLong(index int, state, initial bool)
	OnStatusBit(index int, state, initial bool)
	OnTroublePart1(index int, state, initial bool)
	OnTroublePart2(index int, state, initial bool)
	OnTroublePart3(index int, state, initial bool)
	OnTroublePart4(index int, state, initial bool)
	OnTroublePart5(index int, state, initial bool)
	OnTroubleMemoryPart1(index int, state, initial bool)
	OnTroubleMemoryPart2(index int, state, initial bool)
	OnTroubleMemoryPart3(index int, state, initial bool)
	OnTroubleMemoryPart4(index int, state, initial bool)
	OnTroubleMemoryPart5(index int, state, initial bool)
	OnPartitionWithViolatedZones(index int, state, initial bool)
	OnZoneIsolate(index int, state, initial bool)

	OnError(err error)
}

func handlerFunc(h Handler, cmd ChangeType) func(int, bool, bool) {
	functions := [...]func(int, bool, bool){
		h.OnZoneViolations,
		h.OnZoneTamper,
		h.OnZoneAlarm,
		h.OnZoneTamperAlarm,
		h.OnZoneAlarmMemory,
		h.OnZoneTamperAlarmMemory,
		h.OnZoneBypass,
		h.OnZoneNoViolationTrouble,
		h.OnZoneLongViolationTrouble,
		h.OnArmedPartitionSuppressed,
		h.OnArmedPartition,
		h.OnPartitionArmedInMode2,
		h.OnPartitionArmedInMode3,
		h.OnPartitionWith1stCodeEntered,
		h.OnPartitionEntryTime,
		h.OnPartitionExitTimeOver10s,
		h.OnPartitionExitTimeUnder10s,
		h.OnPartitionTemporaryBlocked,
		h.OnPartitionBlockedForGuardRound,
		h.OnPartitionAlarm,
		h.OnPartitionFireAlarm,
		h.OnPartitionAlarmMemory,
		h.OnPartitionFireAlarmMemory,
		h.OnOutput,
		h.OnDoorOpened,
		h.OnDoorOpenedLong,
		h.OnStatusBit,
		h.OnTroublePart1,
		h.OnTroublePart2,
		h.OnTroublePart3,
		h.OnTroublePart4,
		h.OnTroublePart5,
		h.OnTroubleMemoryPart1,
		h.OnTroubleMemoryPart2,
		h.OnTroubleMemoryPart3,
		h.OnTroubleMemoryPart4,
		h.OnTroubleMemoryPart5,
		h.OnPartitionWithViolatedZones,
		h.OnZoneIsolate,
	}

	return functions[cmd]
}
