package llm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

func relayLoginCallback(authURL, callbackURL string) error {
	authorization, err := url.Parse(authURL)
	if err != nil {
		return fmt.Errorf("URL xác thực không hợp lệ")
	}
	expected, err := url.Parse(authorization.Query().Get("redirect_uri"))
	if err != nil || expected.Scheme != "http" || expected.Port() == "" {
		return fmt.Errorf("CLI không yêu cầu callback localhost")
	}
	if expected.Hostname() != "localhost" && expected.Hostname() != "127.0.0.1" &&
		expected.Hostname() != "::1" {
		return fmt.Errorf("callback phải trỏ về CLI trên máy chủ")
	}
	received, err := url.Parse(callbackURL)
	if err != nil || received.User != nil || received.Scheme != expected.Scheme ||
		received.Host != expected.Host || received.Path != expected.Path {
		return fmt.Errorf("callback không khớp với phiên đăng nhập đang mở")
	}
	state := authorization.Query().Get("state")
	if state == "" || received.Query().Get("state") != state || received.Query().Get("code") == "" {
		return fmt.Errorf("mã hoặc state không khớp với phiên đăng nhập")
	}
	query := expected.Query()
	query.Set("code", received.Query().Get("code"))
	query.Set("state", state)
	expected.RawQuery = query.Encode()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, expected.String(), nil)
	if err != nil {
		return fmt.Errorf("không tạo được callback")
	}
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("không gửi được callback tới CLI trên máy chủ")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode >= 400 {
		return fmt.Errorf("CLI từ chối callback xác thực")
	}
	return nil
}
