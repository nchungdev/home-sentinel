package main

import (
	"testing"
)

func TestExtractTVMessage(t *testing.T) {
	tests := []struct {
		input       string
		expectedMsg string
		expectedOk  bool
	}{
		{
			input:       "nói với tv là Hôm nay có cuộc họp quan trọng",
			expectedMsg: "Hôm nay có cuộc họp quan trọng",
			expectedOk:  true,
		},
		{
			input:       "nói với tv rằng tối nay về muộn",
			expectedMsg: "tối nay về muộn",
			expectedOk:  true,
		},
		{
			input:       "nói với tv: Nhớ uống thuốc lúc 9h",
			expectedMsg: "Nhớ uống thuốc lúc 9h",
			expectedOk:  true,
		},
		{
			input:       "nhắn lên tv là chuẩn bị ăn cơm",
			expectedMsg: "chuẩn bị ăn cơm",
			expectedOk:  true,
		},
		{
			input:       "bảo tv là tắt máy đi ngủ",
			expectedMsg: "tắt máy đi ngủ",
			expectedOk:  true,
		},
		{
			input:       "gửi 1 lời chào lên tv của tôi",
			expectedMsg: "",
			expectedOk:  false,
		},
		{
			input:       "gửi lời chào lên tv",
			expectedMsg: "",
			expectedOk:  false,
		},
		{
			input:       "nói gì đó lên tv",
			expectedMsg: "",
			expectedOk:  false,
		},
		{
			input:       "chào tv đi",
			expectedMsg: "",
			expectedOk:  false,
		},
	}

	for _, tc := range tests {
		msg, ok := extractTVMessage(tc.input)
		if ok != tc.expectedOk {
			t.Errorf("extractTVMessage(%q): expected ok=%v, got %v", tc.input, tc.expectedOk, ok)
		}
		if msg != tc.expectedMsg {
			t.Errorf("extractTVMessage(%q): expected msg=%q, got %q", tc.input, tc.expectedMsg, msg)
		}
	}
}
