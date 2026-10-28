import importlib.util
from pathlib import Path
import sys
from types import SimpleNamespace
import unittest
from unittest.mock import patch


with patch.dict(sys.modules, {"cv2": SimpleNamespace()}):
    specification = importlib.util.spec_from_file_location(
        "stream_under_test", Path(__file__).with_name("stream.py")
    )
    streamModule = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(streamModule)


class Frame:
    def copy(self):
        return "frame-copy"


class StreamFreshnessTest(unittest.TestCase):
    def testFreshFrameIsCopied(self):
        reader = streamModule.VideoStreamReader("fixture")
        reader.latestFrame = Frame()
        reader.lastFrameAt = 10.0
        with patch.object(streamModule.time, "monotonic", return_value=11.0):
            self.assertEqual(reader.readLatestFrame(), "frame-copy")

    def testStaleFrameIsNotReused(self):
        reader = streamModule.VideoStreamReader("fixture")
        reader.latestFrame = Frame()
        reader.lastFrameAt = 10.0
        with patch.object(streamModule.time, "monotonic", return_value=20.0):
            self.assertIsNone(reader.readLatestFrame())

    def testDisconnectClearsPreviousFrame(self):
        reader = streamModule.VideoStreamReader("fixture")
        reader.latestFrame = Frame()
        reader.lastFrameAt = 10.0
        reader.isRunning = True

        def disconnected():
            reader.isRunning = False
            return False, None

        capture = SimpleNamespace(
            set=lambda *args: None, isOpened=lambda: True,
            read=disconnected, release=lambda: None
        )
        streamModule.cv2 = SimpleNamespace(
            VideoCapture=lambda *args: capture, CAP_FFMPEG=0, CAP_PROP_BUFFERSIZE=0
        )
        with patch.object(streamModule.time, "sleep", return_value=None):
            reader._captureLoop()
        self.assertIsNone(reader.latestFrame)
        self.assertIsNone(reader.lastFrameAt)


if __name__ == "__main__":
    unittest.main()
