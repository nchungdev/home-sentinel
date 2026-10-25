import json
import logging
import os
import signal
import sys
import time
from dispatcher import EventDispatcher
from detector import PersonDetector
from motion import MotionPreFilter
from presence import PresenceTracker, EVENT_ENTER
from snapshot import SnapshotManager
from stream import VideoStreamReader
from zone import Zone

CONFIG_FILE_PATH = os.environ.get("CAMERA_CONFIG", "camera_config.json")
DEFAULT_LOOP_INTERVAL_SECONDS = 0.1

def loadConfiguration(filePath: str) -> dict:
    if not os.path.exists(filePath):
        sys.stderr.write(f"Configuration file not found: {filePath}\n")
        sys.exit(1)

    with open(filePath, "r", encoding="utf-8") as configFile:
        return json.load(configFile)

def createZoneInstances(zonesConfig: list) -> list:
    zoneInstances = []
    for zoneData in zonesConfig:
        zone = Zone(
            zoneIdentifier=zoneData["id"],
            polygonCoordinates=[tuple(point) for point in zoneData["polygon"]],
            enterDebounceSeconds=zoneData.get("enter_debounce_seconds", 3.0),
            leaveDebounceSeconds=zoneData.get("leave_debounce_seconds", 60.0)
        )
        zoneInstances.append(zone)

    return zoneInstances

def main():
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(levelname)s] %(message)s"
    )

    configuration = loadConfiguration(CONFIG_FILE_PATH)
    streamUrl = configuration["stream_url"]
    modelPath = configuration.get("model", "yolov8n.pt")
    confidence = float(configuration.get("confidence_threshold", 0.5))
    webhookUrl = configuration["webhook_url"]
    privacyBlur = bool(configuration.get("privacy_blur", False))

    zones = createZoneInstances(configuration.get("zones", []))
    dispatcher = EventDispatcher(webhookUrl)
    snapshotManager = SnapshotManager(enableFacePrivacyBlur=privacyBlur)
    motionFilter = MotionPreFilter()
    detector = PersonDetector(modelPath, confidence)
    reader = VideoStreamReader(streamUrl)

    latestCapturedFrame = None

    def onPresence(zoneIdentifier: str, eventType: str, metadata: dict = None):
        if eventType == EVENT_ENTER and latestCapturedFrame is not None:
            snapshotPath = snapshotManager.captureAndStore(
                latestCapturedFrame,
                zoneIdentifier
            )
            if metadata is not None:
                metadata["snapshot_path"] = snapshotPath

        dispatcher.dispatchEvent(zoneIdentifier, eventType, metadata)

    tracker = PresenceTracker(zones, onPresence)
    shouldTerminate = False

    def handleSignal(signalNumber, frameObject):
        nonlocal shouldTerminate
        shouldTerminate = True

    signal.signal(signal.SIGINT, handleSignal)
    signal.signal(signal.SIGTERM, handleSignal)

    logging.info("Starting video stream reader for %s", streamUrl)
    reader.start()

    try:
        while not shouldTerminate:
            frame = reader.readLatestFrame()
            if frame is not None:
                latestCapturedFrame = frame
                hasMotion = motionFilter.evaluateMotion(frame)
                if hasMotion:
                    detectedPersons = detector.detectPersons(frame)
                    tracker.processDetectedPoints(detectedPersons)
                else:
                    tracker.processDetectedPoints([])

            time.sleep(DEFAULT_LOOP_INTERVAL_SECONDS)
    finally:
        logging.info("Stopping video stream reader...")
        reader.stop()

if __name__ == "__main__":
    main()
