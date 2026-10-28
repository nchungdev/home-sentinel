import csv
import json
import os
from pathlib import Path
import socket
import struct
import numpy as np
import onnxruntime as ort

WORKER_DIRECTORY = Path(__file__).resolve().parent
MODEL_PATH = os.environ.get("YAMNET_MODEL_PATH", str(WORKER_DIRECTORY / "yamnet.onnx"))
CSV_PATH = os.environ.get("YAMNET_CLASS_MAP_PATH", str(WORKER_DIRECTORY / "yamnet_class_map.csv"))
PORT = 10400

# 1. Load Class Map
class_map = {}
with open(CSV_PATH, mode="r", encoding="utf-8") as f:
    reader = csv.reader(f)
    next(reader)  # skip header: index,mid,display_name
    for row in reader:
        if len(row) >= 3:
            class_map[int(row[0])] = row[2]

# 2. Init ONNX Session
print("Đang tải YAMNet ONNX session...")
sess = ort.InferenceSession(MODEL_PATH, providers=["CPUExecutionProvider"])
input_name = sess.get_inputs()[0].name
output_name = sess.get_outputs()[0].name
print(f"YAMNet đã sẵn sàng trên cổng TCP {PORT}")

# 3. Server Loop
server_sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
server_sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
server_sock.bind(("0.0.0.0", PORT))
server_sock.listen(5)

while True:
    conn, _ = server_sock.accept()
    try:
        # Nhận độ dài payload (4 bytes uint32 big-endian)
        raw_len = conn.recv(4)
        if not raw_len or len(raw_len) < 4:
            conn.close()
            continue
        payload_len = struct.unpack(">I", raw_len)[0]
        
        # Nhận PCM bytes
        buf = bytearray()
        while len(buf) < payload_len:
            packet = conn.recv(payload_len - len(buf))
            if not packet:
                break
            buf.extend(packet)
            
        if len(buf) < payload_len or len(buf) < 16000 * 2 * 0.4:  # tối thiểu 0.4s
            conn.sendall(json.dumps({"error": "âm thanh quá ngắn"}).encode("utf-8") + b"\n")
            conn.close()
            continue

        # Chuyển đổi PCM 16-bit mono 16kHz -> float32 [-1.0, 1.0]
        pcm_arr = np.frombuffer(buf, dtype=np.int16)
        waveform = pcm_arr.astype(np.float32) / 32768.0

        # Chạy inference YAMNet
        scores, _, _ = sess.run(None, {input_name: waveform})
        mean_scores = np.mean(scores, axis=0)
        top5_indices = np.argsort(mean_scores)[::-1][:5]

        results = []
        is_speech = False
        is_alarm = False
        is_clap = False

        for idx in top5_indices:
            label = class_map.get(idx, "Unknown")
            prob = float(mean_scores[idx])
            results.append({"class_id": int(idx), "label": label, "score": round(prob, 4)})
            
            lower_l = label.lower()
            if "speech" in lower_l or "whispering" in lower_l or "shout" in lower_l:
                if prob > 0.15:
                    is_speech = True
            if "fire alarm" in lower_l or "smoke detector" in lower_l or "siren" in lower_l or "alarm" in lower_l:
                if prob > 0.15:
                    is_alarm = True
            if "clapping" in lower_l or "applause" in lower_l or "hands" in lower_l:
                if prob > 0.15:
                    is_clap = True

        top_label = results[0]["label"]
        top_score = results[0]["score"]

        response = {
            "top_label": top_label,
            "top_score": top_score,
            "is_speech": is_speech,
            "is_alarm": is_alarm,
            "is_clap": is_clap,
            "predictions": results
        }

        conn.sendall(json.dumps(response).encode("utf-8") + b"\n")
    except Exception as e:
        try:
            conn.sendall(json.dumps({"error": str(e)}).encode("utf-8") + b"\n")
        except:
            pass
    finally:
        conn.close()
