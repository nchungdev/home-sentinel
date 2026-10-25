import unittest
from unittest.mock import patch
import home_tools
import media_bridge


class TvScenariosTest(unittest.TestCase):
    def test_climate_temperature_control(self):
        state = {
            'entity_id': 'climate.dieu_hoa_phong_khach',
            'state': 'cool',
            'attributes': {
                'friendly_name': 'Điều hòa phòng khách',
                'min_temp': 16.0,
                'max_temp': 30.0,
                'temperature': 26.0
            }
        }
        updated_state = dict(state)
        updated_state['attributes'] = dict(state['attributes'], temperature=24.0)

        with patch.object(home_tools, 'ha_request', side_effect=[[state], [], updated_state]) as request:
            reply = home_tools.ha_command('chỉnh điều hòa phòng khách 24 độ')
            self.assertIn('HA đã nhận', reply)
            self.assertEqual('/api/services/climate/set_temperature', request.call_args_list[1].args[0])
            self.assertEqual(24.0, request.call_args_list[1].args[1]['temperature'])

    def test_climate_temperature_out_of_bounds(self):
        state = {
            'entity_id': 'climate.dieu_hoa_phong_khach',
            'state': 'cool',
            'attributes': {
                'friendly_name': 'Điều hòa phòng khách',
                'min_temp': 16.0,
                'max_temp': 30.0
            }
        }
        with patch.object(home_tools, 'ha_request', return_value=[state]):
            reply = home_tools.ha_command('đặt điều hòa phòng khách 40 độ')
            self.assertIn('ngoài giới hạn', reply)

    def test_cover_curtain_controls(self):
        state = {
            'entity_id': 'cover.rem_phong_khach',
            'state': 'open',
            'attributes': {'friendly_name': 'Rèm phòng khách'}
        }
        with patch.object(home_tools, 'ha_request', side_effect=[[state], [], state]) as request:
            reply = home_tools.ha_command('đóng rèm phòng khách')
            self.assertIn('HA đã nhận', reply)
            self.assertEqual('/api/services/cover/close_cover', request.call_args_list[1].args[0])

    def test_camera_ptz_and_privacy_actions(self):
        with patch.dict('os.environ', {'AGENT_CAMERA_URL': 'http://127.0.0.1:8792',
                                       'AGENT_CAMERA_PASSWORD': 'test_secret'}):
            with patch.object(home_tools, 'request_json', return_value={'ok': True}) as req:
                home_tools.camera_command('quay camera phai')
                self.assertEqual('/api/camera/ptz', req.call_args_list[0].args[1])
                body = req.call_args_list[0].kwargs.get('body', {})
                self.assertEqual('right', body.get('direction'))

            with patch.object(home_tools, 'request_json', return_value={'ok': True}) as req:
                home_tools.camera_command('bat rieng tu camera')
                self.assertEqual('/api/camera/privacy', req.call_args_list[0].args[1])
                body = req.call_args_list[0].kwargs.get('body', {})
                self.assertTrue(body.get('enabled'))

            with patch.object(home_tools, 'request_json', return_value={'ok': True}) as req:
                home_tools.camera_command('nói qua camera: Mời cả nhà ăn cơm')
                self.assertEqual('/api/camera/speak', req.call_args_list[0].args[1])
                body = req.call_args_list[0].kwargs.get('body', {})
                self.assertEqual('Mời cả nhà ăn cơm', body.get('text'))

    def test_media_query_cleaning_variations(self):
        self.assertEqual('avatar', media_bridge.clean_query('tìm phim avatar vietsub'))
        self.assertEqual('dune 2021', media_bridge.clean_query('phim lẻ dune 2021 pro'))
        self.assertEqual('breaking bad', media_bridge.clean_query('breaking bad season 1'))
        self.assertEqual('doraemon', media_bridge.clean_query('xem hoạt hình doraemon tập 1'))

    def test_unavailable_device_aborts_early(self):
        state = {
            'entity_id': 'light.den_ban_cong',
            'state': 'unavailable',
            'attributes': {'friendly_name': 'Đèn ban công'}
        }
        with patch.object(home_tools, 'ha_request', return_value=[state]):
            reply = home_tools.ha_command('bật đèn ban công')
            self.assertIn('không khả dụng', reply)


if __name__ == '__main__':
    unittest.main()
