import unittest
from unittest.mock import patch

from inference_schedule import InferenceSchedule
from presence import PresenceTracker
from zone import Zone


class InferenceScheduleTest(unittest.TestCase):
    def testStationaryPersonDoesNotBecomeEmpty(self):
        clock = [0.0]
        events = []
        schedule = InferenceSchedule(2.0, clock=lambda: clock[0])
        zone = Zone("sofa", [(0, 0), (1, 0), (1, 1), (0, 1)], 3.0, 5.0)
        tracker = PresenceTracker([zone], lambda *event: events.append(event))
        for tick in range(21):
            clock[0] = float(tick)
            with patch("presence.time.time", return_value=clock[0]):
                if schedule.shouldInfer(hasMotion=tick < 3):
                    tracker.processDetectedPoints([(0.5, 0.5)])
        self.assertTrue(tracker.zoneOccupiedStates["sofa"])
        self.assertEqual([event[1] for event in events], ["enter"])

    def testEmptyInferenceEventuallyEmitsLeave(self):
        clock = [0.0]
        events = []
        schedule = InferenceSchedule(2.0, clock=lambda: clock[0])
        zone = Zone("sofa", [(0, 0), (1, 0), (1, 1), (0, 1)], 1.0, 5.0)
        tracker = PresenceTracker([zone], lambda *event: events.append(event))
        for tick in range(15):
            clock[0] = float(tick)
            with patch("presence.time.time", return_value=clock[0]):
                if schedule.shouldInfer(hasMotion=tick < 3):
                    tracker.processDetectedPoints([(0.5, 0.5)] if tick < 3 else [])
        self.assertFalse(tracker.zoneOccupiedStates["sofa"])
        self.assertEqual([event[1] for event in events], ["enter", "leave"])

    def testDisconnectDoesNotCountAsObservedEmptyTime(self):
        events = []
        zone = Zone("sofa", [(0, 0), (1, 0), (1, 1), (0, 1)], 1.0, 5.0)
        tracker = PresenceTracker([zone], lambda *event: events.append(event))
        with patch("presence.time.time", return_value=0.0):
            tracker.processDetectedPoints([(0.5, 0.5)])
        with patch("presence.time.time", return_value=1.0):
            tracker.processDetectedPoints([(0.5, 0.5)])
        tracker.suspendObservation()
        with patch("presence.time.time", return_value=100.0):
            tracker.processDetectedPoints([])
        self.assertTrue(tracker.zoneOccupiedStates["sofa"])
        with patch("presence.time.time", return_value=105.0):
            tracker.processDetectedPoints([])
        self.assertEqual([event[1] for event in events], ["enter", "leave"])

    def testNoInferenceIsNotAnEmptyObservation(self):
        clock = [0.0]
        schedule = InferenceSchedule(2.0, clock=lambda: clock[0])
        self.assertTrue(schedule.shouldInfer(False))
        clock[0] = 0.5
        self.assertFalse(schedule.shouldInfer(False))
        clock[0] = 2.0
        self.assertTrue(schedule.shouldInfer(False))


if __name__ == "__main__":
    unittest.main()
