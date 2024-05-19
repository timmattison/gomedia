package rtsp

import (
	"strings"
)

type RtspCodecId int

const (
	RtspCodecH264 RtspCodecId = iota
	RtspCodecH265
	RtspCodecAac
	RtspCodecG711a
	RtspCodecG711u
	RtspCodecPs
	RtspCodecTs
)

type RtspCodec struct {
	Cid          RtspCodecId //H264,H265,PCMU,PCMA...
	PayloadType  uint8
	SampleRate   uint32
	ChannelCount uint8
}

func GetCodecIdByEncodeName(name string) RtspCodecId {
	lowName := strings.ToLower(name)
	switch lowName {
	case "h264":
		return RtspCodecH264
	case "h265":
		return RtspCodecH265
	case "mpeg4-generic", "mpeg4-latm":
		return RtspCodecAac
	case "pcmu":
		return RtspCodecG711a
	case "pcma":
		return RtspCodecG711u
	case "mp2t":
		return RtspCodecTs
	}
	panic("unsupport codec")
}

func GetEncodeNameByCodecId(cid RtspCodecId) string {
	switch cid {
	case RtspCodecH264:
		return "H264"
	case RtspCodecH265:
		return "H265"
	case RtspCodecAac:
		return "mpeg4-generic"
	case RtspCodecG711a:
		return "pcma"
	case RtspCodecG711u:
		return "pcmu"
	case RtspCodecPs:
		return "MP2P"
	case RtspCodecTs:
		return "MP2T"
	default:
		panic("unsupport rtsp codec id")
	}
}

func NewCodec(name string, pt uint8, sampleRate uint32, channel uint8) RtspCodec {
	return RtspCodec{Cid: GetCodecIdByEncodeName(name), PayloadType: pt, SampleRate: sampleRate, ChannelCount: channel}
}

func NewVideoCodec(name string, pt uint8, sampleRate uint32) RtspCodec {
	return RtspCodec{Cid: GetCodecIdByEncodeName(name), PayloadType: pt, SampleRate: sampleRate}
}

func NewAudioCodec(name string, pt uint8, sampleRate uint32, channelCount int) RtspCodec {
	return RtspCodec{Cid: GetCodecIdByEncodeName(name), PayloadType: pt, SampleRate: sampleRate, ChannelCount: uint8(channelCount)}
}

func NewApplicatioCodec(name string, pt uint8) RtspCodec {
	return RtspCodec{Cid: GetCodecIdByEncodeName(name), PayloadType: pt}
}
