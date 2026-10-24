import json
import unittest
from unittest.mock import MagicMock, patch

import dqsmart_gateway
import server


class SmarthomeGatewayTest(unittest.TestCase):
    def test_switch_group_device_generation(self):
        self.assertEqual(len(dqsmart_gateway.SWITCH_GROUPS), 5)
        self.assertEqual(len(dqsmart_gateway.DEVICES), 20)

        cuaChinhNuts = [d for d in dqsmart_gateway.DEVICES if d["id"].startswith("ct_cua_chinh")]
        self.assertEqual(len(cuaChinhNuts), 4)
        self.assertEqual(cuaChinhNuts[0]["x"], 297)
        self.assertEqual(cuaChinhNuts[1]["x"], 333)
        self.assertEqual(cuaChinhNuts[2]["x"], 369)
        self.assertEqual(cuaChinhNuts[3]["x"], 405)

    def test_scene_definitions(self):
        sceneIds = [s["id"] for s in dqsmart_gateway.SCENES]
        self.assertIn("tat_dien_khu_sinh_hoat_chung", sceneIds)
        self.assertIn("roi_nha", sceneIds)
        self.assertIn("ve_nha", sceneIds)

    def test_pixel_rgb_extraction(self):
        fakeData = bytearray(1920 * 1080 * 4 + 54)
        offset = 54
        targetIndex = offset + 905 * 1920 * 4 + 300 * 4
        fakeData[targetIndex:targetIndex + 4] = bytearray([50, 40, 230, 255])
        red, green, blue = dqsmart_gateway.extractPixelRgb(fakeData, offset, 300, 905)
        self.assertEqual(red, 230)
        self.assertEqual(green, 40)
        self.assertEqual(blue, 50)

    @patch("dqsmart_gateway.readAllStates")
    @patch("dqsmart_gateway.executeAdb")
    @patch("dqsmart_gateway.ensureDevicesScreen")
    def test_control_device_toggle(self, mockEnsure, mockAdb, mockRead):
        mockRead.side_effect = [
            [{"id": "ct_cua_chinh_1", "state": "off"}],
            [{"id": "ct_cua_chinh_1", "state": "on"}],
        ]
        updated = dqsmart_gateway.controlDevice("ct_cua_chinh_1", "toggle")
        self.assertEqual(updated[0]["state"], "on")
        mockAdb.assert_called_with(["shell", "input", "tap", "297", "905"])


if __name__ == "__main__":
    unittest.main()
