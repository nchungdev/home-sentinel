package playback

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

func publicIP(address net.IP) bool {
	return address.IsGlobalUnicast() && !address.IsPrivate() && !address.IsLoopback() &&
		!address.IsLinkLocalUnicast() && !address.IsLinkLocalMulticast() && !reservedStreamIP(address)
}

func publicAddresses(ctx context.Context, host string) ([]net.IPAddr, error) {
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, fmt.Errorf("stream_host_unreachable")
	}
	for _, address := range addresses {
		if !publicIP(address.IP) {
			return nil, fmt.Errorf("stream_host_not_allowed")
		}
	}
	return addresses, nil
}

func validatePublicHost(ctx context.Context, host string) error {
	_, err := publicAddresses(ctx, host)
	return err
}

func streamClient(trustedOrigin string, headers http.Header) *http.Client {
	transport := &http.Transport{MaxIdleConnsPerHost: 4, MaxConnsPerHost: 8,
		IdleConnTimeout: 30 * time.Second, ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		trusted, _ := url.Parse(trustedOrigin)
		dialer := net.Dialer{Timeout: 5 * time.Second}
		trustedAddress := ""
		if trusted != nil && trusted.Hostname() != "" {
			trustedPort := trusted.Port()
			if trustedPort == "" {
				trustedPort = "80"
				if trusted.Scheme == "https" {
					trustedPort = "443"
				}
			}
			trustedAddress = net.JoinHostPort(trusted.Hostname(), trustedPort)
		}
		if address == trustedAddress {
			return dialer.DialContext(ctx, network, address)
		}
		addresses, err := publicAddresses(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, candidate := range addresses {
			connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
			if err == nil {
				return connection, nil
			}
		}
		return nil, fmt.Errorf("stream_host_unreachable")
	}
	return &http.Client{Transport: transport, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too_many_redirects")
		}
		if request.URL.Scheme != "http" && request.URL.Scheme != "https" {
			return fmt.Errorf("invalid_stream_redirect")
		}
		if request.URL.Scheme+"://"+request.URL.Host != trustedOrigin {
			for name := range headers {
				if name != "User-Agent" && name != "Referer" && name != "Origin" {
					request.Header.Del(name)
				}
			}
		}
		return nil
	}}
}
