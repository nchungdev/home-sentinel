import json
import os


class CameraAdapter:
    def __init__(self, requester=None):
        self._requester = requester

    def _request(self, base, path, body=None):
        if self._requester is not None:
            return self._requester(base, path, body=body)
        import home_tools
        return home_tools.request_json(base, path, body=body)

    def execute_step(self, step_id, action_type, parameters=None):
        parameters = parameters or {}
        base = os.environ.get('AGENT_CAMERA_URL', '')
        if not base:
            return {
                'step_id': step_id,
                'status': 'failed',
                'error_code': 'CONFIG_MISSING_CAMERA_URL',
                'detail': 'Chưa cấu hình Camera service cho Agent.',
                'can_retry': False,
            }

        if action_type == 'camera.status':
            try:
                res = self._request(base, '/api/security/status')
                return {
                    'step_id': step_id,
                    'status': 'succeeded',
                    'detail': 'Camera service phản hồi: ' + json.dumps(res, ensure_ascii=False),
                    'actual_state': res,
                }
            except Exception as e:
                return {
                    'step_id': step_id,
                    'status': 'failed',
                    'error_code': 'CAMERA_STATUS_ERROR',
                    'detail': str(e),
                    'can_retry': True,
                }

        password = os.environ.get('AGENT_CAMERA_PASSWORD', '')
        if not password:
            return {
                'step_id': step_id,
                'status': 'failed',
                'error_code': 'CAMERA_AUTH_MISSING',
                'detail': 'Chưa cấu hình mật khẩu Camera trên máy chủ Agent.',
                'can_retry': False,
            }

        body = {
            'ip': os.environ.get('AGENT_CAMERA_IP', '192.168.1.6'),
            'user': os.environ.get('AGENT_CAMERA_USER', 'admin'),
            'password': password,
        }

        if action_type == 'camera.ptz':
            direction = parameters.get('direction', 'left')
            step_val = min(int(parameters.get('step', 10)), 45)
            body.update(direction=direction, step=step_val)
            path = '/api/camera/ptz'
        elif action_type == 'camera.privacy':
            body['enabled'] = bool(parameters.get('enabled', True))
            path = '/api/camera/privacy'
        elif action_type == 'camera.speak':
            text = str(parameters.get('text', ''))[:500]
            body.update(text=text, speed=1.0)
            path = '/api/camera/speak'
        else:
            return {
                'step_id': step_id,
                'status': 'failed',
                'error_code': 'UNSUPPORTED_CAMERA_ACTION',
                'detail': f'Action {action_type} không được hỗ trợ.',
                'can_retry': False,
            }

        try:
            res = self._request(base, path, body=body)
        except Exception as e:
            return {
                'step_id': step_id,
                'status': 'failed',
                'error_code': 'CAMERA_REQUEST_ERROR',
                'detail': str(e),
                'can_retry': True,
            }

        if not res.get('ok'):
            return {
                'step_id': step_id,
                'status': 'failed',
                'error_code': 'CAMERA_REJECTED',
                'detail': 'Camera từ chối hoặc không thực hiện được lệnh.',
                'can_retry': False,
            }

        msg = res.get('message', 'Camera API đã nhận lệnh.')
        return {
            'step_id': step_id,
            'status': 'succeeded',
            'detail': msg,
            'actual_state': res,
            'can_retry': False,
        }
