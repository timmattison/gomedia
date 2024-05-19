package rtsp

import (
	"fmt"
	"strings"
)

type LowerTransport int

const (
	UDP LowerTransport = iota
	TCP
)

const (
	ModePlay   = "PLAY"
	ModeRecord = "RECORD"
)

type RtspTransport struct {
	IsMultiCast bool
	Proto       LowerTransport
	ClientPorts [2]uint16
	ServerPorts [2]uint16
	Interleaved [2]int
	mode        string
}

type TransportOption func(transport *RtspTransport)

func WithEnableUdp() TransportOption {
	return func(transport *RtspTransport) {
		transport.Proto = UDP
	}
}

func WithClientUdpPort(rtpPort uint16, rtcpPort uint16) TransportOption {
	return func(transport *RtspTransport) {
		transport.ClientPorts[0] = rtpPort
		transport.ClientPorts[1] = rtcpPort
	}
}

func WithServerUdpPort(rtpPort uint16, rtcpPort uint16) TransportOption {
	return func(transport *RtspTransport) {
		transport.ServerPorts[0] = rtpPort
		transport.ServerPorts[1] = rtcpPort
	}
}

func WithTcpInterleaved(interleaved [2]int) TransportOption {
	return func(transport *RtspTransport) {
		transport.Interleaved[0] = interleaved[0]
		transport.Interleaved[1] = interleaved[1]
	}
}

func WithMode(mode string) TransportOption {
	return func(transport *RtspTransport) {
		transport.mode = mode
	}
}

func NewRtspTransport(opt ...TransportOption) *RtspTransport {
	transport := &RtspTransport{
		IsMultiCast: false,
		Proto:       TCP,
		mode:        ModePlay,
	}
	for _, o := range opt {
		o(transport)
	}
	return transport
}

func (transport *RtspTransport) SetServerUdpPort(rtpPort uint16, rtcpPort uint16) {
	transport.ClientPorts[0] = rtpPort
	transport.ClientPorts[1] = rtcpPort
}

func (transport *RtspTransport) SetClientUdpPort(rtpPort uint16, rtcpPort uint16) {
	transport.ClientPorts[0] = rtpPort
	transport.ClientPorts[1] = rtcpPort
}

func (transport *RtspTransport) SetInterleaved(interleaved [2]int) {
	transport.Interleaved[0] = interleaved[0]
	transport.Interleaved[1] = interleaved[1]
}

// Transport: RTP/AVP;multicast;ttl=127;mode="PLAY",
//            RTP/AVP;unicast;client_port=3456-3457;mode="PLAY"

func (transport *RtspTransport) Decode(data []byte) error {
	return transport.DecodeString(string(data))
}

func (transport *RtspTransport) DecodeString(data string) error {
	items := strings.Split(data, ";")
	for _, item := range items {
		kv := strings.Split(item, "=")
		switch kv[0] {
		case "RTP/AVP/TCP":
			transport.Proto = TCP
		case "RTP/AVP", "RTP/AVP/UDP":
			transport.Proto = UDP
		case "multicast":
			transport.IsMultiCast = true
		case "unicast":
			transport.IsMultiCast = false
		case "mode":
			transport.mode = kv[1]
		case "client_port":
			fmt.Sscanf(kv[1], "%d-%d", &transport.ClientPorts[0], &transport.ClientPorts[1])
		case "server_port":
			fmt.Sscanf(kv[1], "%d-%d", &transport.ServerPorts[0], &transport.ServerPorts[1])
		case "interleaved":
			fmt.Sscanf(kv[1], "%d-%d", &transport.Interleaved[0], &transport.Interleaved[1])
		}
	}
	return nil
}

func (transport *RtspTransport) EncodeString() string {
	str := ""
	if transport.Proto == TCP {
		str += "RTP/AVP/TCP"
	} else {
		str += "RTP/AVP/UDP"
	}
	if transport.IsMultiCast {
		str += ";multicast"
	} else {
		str += ";unicast"
	}

	if transport.Proto == TCP {
		str += fmt.Sprintf(";interleaved=%d-%d", transport.Interleaved[0], transport.Interleaved[1])
	} else {
		if transport.ClientPorts[0] != 0 {
			str += fmt.Sprintf(";client_port=%d-%d", transport.ClientPorts[0], transport.ClientPorts[1])
		}
		if transport.ServerPorts[0] != 0 {
			str += fmt.Sprintf(";server_port=%d-%d", transport.ServerPorts[0], transport.ServerPorts[1])
		}
	}

	if strings.ToUpper(transport.mode) == ModePlay {
		str += ";mode=PLAY"
	} else if strings.ToUpper(transport.mode) == ModeRecord {
		str += ";mode=RECORD"
	}
	return str
}
