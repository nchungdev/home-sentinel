from dataclasses import dataclass, field
from typing import Any, List, Optional, Tuple
try:
    import numpy
except ImportError:
    numpy = None
from posture import PostureClassifier, POSTURE_UNKNOWN

Point = Tuple[float, float]
BoundingBox = Tuple[float, float, float, float]

CLASS_PERSON = 0
CLASS_CAT = 15
CLASS_DOG = 16
CLASS_LAPTOP = 63
CLASS_PHONE = 67
CLASS_BOOK = 73

TRACKED_CLASSES = [
    CLASS_PERSON,
    CLASS_CAT,
    CLASS_DOG,
    CLASS_LAPTOP,
    CLASS_PHONE,
    CLASS_BOOK
]

@dataclass
class DetectedPerson:
    footPosition: Point
    normalizedBox: BoundingBox
    aspectRatio: float
    posture: str
    activeItems: List[str] = field(default_factory=list)

@dataclass
class DetectedPet:
    petType: str
    confidence: float
    footPosition: Point
    normalizedBox: BoundingBox

@dataclass
class FrameDetectionResult:
    persons: List[DetectedPerson]
    pets: List[DetectedPet]

class PersonDetector:
    def __init__(
        self,
        modelPath: str = "yolov8n.pt",
        confidenceThreshold: float = 0.5
    ):
        try:
            from ultralytics import YOLO
            self.model = YOLO(modelPath)
        except ImportError:
            self.model = None
        self.confidenceThreshold = confidenceThreshold
        self.postureClassifier = PostureClassifier()

    def detectFrame(self, frameImage: Any) -> FrameDetectionResult:
        if self.model is None or frameImage is None:
            return FrameDetectionResult(persons=[], pets=[])
        frameHeight, frameWidth = frameImage.shape[:2]
        predictionResults = self.model.predict(
            source=frameImage,
            conf=self.confidenceThreshold,
            classes=TRACKED_CLASSES,
            verbose=False
        )

        rawPersons = []
        rawPets = []
        rawItems = []

        for result in predictionResults:
            for boundingBox in result.boxes:
                classId = int(boundingBox.cls[0].item())
                confidenceScore = float(boundingBox.conf[0].item())
                leftX, topY, rightX, bottomY = boundingBox.xyxy[0].tolist()

                clampedBox = (
                    max(0.0, leftX / frameWidth),
                    max(0.0, topY / frameHeight),
                    min(1.0, rightX / frameWidth),
                    min(1.0, bottomY / frameHeight)
                )
                bottomCenter = (
                    max(0.0, min(1.0, ((leftX + rightX) / 2.0) / frameWidth)),
                    max(0.0, min(1.0, bottomY / frameHeight))
                )

                if classId == CLASS_PERSON:
                    pixelW = rightX - leftX
                    pixelH = bottomY - topY
                    ratio = pixelW / float(pixelH) if pixelH > 0 else 0.0
                    posture = self.postureClassifier.classifyByBoundingBox(pixelW, pixelH)
                    rawPersons.append({
                        "foot": bottomCenter,
                        "box": clampedBox,
                        "ratio": round(ratio, 2),
                        "posture": posture,
                        "items": []
                    })
                elif classId in (CLASS_CAT, CLASS_DOG):
                    petName = "cat" if classId == CLASS_CAT else "dog"
                    rawPets.append(DetectedPet(
                        petType=petName,
                        confidence=round(confidenceScore, 2),
                        footPosition=bottomCenter,
                        normalizedBox=clampedBox
                    ))
                elif classId == CLASS_PHONE:
                    rawItems.append(("phone", clampedBox))
                elif classId == CLASS_LAPTOP:
                    rawItems.append(("laptop", clampedBox))
                elif classId == CLASS_BOOK:
                    rawItems.append(("book", clampedBox))

        detectedPersons = []
        for personData in rawPersons:
            personBox = personData["box"]
            associatedItems = []
            for itemName, itemBox in rawItems:
                if self._isOverlapping(personBox, itemBox):
                    associatedItems.append(itemName)

            detectedPersons.append(DetectedPerson(
                footPosition=personData["foot"],
                normalizedBox=personBox,
                aspectRatio=personData["ratio"],
                posture=personData["posture"],
                activeItems=associatedItems
            ))

        return FrameDetectionResult(persons=detectedPersons, pets=rawPets)

    def detectPersons(self, frameImage: Any) -> List[DetectedPerson]:
        result = self.detectFrame(frameImage)
        return result.persons

    def detectPersonFootPositions(self, frameImage: Any) -> List[Point]:
        persons = self.detectPersons(frameImage)
        return [person.footPosition for person in persons]

    def _isOverlapping(self, boxA: BoundingBox, boxB: BoundingBox) -> bool:
        aLeft, aTop, aRight, aBottom = boxA
        bLeft, bTop, bRight, bBottom = boxB
        noOverlap = (
            aRight < bLeft or
            aLeft > bRight or
            aBottom < bTop or
            aTop > bBottom
        )
        return not noOverlap
