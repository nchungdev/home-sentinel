import os
import unicodedata


class HomeAssistantAdapter:
    SUPPORTED_DOMAINS = {'light', 'switch', 'fan', 'climate', 'cover'}

    def __init__(self, requester=None):
        self._requester = requester

    def _request(self, path, body=None):
        if self._requester is not None:
            return self._requester(path, body)
        import home_tools
        return home_tools.ha_request(path, body)

    @staticmethod
    def normalize(text):
        if not text:
            return ''
        text = text.lower().replace('đ', 'd')
        return ''.join(c for c in unicodedata.normalize('NFD', text)
                       if unicodedata.category(c) != 'Mn').strip()

    def execute_step(self, step_id, action_type, target_id, parameters=None, is_idempotent=False):
        parameters = parameters or {}
        if not target_id or '.' not in target_id:
            return {
                'step_id': step_id,
                'status': 'failed',
                'error_code': 'INVALID_ENTITY',
                'detail': 'ID thực thể không hợp lệ.',
                'can_retry': False,
            }

        domain = target_id.split('.')[0]
        if domain not in self.SUPPORTED_DOMAINS:
            return {
                'step_id': step_id,
                'status': 'failed',
                'error_code': 'UNSUPPORTED_DOMAIN',
                'detail': f'Domain {domain} không được hỗ trợ.',
                'can_retry': False,
            }

        blocked = [e.strip() for e in os.environ.get('AGENT_HA_PROTECTED_ENTITIES', '').split(',') if e.strip()]
        if target_id in blocked:
            return {
                'step_id': step_id,
                'status': 'failed',
                'error_code': 'PROTECTED_ENTITY',
                'detail': 'Thiết bị nguồn trọng yếu được bảo vệ, Agent không điều khiển mục này.',
                'can_retry': False,
            }

        try:
            current = self._request(f'/api/states/{target_id}')
        except Exception as error:
            return {
                'step_id': step_id,
                'status': 'failed',
                'error_code': 'STATE_QUERY_ERROR',
                'detail': str(error),
                'can_retry': True,
            }

        state_val = current.get('state', '')
        if state_val in ('unavailable', 'unknown'):
            return {
                'step_id': step_id,
                'status': 'failed',
                'error_code': 'ENTITY_UNAVAILABLE',
                'detail': 'Thiết bị hiện không khả dụng, chưa gửi lệnh.',
                'can_retry': True,
            }

        attrs = current.get('attributes', {})
        service_data = {'entity_id': target_id}

        if 'temperature' in parameters:
            if domain != 'climate':
                return {
                    'step_id': step_id,
                    'status': 'failed',
                    'error_code': 'INVALID_DOMAIN_FOR_TEMP',
                    'detail': 'Chỉ điều hòa hỗ trợ lệnh đặt nhiệt độ này.',
                    'can_retry': False,
                }
            val = float(parameters['temperature'])
            min_temp = attrs.get('min_temp', 7.0)
            max_temp = attrs.get('max_temp', 35.0)
            if not min_temp <= val <= max_temp:
                return {
                    'step_id': step_id,
                    'status': 'failed',
                    'error_code': 'TEMP_OUT_OF_RANGE',
                    'detail': 'Nhiệt độ nằm ngoài giới hạn thiết bị.',
                    'can_retry': False,
                }
            service = 'set_temperature'
            service_data['temperature'] = val
        elif domain == 'cover':
            cmd = self.normalize(parameters.get('command', action_type))
            service_map = {
                'open': 'open_cover', 'mo': 'open_cover', 'bat': 'open_cover',
                'close': 'close_cover', 'dong': 'close_cover', 'tat': 'close_cover',
                'stop': 'stop_cover', 'dung': 'stop_cover',
            }
            service = service_map.get(cmd)
            if not service:
                return {
                    'step_id': step_id,
                    'status': 'failed',
                    'error_code': 'UNSUPPORTED_COVER_COMMAND',
                    'detail': f'Lệnh rèm không được hỗ trợ: {cmd}',
                    'can_retry': False,
                }
        else:
            cmd = self.normalize(parameters.get('command', action_type))
            if cmd in ('tat', 'off', 'turn_off', 'dong'):
                service = 'turn_off'
            elif cmd in ('bat', 'on', 'turn_on', 'mo'):
                service = 'turn_on'
            else:
                return {
                    'step_id': step_id,
                    'status': 'failed',
                    'error_code': 'UNSUPPORTED_COMMAND',
                    'detail': f'Lệnh này không phù hợp với thiết bị: {cmd}',
                    'can_retry': False,
                }

        if is_idempotent:
            if service == 'turn_on' and state_val == 'on':
                return {'step_id': step_id, 'status': 'succeeded', 'actual_state': current}
            if service == 'turn_off' and state_val == 'off':
                return {'step_id': step_id, 'status': 'succeeded', 'actual_state': current}

        try:
            self._request(f'/api/services/{domain}/{service}', service_data)
            updated = self._request(f'/api/states/{target_id}')
        except Exception as error:
            return {
                'step_id': step_id,
                'status': 'failed',
                'error_code': 'SERVICE_CALL_ERROR',
                'detail': str(error),
                'can_retry': True,
            }

        name = attrs.get('friendly_name', target_id)
        detail = (f'HA đã nhận lệnh cho {name}. Trạng thái đọc lại: '
                  f'{updated.get("state", "unknown")}; '
                  f'nhiệt độ đặt: {updated.get("attributes", {}).get("temperature", "—")}.')
        return {
            'step_id': step_id,
            'status': 'succeeded',
            'detail': detail,
            'actual_state': updated,
            'can_retry': False,
        }
