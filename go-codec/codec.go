package codec

type CodecID int

const (
	CodecidVideoH264 CodecID = iota
	CodecidVideoH265
	CodecidVideoVp8

	CodecidAudioAac CodecID = iota + 98
	CodecidAudioG711a
	CodecidAudioG711u
	CodecidAudioOpus
	CodecidAudioMp3

	CodecidUnrecognized = 999
)

type H264NalType int

const (
	H264NalReserved H264NalType = iota
	H264NalPSlice
	H264NalSliceA
	H264NalSliceB
	H264NalSliceC
	H264NalISlice
	H264NalSei
	H264NalSps
	H264NalPps
	H264NalAud
)

type H265NalType int

const (
	H265NalSliceTrailN H265NalType = iota
	H265NalLiceTrailR
	H265NalSliceTsaN
	H265NalSliceTsaR
	H265NalSliceStsaN
	H265NalSliceStsaR
	H265NalSliceRadlN
	H265NalSliceRadlR
	H265NalSliceRaslN
	H265NalSliceRaslR

	//IDR
	H265NalSliceBlaWLp H265NalType = iota + 6
	H265NalSliceBlaWRadl
	H265NalSliceBlaNLp
	H265NalSliceIdrWRadl
	H265NalSliceIdrNLp
	H265NalSliceCra

	//vps pps sps
	H265NalVps H265NalType = iota + 16
	H265NalSps
	H265NalPps
	H265NalAud

	//SEI
	H265NalSei H265NalType = iota + 19
	H265NalSeiSuffix
)

func CodecString(codecid CodecID) string {
	switch codecid {
	case CodecidVideoH264:
		return "H264"
	case CodecidVideoH265:
		return "H265"
	case CodecidVideoVp8:
		return "VP8"
	case CodecidAudioAac:
		return "AAC"
	case CodecidAudioG711a:
		return "G711A"
	case CodecidAudioG711u:
		return "G711U"
	case CodecidAudioOpus:
		return "OPUS"
	case CodecidAudioMp3:
		return "MP3"
	default:
		return "UNRECOGNIZED"
	}
}
