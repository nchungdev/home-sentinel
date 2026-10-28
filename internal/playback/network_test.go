package playback

import (
	"net"
	"testing"
)

func TestStreamSourceRejectsPrivateAndReservedAddresses(t *testing.T) {
	for _, value := range []string{
		"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "198.18.0.1",
		"192.0.2.1", "::1", "fc00::1", "fe80::1", "fec0::1", "2001:db8::1",
		"::ffff:127.0.0.1", "::ffff:100.64.0.1",
	} {
		if publicIP(net.ParseIP(value)) {
			t.Errorf("non-public source accepted: %s", value)
		}
	}
	for _, value := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !publicIP(net.ParseIP(value)) {
			t.Errorf("public source rejected: %s", value)
		}
	}
}
