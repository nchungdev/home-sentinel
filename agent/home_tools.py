import json
import os
import re
import shutil
import socket
import subprocess
import unicodedata
import urllib.error
import urllib.request


def normalized(value):
    value = value.lower().replace('đ', 'd')
    return ''.join(c for c in unicodedata.normalize('NFD', value)
                   if unicodedata.category(c) != 'Mn').strip()


def request_json(base, path, token='', body=None):
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    request = urllib.request.Request(
        base.rstrip('/') + path,
        data=None if body is None else json.dumps(body).encode(), headers=headers)
    opener = urllib.request.build_opener(NoRedirect)
    with opener.open(request, timeout=8) as response:
        return json.load(response)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, new_url):
        return None


def ha_request(path, body=None):
    base = os.environ.get('AGENT_HA_URL', '')
    token = os.environ.get('AGENT_HA_TOKEN', '')
    if not base or not token:
        raise ValueError('Chưa cấu hình kết nối Home Assistant cho Agent.')
    return request_json(base, path, token, body)


def nas_status():
    disks = []
    for path in ['/', '/srv/mergerfs/MainPool']:
        if os.path.exists(path):
            usage = shutil.disk_usage(path)
            disks.append(f'{path}: trống {usage.free / 2**30:.1f} GB')
    load = os.getloadavg()[0]
    try:
        result = subprocess.run(
            ['docker', 'ps', '--format', '{{.Names}}: {{.Status}}'],
            capture_output=True, text=True, timeout=5, check=True)
        containers = result.stdout.strip()
    except (OSError, subprocess.SubprocessError):
        containers = 'Không đọc được trạng thái Docker.'
    return f'NAS {socket.gethostname()}, load 1 phút {load:.2f}.\n' + '\n'.join(
        disks + [containers])


def ha_command(text):
    clean = normalized(text)
    match = re.fullmatch(r'(bat|tat|mo|dong|dung)\s+(.+)', clean)
    temperature = re.fullmatch(r'(?:dat|chinh)\s+(.+?)\s+(\d+(?:\.\d+)?)\s*(?:do|°c)?', clean)
    if not match and not temperature:
        return None
    verb, target = match.groups() if match else ('temperature', temperature.group(1))
    states = ha_request('/api/states')
    domains = {'light', 'switch', 'fan', 'climate', 'cover'}
    candidates = []
    for state in states:
        entity = state['entity_id']
        if entity.split('.')[0] not in domains:
            continue
        label = state.get('attributes', {}).get('friendly_name', entity)
        if target == normalized(entity) or target == normalized(label):
            candidates = [state]
            break
        if target in normalized(label):
            candidates.append(state)
    if not candidates:
        return 'Không tìm thấy thiết bị HA khớp tên này.'
    if len(candidates) != 1:
        names = [s.get('attributes', {}).get('friendly_name', s['entity_id'])
                 for s in candidates[:8]]
        return 'Có nhiều thiết bị khớp. Hãy nói tên đầy đủ: ' + '; '.join(names)
    state = candidates[0]
    entity = state['entity_id']
    if state['state'] in ('unavailable', 'unknown'):
        return 'Thiết bị hiện không khả dụng, chưa gửi lệnh.'
    blocked = os.environ.get('AGENT_HA_PROTECTED_ENTITIES', '').split(',')
    if entity in blocked:
        return 'Thiết bị nguồn trọng yếu được bảo vệ, Agent không điều khiển mục này.'
    domain = entity.split('.')[0]
    data = {'entity_id': entity}
    if temperature:
        if domain != 'climate':
            return 'Chỉ điều hòa hỗ trợ lệnh đặt nhiệt độ này.'
        value = float(temperature.group(2))
        attrs = state.get('attributes', {})
        if not attrs.get('min_temp', 7) <= value <= attrs.get('max_temp', 35):
            return 'Nhiệt độ nằm ngoài giới hạn thiết bị.'
        service = 'set_temperature'
        data['temperature'] = value
    elif domain == 'cover':
        service = {'mo': 'open_cover', 'dong': 'close_cover', 'dung': 'stop_cover',
                   'bat': 'open_cover', 'tat': 'close_cover'}[verb]
    elif verb in ('bat', 'mo', 'tat'):
        service = 'turn_off' if verb == 'tat' else 'turn_on'
    else:
        return 'Lệnh này không phù hợp với thiết bị.'
    ha_request(f'/api/services/{domain}/{service}', data)
    updated = ha_request('/api/states/' + entity)
    name = state.get('attributes', {}).get('friendly_name', entity)
    return (f'HA đã nhận lệnh cho {name}. Trạng thái đọc lại: '
            f'{updated.get("state", "unknown")}; '
            f'nhiệt độ đặt: {updated.get("attributes", {}).get("temperature", "—")}.')


def camera_command(text):
    clean = normalized(text)
    if 'camera' not in clean:
        return None
    base = os.environ.get('AGENT_CAMERA_URL', '')
    if not base:
        return 'Chưa cấu hình Camera service cho Agent.'
    if clean in ('trang thai camera', 'kiem tra camera'):
        result = request_json(base, '/api/security/status')
        return 'Camera service phản hồi: ' + json.dumps(result, ensure_ascii=False)
    password = os.environ.get('AGENT_CAMERA_PASSWORD', '')
    if not password:
        return 'Chưa cấu hình mật khẩu Camera trên máy chủ Agent.'
    body = {'ip': os.environ.get('AGENT_CAMERA_IP', '192.168.1.6'),
            'user': os.environ.get('AGENT_CAMERA_USER', 'admin'), 'password': password}
    move = re.fullmatch(r'(?:quay|xoay) camera (trai|phai|len|xuong)', clean)
    privacy = re.fullmatch(r'(bat|tat) (?:che do )?rieng tu camera', clean)
    speak = re.fullmatch(r'(?:noi|phat loa) qua camera\s*:?\s*(.+)', text, re.I)
    if move:
        body.update(direction={'trai': 'left', 'phai': 'right', 'len': 'up',
                               'xuong': 'down'}[move.group(1)], step=10)
        path = '/api/camera/ptz'
    elif privacy:
        body['enabled'] = privacy.group(1) == 'bat'
        path = '/api/camera/privacy'
    elif speak:
        body.update(text=speak.group(1)[:500], speed=1.0)
        path = '/api/camera/speak'
    else:
        return 'Hỗ trợ: trạng thái camera, quay camera trái/phải/lên/xuống, ' + \
            'bật/tắt riêng tư camera, nói qua camera: nội dung.'
    result = request_json(base, path, body=body)
    if not result.get('ok'):
        return 'Camera từ chối hoặc không thực hiện được lệnh.'
    return result.get('message', 'Camera API đã nhận lệnh.')


def handle(text):
    clean = normalized(text)
    try:
        if clean in ('trang thai nas', 'kiem tra nas', 'nas con bao nhieu dung luong'):
            return nas_status()
        if 'camera' in clean:
            return camera_command(text)
        if re.match(r'^(bat|tat|mo|dong|dung|dat|chinh)\s+', clean):
            return ha_command(text)
        if clean in ('trang thai nha', 'danh sach thiet bi ha'):
            states = ha_request('/api/states')
            rows = [s for s in states if s['entity_id'].split('.')[0] in
                    {'light', 'switch', 'fan', 'climate', 'cover'}]
            return '\n'.join(f'{s.get("attributes", {}).get("friendly_name", s["entity_id"])}: '
                             f'{s["state"]}' for s in rows[:60])
    except (OSError, ValueError, urllib.error.URLError, subprocess.SubprocessError):
        return 'Không thực hiện được công cụ. Hãy kiểm tra kết nối và cấu hình máy chủ.'
    return None
