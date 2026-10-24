import json
import os
import struct
import subprocess
import threading
import time

TV_SERIAL = os.environ.get("TV_HOST", "192.168.1.8:5555")

SWITCH_GROUPS = [
    {
        "id": "ct_pn_nho",
        "name": "Công tắc Phòng ngủ nhỏ",
        "room": "Phòng ngủ nhỏ",
        "start_x": 14,
        "gangs": [
            {"id": "ct_pn_nho_1", "name": "PN Nhỏ - Nút 1", "gang": 1},
            {"id": "ct_pn_nho_2", "name": "PN Nhỏ - Nút 2", "gang": 2},
            {"id": "ct_pn_nho_3", "name": "PN Nhỏ - Nút 3", "gang": 3},
            {"id": "ct_pn_nho_4", "name": "PN Nhỏ - Nút 4", "gang": 4},
        ],
    },
    {
        "id": "ct_cua_chinh",
        "name": "Công tắc Cửa chính",
        "room": "Cửa chính & Khách",
        "start_x": 250,
        "gangs": [
            {"id": "ct_cua_chinh_1", "name": "Cửa chính - Nút 1 (Gateway/Nguồn)", "gang": 1, "is_critical": True},
            {"id": "ct_cua_chinh_2", "name": "Cửa chính - Nút 2", "gang": 2},
            {"id": "ct_cua_chinh_3", "name": "Cửa chính - Nút 3", "gang": 3},
            {"id": "ct_cua_chinh_4", "name": "Cửa chính - Nút 4", "gang": 4},
        ],
    },
    {
        "id": "ct_pk_1",
        "name": "Công tắc Phòng khách 1",
        "room": "Phòng khách",
        "start_x": 486,
        "gangs": [
            {"id": "ct_pk_1_1", "name": "Phòng khách 1 - Nút 1", "gang": 1},
            {"id": "ct_pk_1_2", "name": "Phòng khách 1 - Nút 2", "gang": 2},
            {"id": "ct_pk_1_3", "name": "Phòng khách 1 - Nút 3", "gang": 3},
            {"id": "ct_pk_1_4", "name": "Phòng khách 1 - Nút 4", "gang": 4},
        ],
    },
    {
        "id": "ct_pn_chinh",
        "name": "Công tắc Phòng ngủ chính",
        "room": "Phòng ngủ chính",
        "start_x": 722,
        "gangs": [
            {"id": "ct_pn_chinh_1", "name": "PN Chính - Nút 1", "gang": 1},
            {"id": "ct_pn_chinh_2", "name": "PN Chính - Nút 2", "gang": 2},
            {"id": "ct_pn_chinh_3", "name": "PN Chính - Nút 3", "gang": 3},
            {"id": "ct_pn_chinh_4", "name": "PN Chính - Nút 4", "gang": 4},
        ],
    },
    {
        "id": "ct_pk_2",
        "name": "Công tắc Phòng khách 2",
        "room": "Phòng khách",
        "start_x": 958,
        "gangs": [
            {"id": "ct_pk_2_1", "name": "Phòng khách 2 - Nút 1", "gang": 1},
            {"id": "ct_pk_2_2", "name": "Phòng khách 2 - Nút 2", "gang": 2},
            {"id": "ct_pk_2_3", "name": "Phòng khách 2 - Nút 3", "gang": 3},
            {"id": "ct_pk_2_4", "name": "Phòng khách 2 - Nút 4", "gang": 4},
        ],
    },
]

DEVICES = []
for switchGroup in SWITCH_GROUPS:
    for gangItem in switchGroup["gangs"]:
        buttonX = switchGroup["start_x"] + 47 + (gangItem["gang"] - 1) * 36
        DEVICES.append({
            "id": gangItem["id"],
            "name": gangItem["name"],
            "group": switchGroup["name"],
            "room": switchGroup["room"],
            "x": buttonX,
            "y": 905,
        })

SCENES = [
    {
        "id": "tat_dien_khu_sinh_hoat_chung",
        "name": "Tắt điện khu sinh hoạt chung",
        "x": 223,
        "y": 417,
    },
    {"id": "roi_nha", "name": "Rời nhà", "x": 223, "y": 314},
    {"id": "ve_nha", "name": "Về nhà", "x": 580, "y": 314},
]

deviceLock = threading.Lock()


def executeAdb(arguments, timeoutSeconds=6):
    command = ["adb", "-s", TV_SERIAL] + arguments
    return subprocess.run(
        command,
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        timeout=timeoutSeconds,
    )


def ensureDevicesScreen():
    output = ""
    try:
        process = subprocess.run(
            ["adb", "-s", TV_SERIAL, "shell", "dumpsys window | grep mCurrentFocus"],
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
            timeout=4,
        )
        output = process.stdout
    except Exception:
        pass

    if "com.dienquangsmartv2.app" not in output:
        executeAdb([
            "shell",
            "am", "start",
            "-a", "android.intent.action.MAIN",
            "-c", "android.intent.category.LAUNCHER",
            "-n", "com.dienquangsmartv2.app/.MainActivity",
        ])
        time.sleep(3.0)

    executeAdb(["shell", "input", "tap", "1345", "1040"])
    time.sleep(0.8)


def captureScreenBitmap(bmpPath="/tmp/dq_current.bmp"):
    devicePngPath = "/sdcard/dq_current.png"
    localPngPath = "/tmp/dq_current.png"

    executeAdb(["shell", f"screencap -p {devicePngPath}"])
    executeAdb(["pull", devicePngPath, localPngPath])
    subprocess.run(
        ["sips", "-s", "format", "bmp", localPngPath, "--out", bmpPath],
        check=True,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        timeout=5,
    )


def extractPixelRgb(bitmapData, pixelOffset, targetX, targetY):
    index = pixelOffset + targetY * 1920 * 4 + targetX * 4
    blue, green, red, _ = bitmapData[index:index + 4]
    return red, green, blue


def readAllStates():
    with deviceLock:
        ensureDevicesScreen()
        bmpFilePath = "/tmp/dq_current.bmp"
        captureScreenBitmap(bmpFilePath)

        with open(bmpFilePath, "rb") as bitmapFile:
            binaryData = bitmapFile.read()

        pixelDataOffset = struct.unpack_from("<I", binaryData, 10)[0]

        deviceList = []
        for deviceItem in DEVICES:
            devX, devY = deviceItem["x"], deviceItem["y"]
            hasActiveRed = any(
                extractPixelRgb(binaryData, pixelDataOffset, px, py)[0] > 200
                and extractPixelRgb(binaryData, pixelDataOffset, px, py)[1] < 100
                for px in range(devX - 4, devX + 5)
                for py in range(devY - 4, devY + 5)
            )
            cloned = dict(deviceItem)
            cloned["state"] = "on" if hasActiveRed else "off"
            deviceList.append(cloned)

        return deviceList


def controlDevice(deviceId, targetState):
    targetDevice = next((dev for dev in DEVICES if dev["id"] == deviceId), None)
    if not targetDevice:
        raise ValueError(f"Unknown device id: {deviceId}")

    if targetDevice.get("is_critical") and targetState == "off":
        raise ValueError(f"Thiet bi {deviceId} la nguon Gateway, da chan lenh tat de bao ve he thong.")

    currentStates = readAllStates()
    currentState = next(dev["state"] for dev in currentStates if dev["id"] == deviceId)
    needsInteraction = (
        targetState == "toggle"
        or (targetState == "on" and currentState == "off")
        or (targetState == "off" and currentState == "on")
    )

    if needsInteraction:
        with deviceLock:
            ensureDevicesScreen()
            executeAdb(["shell", "input", "tap", str(targetDevice["x"]), str(targetDevice["y"])])
            time.sleep(1.0)
        currentStates = readAllStates()

    return currentStates


def triggerScene(sceneId):
    if sceneId == "tat_dien_khu_sinh_hoat_chung":
        raise ValueError("Kich ban nay tat nguon Gateway, da chan de bao ve he thong.")

    targetScene = next((sc for sc in SCENES if sc["id"] == sceneId), None)
    if not targetScene:
        raise ValueError(f"Unknown scene id: {sceneId}")

    with deviceLock:
        ensureDevicesScreen()
        executeAdb(["shell", "input", "tap", str(targetScene["x"]), str(targetScene["y"])])
        time.sleep(1.5)

    return readAllStates()
