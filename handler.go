package satel

type Handler interface {
	OnZoneViolations(index int, state bool)
	OnZoneTamper(index int, state bool)
	OnZoneAlarm(index int, state bool)
	OnZoneTamperAlarm(index int, state bool)
	OnZoneAlarmMemory(index int, state bool)
	OnZoneTamperAlarmMemory(index int, state bool)
	OnZoneBypass(index int, state bool)
	OnZoneNoViolationTrouble(index int, state bool)
	OnZoneLongViolationTrouble(index int, state bool)
	OnArmedPartitionSuppressed(index int, state bool)
	OnArmedPartition(index int, state bool)
	OnPartitionArmedInMode2(index int, state bool)
	OnPartitionArmedInMode3(index int, state bool)
	OnPartitionWith1stCodeEntered(index int, state bool)
	OnPartitionEntryTime(index int, state bool)
	OnPartitionExitTimeOver10s(index int, state bool)
	OnPartitionExitTimeUnder10s(index int, state bool)
	OnPartitionTemporaryBlocked(index int, state bool)
	OnPartitionBlockedForGuardRound(index int, state bool)
	OnPartitionAlarm(index int, state bool)
	OnPartitionFireAlarm(index int, state bool)
	OnPartitionAlarmMemory(index int, state bool)
	OnPartitionFireAlarmMemory(index int, state bool)
	OnOutput(index int, state bool)
	OnDoorOpened(index int, state bool)
	OnDoorOpenedLong(index int, state bool)
	OnStatusBit(index int, state bool)
	OnTroublePart1(index int, state bool)
	OnTroublePart2(index int, state bool)
	OnTroublePart3(index int, state bool)
	OnTroublePart4(index int, state bool)
	OnTroublePart5(index int, state bool)
	OnTroubleMemoryPart1(index int, state bool)
	OnTroubleMemoryPart2(index int, state bool)
	OnTroubleMemoryPart3(index int, state bool)
	OnTroubleMemoryPart4(index int, state bool)
	OnTroubleMemoryPart5(index int, state bool)
	OnPartitionWithViolatedZones(index int, state bool)
	OnZoneIsolate(index int, state bool)
}

func handlerFunc(h Handler, cmd ChangeType) func(int, bool) {
	functions := [...]func(int, bool){
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
