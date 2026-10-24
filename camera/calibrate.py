import json
import sys
import cv2

CLICKED_POINTS = []

def handleMouseClick(event, xCoordinate, yCoordinate, flags, parameters):
    if event == cv2.EVENT_LBUTTONDOWN:
        frameWidth, frameHeight = parameters
        normalizedX = round(xCoordinate / frameWidth, 3)
        normalizedY = round(yCoordinate / frameHeight, 3)
        CLICKED_POINTS.append([normalizedX, normalizedY])
        print(f"Added point: [{normalizedX}, {normalizedY}]")

def main():
    if len(sys.argv) < 2:
        print("Usage: python3 calibrate.py <rtmp_url_or_video_file>")
        sys.exit(1)

    sourceUrl = sys.argv[1]
    capture = cv2.VideoCapture(sourceUrl)
    hasFrame, frame = capture.read()
    capture.release()

    if not hasFrame:
        print(f"Error: Could not read frame from {sourceUrl}")
        sys.exit(1)

    frameHeight, frameWidth = frame.shape[:2]
    windowName = "Click 4 corners of sofa (Press Q when done)"
    cv2.namedWindow(windowName)
    cv2.setMouseCallback(windowName, handleMouseClick, (frameWidth, frameHeight))

    while True:
        displayFrame = frame.copy()
        if len(CLICKED_POINTS) > 0:
            for point in CLICKED_POINTS:
                pixelX = int(point[0] * frameWidth)
                pixelY = int(point[1] * frameHeight)
                cv2.circle(displayFrame, (pixelX, pixelY), 5, (0, 255, 0), -1)

        cv2.imshow(windowName, displayFrame)
        pressedKey = cv2.waitKey(20) & 0xFF
        if pressedKey == ord("q"):
            break

    cv2.destroyAllWindows()

    print("\nCoordinates for camera_config.json:")
    print(json.dumps(CLICKED_POINTS, indent=2))

if __name__ == "__main__":
    main()
