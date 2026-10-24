import time
from typing import Callable, Dict, List, Optional
from zone import Point, Zone

EVENT_ENTER = "enter"
EVENT_LEAVE = "leave"

class PresenceTracker:
    def __init__(
        self,
        monitoredZones: List[Zone],
        onPresenceEvent: Callable[[str, str], None]
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

    def processDetectedPoints(self, detectedPoints: List[Point]):
        currentTimestamp = time.time()

        for zone in self.monitoredZones:
            identifier = zone.zoneIdentifier
            isPointInZone = any(
                zone.containsPoint(point) for point in detectedPoints
            )
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
                            self.onPresenceEvent(identifier, EVENT_ENTER)
                else:
                    self.firstSeenTimestamps[identifier] = None

            else:
                if isPointInZone:
                    self.lastSeenTimestamps[identifier] = currentTimestamp
                else:
                    lastSeen = self.lastSeenTimestamps[identifier]
                    if lastSeen is not None:
                        elapsed = currentTimestamp - lastSeen
                        if elapsed >= zone.leaveDebounceSeconds:
                            self.zoneOccupiedStates[identifier] = False
                            self.lastSeenTimestamps[identifier] = None
                            self.onPresenceEvent(identifier, EVENT_LEAVE)
