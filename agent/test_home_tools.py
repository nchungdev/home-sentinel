import unittest
from unittest.mock import patch
import home_tools
import media_bridge


class HomeToolsTest(unittest.TestCase):
    def test_ambiguous_name_does_not_control(self):
        states = [{'entity_id': 'light.a', 'state': 'on',
                   'attributes': {'friendly_name': 'Đèn phòng khách'}},
                  {'entity_id': 'light.b', 'state': 'off',
                   'attributes': {'friendly_name': 'Đèn phòng ngủ'}}]
        with patch.object(home_tools, 'ha_request', return_value=states) as request:
            self.assertIn('nhiều thiết bị', home_tools.ha_command('bật đèn'))
            request.assert_called_once_with('/api/states')

    def test_protected_source_never_receives_action(self):
        states = [{'entity_id': 'light.hub', 'state': 'on',
                   'attributes': {'friendly_name': 'Nguồn Hub'}}]
        with patch.object(home_tools, 'ha_request', return_value=states) as request, \
                patch.dict('os.environ', {'AGENT_HA_PROTECTED_ENTITIES': 'light.hub'}):
            self.assertIn('bảo vệ', home_tools.ha_command('tắt nguồn hub'))
            self.assertEqual(1, request.call_count)

    def test_service_domain_and_readback(self):
        state = {'entity_id': 'fan.room', 'state': 'off',
                 'attributes': {'friendly_name': 'Quạt phòng'}}
        with patch.object(home_tools, 'ha_request', side_effect=[[state], [], state]) as request:
            self.assertIn('HA đã nhận', home_tools.ha_command('bật quạt phòng'))
            self.assertEqual('/api/services/fan/turn_on', request.call_args_list[1].args[0])
            self.assertEqual('/api/states/fan.room', request.call_args_list[2].args[0])

    def test_no_delete_intent_reaches_tools(self):
        with patch.object(home_tools, 'ha_request') as request:
            self.assertIsNone(home_tools.handle('xóa tất cả phim'))
            request.assert_not_called()

    def test_camera_false_success_is_rejected(self):
        with patch.dict('os.environ', {'AGENT_CAMERA_URL': 'http://127.0.0.1:8792',
                                      'AGENT_CAMERA_PASSWORD': 'test'}), \
                patch.object(home_tools, 'request_json', return_value={'ok': False}):
            self.assertIn('không thực hiện', home_tools.camera_command('quay camera trái'))

    def test_movie_remake_requires_year(self):
        items = [{'title': 'Dune', 'year': 1984, 'tmdbId': 1},
                 {'title': 'Dune', 'year': 2021, 'tmdbId': 2}]
        self.assertIsNone(media_bridge.select_arr_item(items, 'Dune', 'tmdbId'))
        self.assertEqual(2, media_bridge.select_arr_item(items, 'Dune 2021', 'tmdbId')['tmdbId'])

    def test_ambiguous_movie_never_added(self):
        with patch.object(media_bridge, 'arr_request', return_value=[]) as request:
            self.assertIn('Chưa xác định', media_bridge.add_to_radarr('unknown'))
            self.assertEqual(1, request.call_count)

    def test_root_and_profile_come_from_actual_service(self):
        item = {'title': 'Example', 'year': 2026, 'tmdbId': 42}
        with patch.object(media_bridge, 'arr_request', side_effect=[
            [item], [], [{'path': '/actual/movies'}], [{'id': 7}],
            dict(item, id=123), dict(item, id=123)]) as request:
            self.assertIn('Đã thêm', media_bridge.add_to_radarr('Example 2026'))
            payload = request.call_args_list[4].args[2]
            self.assertEqual('/actual/movies', payload['rootFolderPath'])
            self.assertEqual(7, payload['qualityProfileId'])
