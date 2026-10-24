import os
import sys
import time
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from posture import (
    PostureClassifier,
    POSTURE_STANDING,
    POSTURE_SITTING,
    POSTURE_LYING
)
from presence import PresenceTracker, EVENT_ENTER, EVENT_LEAVE
from zone import Zone

class PostureClassifierTest(unittest.TestCase):
    def setUp(self):
        self.classifier = PostureClassifier()

    def testStandingPosture(self):
        posture = self.classifier.classifyByBoundingBox(boxWidth=60.0, boxHeight=180.0)
        self.assertEqual(posture, POSTURE_STANDING)

    def testSittingPosture(self):
        posture = self.classifier.classifyByBoundingBox(boxWidth=90.0, boxHeight=110.0)
        self.assertEqual(posture, POSTURE_SITTING)

    def testLyingPosture(self):
        posture = self.classifier.classifyByBoundingBox(boxWidth=200.0, boxHeight=80.0)
        self.assertEqual(posture, POSTURE_LYING)

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
    def testEnterAndLeaveDebounceWithMetadata(self):
        emittedEvents = []

        def captureEvent(zoneId: str, eventType: str, metadata: dict = None):
            emittedEvents.append((zoneId, eventType, metadata))

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
        self.assertEqual(emittedEvents[0][0], "sofa")
        self.assertEqual(emittedEvents[0][1], EVENT_ENTER)
        self.assertEqual(emittedEvents[0][2]["headcount"], 1)

        tracker.processDetectedPoints([])
        self.assertEqual(len(emittedEvents), 1)

        time.sleep(0.06)
        tracker.processDetectedPoints([])
        self.assertEqual(len(emittedEvents), 2)
        self.assertEqual(emittedEvents[1][0], "sofa")
        self.assertEqual(emittedEvents[1][1], EVENT_LEAVE)
        self.assertEqual(emittedEvents[1][2]["headcount"], 0)

class DetectorItemAssociationTest(unittest.TestCase):
    def testBoxOverlapDetection(self):
        from detector import PersonDetector
        detector = PersonDetector.__new__(PersonDetector)
        personBox = (0.2, 0.2, 0.6, 0.8)
        phoneBox = (0.3, 0.4, 0.4, 0.5)
        awayBox = (0.7, 0.1, 0.8, 0.2)
        self.assertTrue(detector._isOverlapping(personBox, phoneBox))
        self.assertFalse(detector._isOverlapping(personBox, awayBox))

if __name__ == "__main__":
    unittest.main()
