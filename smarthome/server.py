import http.server
import json
import os
import socketserver
import sys

from dqsmart_gateway import (
    DEVICES,
    SCENES,
    controlDevice,
    readAllStates,
    triggerScene,
)

SERVER_PORT = int(os.environ.get("SMARTHOME_PORT", "8088"))
WEB_DIRECTORY = os.path.join(os.path.dirname(os.path.abspath(__file__)), "web")


class SmarthomeRequestHandler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=WEB_DIRECTORY, **kwargs)

    def sendJsonResponse(self, responsePayload, statusCode=200):
        binaryPayload = json.dumps(responsePayload, ensure_ascii=False).encode("utf-8")
        self.send_response(statusCode)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type")
        self.send_header("Content-Length", str(len(binaryPayload)))
        self.end_headers()
        self.wfile.write(binaryPayload)

    def do_OPTIONS(self):
        self.sendJsonResponse({"ok": True})

    def do_GET(self):
        if self.path.startswith("/api/devices"):
            try:
                deviceStates = readAllStates()
                activeCount = sum(1 for dev in deviceStates if dev["state"] == "on")
                self.sendJsonResponse({
                    "success": True,
                    "account": "U12201@dienquangsmart.vn",
                    "owner": "Chung Nguyễn",
                    "on_count": activeCount,
                    "total": len(deviceStates),
                    "scenes": SCENES,
                    "devices": deviceStates,
                })
            except Exception as failureException:
                self.sendJsonResponse(
                    {"success": False, "error": str(failureException)},
                    statusCode=500,
                )
        else:
            super().do_GET()

    def do_POST(self):
        contentLength = int(self.headers.get("Content-Length", 0))
        requestBody = (
            self.rfile.read(contentLength).decode("utf-8")
            if contentLength > 0
            else "{}"
        )

        try:
            parsedData = json.loads(requestBody) if requestBody else {}

            if self.path == "/api/control":
                deviceId = parsedData.get("id")
                targetState = parsedData.get("state", "toggle")
                updatedDevices = controlDevice(deviceId, targetState)
                activeCount = sum(1 for dev in updatedDevices if dev["state"] == "on")
                self.sendJsonResponse({
                    "success": True,
                    "on_count": activeCount,
                    "devices": updatedDevices,
                })
            elif self.path == "/api/scene":
                sceneId = parsedData.get("id", "tat_dien_khu_sinh_hoat_chung")
                updatedDevices = triggerScene(sceneId)
                activeCount = sum(1 for dev in updatedDevices if dev["state"] == "on")
                self.sendJsonResponse({
                    "success": True,
                    "on_count": activeCount,
                    "devices": updatedDevices,
                })
            else:
                self.sendJsonResponse(
                    {"success": False, "error": "Not found"},
                    statusCode=404,
                )
        except Exception as failureException:
            self.sendJsonResponse(
                {"success": False, "error": str(failureException)},
                statusCode=500,
            )


def runServer():
    socketserver.ThreadingTCPServer.allow_reuse_address = True
    with socketserver.ThreadingTCPServer(("0.0.0.0", SERVER_PORT), SmarthomeRequestHandler) as serverInstance:
        sys.stderr.write(f"Smarthome server running at http://0.0.0.0:{SERVER_PORT}\n")
        serverInstance.serve_forever()


if __name__ == "__main__":
    runServer()
