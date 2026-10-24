import time
from typing import Any, Callable, Dict, List, Optional, Union
from zone import Point, Zone

EVENT_ENTER = "enter"
EVENT_LEAVE = "leave"

class PresenceTracker:
    def __init__(
        self,
        monitoredZones: List[Zone],
        onPresenceEvent: Callable[..., None]
    ):
        self.monitoredZones = monitoredZones
        self.onPresenceEvent = onPresenceEvent
        self.zoneOccupiedStates: Dict[str, bool] = {
            zone.zoneIdentifier: False for zone in monitoredZones
        }
        self.firstSeenTimestamps: Dict[str, Optional[float]] = {
            zone.zoneIdentifier: None for zone in monitoredZones
        }
        self.lastSeenTimestamps: Dict[str, Optional[float]] = {
            zone.zoneIdentifier: None for zone in monitoredZones
        }
        self.zoneHeadcounts: Dict[str, int] = {
            zone.zoneIdentifier: 0 for zone in monitoredZones
        }

    def processDetectedPoints(self, detectedItems: List[Any]):
        currentTimestamp = time.time()

        for zone in self.monitoredZones:
            identifier = zone.zoneIdentifier
            personsInZone = []

            for item in detectedItems:
                footPoint = getattr(item, "footPosition", item)
                if zone.containsPoint(footPoint):
                    personsInZone.append(item)

            headcount = len(personsInZone)
            isPointInZone = headcount > 0
            isCurrentlyOccupied = self.zoneOccupiedStates[identifier]

            if not isCurrentlyOccupied:
                if isPointInZone:
                    if self.firstSeenTimestamps[identifier] is None:
                        self.firstSeenTimestamps[identifier] = currentTimestamp
                    else:
                        elapsed = currentTimestamp - self.firstSeenTimestamps[identifier]
                        if elapsed >= zone.enterDebounceSeconds:
                            self.zoneOccupiedStates[identifier] = True
                            self.lastSeenTimestamps[identifier] = currentTimestamp
                            self.firstSeenTimestamps[identifier] = None
                            self.zoneHeadcounts[identifier] = headcount
                            metadata = self._buildMetadata(personsInZone)
                            self._emitEvent(identifier, EVENT_ENTER, metadata)
                else:
                    self.firstSeenTimestamps[identifier] = None

            else:
                if isPointInZone:
                    self.lastSeenTimestamps[identifier] = currentTimestamp
                    self.zoneHeadcounts[identifier] = headcount
                else:
                    lastSeen = self.lastSeenTimestamps[identifier]
                    if lastSeen is not None:
                        elapsed = currentTimestamp - lastSeen
                        if elapsed >= zone.leaveDebounceSeconds:
                            self.zoneOccupiedStates[identifier] = False
                            self.lastSeenTimestamps[identifier] = None
                            self.zoneHeadcounts[identifier] = 0
                            self._emitEvent(identifier, EVENT_LEAVE, {"headcount": 0})

    def _buildMetadata(self, personsInZone: List[Any]) -> Dict[str, Any]:
        postures = [
            getattr(p, "posture", "unknown") for p in personsInZone
        ]
        primaryPosture = "unknown"
        if "lying" in postures:
            primaryPosture = "lying"
        elif "sitting" in postures:
            primaryPosture = "sitting"
        elif "standing" in postures:
            primaryPosture = "standing"

        return {
            "headcount": len(personsInZone),
            "postures": postures,
            "primary_posture": primaryPosture
        }

    def _emitEvent(
        self,
        zoneIdentifier: str,
        eventType: str,
        metadata: Dict[str, Any]
    ):
        try:
            self.onPresenceEvent(zoneIdentifier, eventType, metadata)
        except TypeError:
            self.onPresenceEvent(zoneIdentifier, eventType)
