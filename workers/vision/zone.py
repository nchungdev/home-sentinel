from typing import List, Tuple

Point = Tuple[float, float]
Polygon = List[Point]

class Zone:
    def __init__(
        self,
        zoneIdentifier: str,
        polygonCoordinates: Polygon,
        enterDebounceSeconds: float = 3.0,
        leaveDebounceSeconds: float = 60.0
    ):
        self.zoneIdentifier = zoneIdentifier
        self.polygonCoordinates = polygonCoordinates
        self.enterDebounceSeconds = enterDebounceSeconds
        self.leaveDebounceSeconds = leaveDebounceSeconds

    def containsPoint(self, point: Point) -> bool:
        pointX, pointY = point
        isInside = False
        vertexCount = len(self.polygonCoordinates)

        if vertexCount < 3:
            return False

        firstIndex = 0
        secondIndex = vertexCount - 1
        while firstIndex < vertexCount:
            firstVertexX, firstVertexY = self.polygonCoordinates[firstIndex]
            secondVertexX, secondVertexY = self.polygonCoordinates[secondIndex]

            intersectY = (firstVertexY > pointY) != (secondVertexY > pointY)
            if intersectY:
                deltaX = secondVertexX - firstVertexX
                deltaY = secondVertexY - firstVertexY
                edgeInterceptX = deltaX * (pointY - firstVertexY) / deltaY + firstVertexX
                if pointX < edgeInterceptX:
                    isInside = not isInside

            secondIndex = firstIndex
            firstIndex += 1

        return isInside
