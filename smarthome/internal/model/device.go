package model

type Device struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Room            string `json:"room"`
	Group           string `json:"group"`
	HomeAssistantID string `json:"home_assistant_id"`
	State           string `json:"state"`
	IsCritical      bool   `json:"is_critical"`
}

type DeviceCommand struct {
	State string `json:"state"`
}

func Definitions() []Device {
	return []Device{
		{
			ID:              "ct_cua_chinh_1",
			Name:            "Cửa chính - Nút 1 (Gateway/Nguồn)",
			Room:            "Cửa chính & Khách",
			Group:           "Công tắc Cửa chính",
			HomeAssistantID: "light.dqsmart_020119140c_1",
			State:           "off",
			IsCritical:      true,
		},
		{
			ID:              "ct_cua_chinh_2",
			Name:            "Cửa chính - Nút 2",
			Room:            "Cửa chính & Khách",
			Group:           "Công tắc Cửa chính",
			HomeAssistantID: "light.dqsmart_020119140c_2",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_cua_chinh_3",
			Name:            "Cửa chính - Nút 3",
			Room:            "Cửa chính & Khách",
			Group:           "Công tắc Cửa chính",
			HomeAssistantID: "light.dqsmart_020119140c_3",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_cua_chinh_4",
			Name:            "Cửa chính - Nút 4",
			Room:            "Cửa chính & Khách",
			Group:           "Công tắc Cửa chính",
			HomeAssistantID: "light.dqsmart_020119140c_4",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pk_1_1",
			Name:            "Phòng khách 1 - Nút 1",
			Room:            "Phòng khách",
			Group:           "Công tắc Phòng khách 1",
			HomeAssistantID: "light.dqsmart_0201bff7b7_1",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pk_1_2",
			Name:            "Phòng khách 1 - Nút 2",
			Room:            "Phòng khách",
			Group:           "Công tắc Phòng khách 1",
			HomeAssistantID: "light.dqsmart_0201bff7b7_2",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pk_1_3",
			Name:            "Phòng khách 1 - Nút 3",
			Room:            "Phòng khách",
			Group:           "Công tắc Phòng khách 1",
			HomeAssistantID: "light.dqsmart_0201bff7b7_3",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pk_1_4",
			Name:            "Phòng khách 1 - Nút 4",
			Room:            "Phòng khách",
			Group:           "Công tắc Phòng khách 1",
			HomeAssistantID: "light.dqsmart_0201bff7b7_4",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pk_2_1",
			Name:            "Phòng khách 2 - Nút 1",
			Room:            "Phòng khách",
			Group:           "Công tắc Phòng khách 2",
			HomeAssistantID: "light.dqsmart_0201c007ce_1",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pk_2_2",
			Name:            "Phòng khách 2 - Nút 2",
			Room:            "Phòng khách",
			Group:           "Công tắc Phòng khách 2",
			HomeAssistantID: "light.dqsmart_0201c007ce_2",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pk_2_3",
			Name:            "Phòng khách 2 - Nút 3",
			Room:            "Phòng khách",
			Group:           "Công tắc Phòng khách 2",
			HomeAssistantID: "light.dqsmart_0201c007ce_3",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pk_2_4",
			Name:            "Phòng khách 2 - Nút 4",
			Room:            "Phòng khách",
			Group:           "Công tắc Phòng khách 2",
			HomeAssistantID: "light.dqsmart_0201c007ce_4",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pn_nho_1",
			Name:            "PN Nhỏ - Nút 1",
			Room:            "Phòng ngủ nhỏ",
			Group:           "Công tắc Phòng ngủ nhỏ",
			HomeAssistantID: "light.dqsmart_0201def916_1",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pn_nho_2",
			Name:            "PN Nhỏ - Nút 2",
			Room:            "Phòng ngủ nhỏ",
			Group:           "Công tắc Phòng ngủ nhỏ",
			HomeAssistantID: "light.dqsmart_0201def916_2",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pn_nho_3",
			Name:            "PN Nhỏ - Nút 3",
			Room:            "Phòng ngủ nhỏ",
			Group:           "Công tắc Phòng ngủ nhỏ",
			HomeAssistantID: "light.dqsmart_0201def916_3",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pn_nho_4",
			Name:            "PN Nhỏ - Nút 4",
			Room:            "Phòng ngủ nhỏ",
			Group:           "Công tắc Phòng ngủ nhỏ",
			HomeAssistantID: "light.dqsmart_0201def916_4",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pn_chinh_1",
			Name:            "PN Chính - Nút 1",
			Room:            "Phòng ngủ chính",
			Group:           "Công tắc Phòng ngủ chính",
			HomeAssistantID: "light.dqsmart_0201decd98_1",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pn_chinh_2",
			Name:            "PN Chính - Nút 2",
			Room:            "Phòng ngủ chính",
			Group:           "Công tắc Phòng ngủ chính",
			HomeAssistantID: "light.dqsmart_0201decd98_2",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pn_chinh_3",
			Name:            "PN Chính - Nút 3",
			Room:            "Phòng ngủ chính",
			Group:           "Công tắc Phòng ngủ chính",
			HomeAssistantID: "light.dqsmart_0201decd98_3",
			State:           "off",
			IsCritical:      false,
		},
		{
			ID:              "ct_pn_chinh_4",
			Name:            "PN Chính - Nút 4",
			Room:            "Phòng ngủ chính",
			Group:           "Công tắc Phòng ngủ chính",
			HomeAssistantID: "light.dqsmart_0201decd98_4",
			State:           "off",
			IsCritical:      false,
		},
	}
}
