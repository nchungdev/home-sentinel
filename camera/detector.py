from dataclasses import dataclass
from typing import List, Tuple
from ultralytics import YOLO
import numpy
from posture import PostureClassifier, POSTURE_UNKNOWN

Point = Tuple[float, float]
BoundingBox = Tuple[float, float, float, float]

@dataclass
class DetectedPerson:
    footPosition: Point
    normalizedBox: BoundingBox
    aspectRatio: float
    posture: str

class PersonDetector:
    def __init__(
        self,
        modelPath: str = "yolov8n.pt",
        confidenceThreshold: float = 0.5
    ):
        self.model = YOLO(modelPath)
        self.confidenceThreshold = confidenceThreshold
        self.personClassIndex = 0
        self.postureClassifier = PostureClassifier()

    def detectPersons(self, frameImage: numpy.ndarray) -> List[DetectedPerson]:
        frameHeight, frameWidth = frameImage.shape[:2]
        predictionResults = self.model.predict(
            source=frameImage,
            conf=self.confidenceThreshold,
            classes=[self.personClassIndex],
            verbose=False
        )

        detectedPersons = []
        for result in predictionResults:
            for boundingBox in result.boxes:
                coordinateValues = boundingBox.xyxy[0].tolist()
                leftX, topY, rightX, bottomY = coordinateValues

                boxPixelWidth = rightX - leftX
                boxPixelHeight = bottomY - topY
                aspectRatio = boxPixelWidth / float(boxPixelHeight) if boxPixelHeight > 0 else 0.0
                posture = self.postureClassifier.classifyByBoundingBox(
                    boxPixelWidth,
                    boxPixelHeight
                )

                bottomCenterX = ((leftX + rightX) / 2.0) / frameWidth
                normalizedBottomY = bottomY / frameHeight

                clampedX = max(0.0, min(1.0, bottomCenterX))
                clampedY = max(0.0, min(1.0, normalizedBottomY))

                normalizedBox = (
                    leftX / frameWidth,
                    topY / frameHeight,
                    rightX / frameWidth,
                    bottomY / frameHeight
                )

                detectedPersons.append(DetectedPerson(
                    footPosition=(clampedX, clampedY),
                    normalizedBox=normalizedBox,
                    aspectRatio=round(aspectRatio, 2),
                    posture=posture
                ))

        return detectedPersons

    def detectPersonFootPositions(self, frameImage: numpy.ndarray) -> List[Point]:
        persons = self.detectPersons(frameImage)
        return [person.footPosition for person in persons]
