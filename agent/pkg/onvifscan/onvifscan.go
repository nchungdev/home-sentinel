package onvifscan

import (
	"encoding/xml"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

type DiscoveredDevice struct {
	IP          string `json:"ip"`
	Name        string `json:"name"`
	Hardware    string `json:"hardware"`
	Type        string `json:"type"`
	XAddr       string `json:"xaddr"`
	OnvifPort   int    `json:"onvif_port"`
	Manufacturer string `json:"manufacturer"`
}

type probeMatchSoap struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		ProbeMatches struct {
			ProbeMatch []struct {
				EndpointReference struct {
					Address string `xml:"Address"`
				} `xml:"EndpointReference"`
				Types  string `xml:"Types"`
				Scopes string `xml:"Scopes"`
				XAddrs string `xml:"XAddrs"`
			} `xml:"ProbeMatch"`
		} `xml:"ProbeMatches"`
	} `xml:"Body"`
}

const probeMessage = `<?xml version="1.0" encoding="utf-8"?>
<Envelope xmlns="http://www.w3.org/2003/05/soap-envelope">
  <Header>
    <wsa:Action xmlns:wsa="http://schemas.xmlsoap.org/ws/2004/08/addressing">http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</wsa:Action>
    <wsa:MessageID xmlns:wsa="http://schemas.xmlsoap.org/ws/2004/08/addressing">uuid:home-sentinel-discovery</wsa:MessageID>
    <wsa:To xmlns:wsa="http://schemas.xmlsoap.org/ws/2004/08/addressing">urn:schemas-xmlsoap-org:ws:2005:04:discovery</wsa:To>
  </Header>
  <Body>
    <Probe xmlns="http://schemas.xmlsoap.org/ws/2005/04/discovery">
      <Types xmlns:dn="http://www.onvif.org/ver10/network/wsdl">dn:NetworkVideoTransmitter</Types>
    </Probe>
  </Body>
</Envelope>`

func ScanDevices(timeout time.Duration) ([]DiscoveredDevice, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	multicastAddr := &net.UDPAddr{
		IP:   net.ParseIP("239.255.255.250"),
		Port: 3702,
	}

	if _, err := conn.WriteTo([]byte(probeMessage), multicastAddr); err != nil {
		return nil, err
	}

	_ = conn.SetReadDeadline(time.Now().Add(timeout))

	deviceMap := make(map[string]DiscoveredDevice)
	buffer := make([]byte, 8192)

	for {
		readBytes, remoteAddr, err := conn.ReadFrom(buffer)
		if err != nil {
			break
		}

		ipStr := ""
		if udpAddr, ok := remoteAddr.(*net.UDPAddr); ok {
			ipStr = udpAddr.IP.String()
		}

		var soapResp probeMatchSoap
		if parseErr := xml.Unmarshal(buffer[:readBytes], &soapResp); parseErr == nil {
			for _, pm := range soapResp.Body.ProbeMatches.ProbeMatch {
				dev := parseProbeMatch(ipStr, pm.Scopes, pm.XAddrs)
				if dev.IP != "" {
					deviceMap[dev.IP] = dev
				}
			}
		}
	}

	var result []DiscoveredDevice
	for _, dev := range deviceMap {
		result = append(result, dev)
	}
	return result, nil
}

func parseProbeMatch(fallbackIP, scopes, xaddrs string) DiscoveredDevice {
	dev := DiscoveredDevice{
		IP:           fallbackIP,
		Name:         "Generic ONVIF Camera",
		Hardware:     "IP Camera",
		Type:         "ONVIF Device",
		XAddr:        xaddrs,
		Manufacturer: "Unknown",
	}

	if xaddrs != "" {
		firstAddr := strings.Fields(xaddrs)[0]
		if u, err := url.Parse(firstAddr); err == nil {
			host, portStr, err := net.SplitHostPort(u.Host)
			if err == nil {
				dev.IP = host
				var p int
				_, _ = fmt.Sscanf(portStr, "%d", &p)
				dev.OnvifPort = p
			} else {
				dev.IP = u.Host
			}
		}
	}

	scopeParts := strings.Fields(scopes)
	for _, s := range scopeParts {
		lower := strings.ToLower(s)
		if strings.Contains(lower, "/name/") {
			parts := strings.Split(s, "/name/")
			if len(parts) > 1 {
				dev.Name = cleanScopeValue(parts[1])
			}
		} else if strings.Contains(lower, "/hardware/") {
			parts := strings.Split(s, "/hardware/")
			if len(parts) > 1 {
				dev.Hardware = cleanScopeValue(parts[1])
			}
		} else if strings.Contains(lower, "/manufacturer/") {
			parts := strings.Split(s, "/manufacturer/")
			if len(parts) > 1 {
				dev.Manufacturer = cleanScopeValue(parts[1])
			}
		}
	}

	nameLower := strings.ToLower(dev.Name + " " + dev.Hardware)
	if strings.Contains(nameLower, "c200") || strings.Contains(nameLower, "tapo") || strings.Contains(nameLower, "c100") || strings.Contains(nameLower, "c310") {
		dev.Manufacturer = "TP-Link Tapo"
	} else if strings.Contains(nameLower, "hikvision") {
		dev.Manufacturer = "Hikvision"
	} else if strings.Contains(nameLower, "dahua") || strings.Contains(nameLower, "imou") {
		dev.Manufacturer = "Dahua / Imou"
	} else if strings.Contains(nameLower, "ezviz") {
		dev.Manufacturer = "EZVIZ"
	}

	return dev
}

func cleanScopeValue(v string) string {
	res, err := url.QueryUnescape(v)
	if err == nil {
		return res
	}
	return v
}
