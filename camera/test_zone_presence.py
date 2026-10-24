import os
import sys
import time
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from presence import PresenceTracker, EVENT_ENTER, EVENT_LEAVE
from zone import Zone

class ZoneGeometryTest(unittest.TestCase):
    def testPointInsideSquareZone(self):
        zone = Zone(
            zoneIdentifier="sofa",
            polygonCoordinates=[(0.2, 0.2), (0.8, 0.2), (0.8, 0.8), (0.2, 0.8)]
        )
        self.assertTrue(zone.containsPoint((0.5, 0.5)))
        self.assertFalse(zone.containsPoint((0.1, 0.5)))
        self.assertFalse(zone.containsPoint((0.5, 0.9)))

    def testPointOnTriangleZone(self):
        zone = Zone(
            zoneIdentifier="triangle",
            polygonCoordinates=[(0.0, 0.0), (1.0, 0.0), (0.5, 1.0)]
        )
        self.assertTrue(zone.containsPoint((0.5, 0.2)))
        self.assertFalse(zone.containsPoint((0.1, 0.9)))

class PresenceTrackerTest(unittest.TestCase):
    def testEnterAndLeaveDebounce(self):
        emittedEvents = []

        def captureEvent(zoneId: str, eventType: str):
            emittedEvents.append((zoneId, eventType))

        zone = Zone(
            zoneIdentifier="sofa",
            polygonCoordinates=[(0.2, 0.2), (0.8, 0.2), (0.8, 0.8), (0.2, 0.8)],
            enterDebounceSeconds=0.05,
            leaveDebounceSeconds=0.05
        )

        tracker = PresenceTracker([zone], captureEvent)

        tracker.processDetectedPoints([])
        self.assertEqual(len(emittedEvents), 0)

        tracker.processDetectedPoints([(0.5, 0.5)])
        self.assertEqual(len(emittedEvents), 0)

        time.sleep(0.06)

        tracker.processDetectedPoints([(0.5, 0.5)])
        self.assertEqual(len(emittedEvents), 1)
        self.assertEqual(emittedEvents[0], ("sofa", EVENT_ENTER))

        tracker.processDetectedPoints([])
        self.assertEqual(len(emittedEvents), 1)

        time.sleep(0.06)
        tracker.processDetectedPoints([])
        self.assertEqual(len(emittedEvents), 2)
        self.assertEqual(emittedEvents[1], ("sofa", EVENT_LEAVE))

if __name__ == "__main__":
    unittest.main()
