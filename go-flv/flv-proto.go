package flv

import "github.com/timmattison/gomedia/go-codec"

type FLVSAMPLEINDEX int

const (
	FlvSample5500 FLVSAMPLEINDEX = iota
	FlvSample11000
	FlvSample22000
	FlvSample44000
)

type TagType int

const (
	AudioTagType  TagType = 8
	VideoTagType  TagType = 9
	ScriptTagType TagType = 18
)

type FlvVideoFrameType int

const (
	KeyFrame   FlvVideoFrameType = 1
	InterFrame FlvVideoFrameType = 2
)

type FlvVideoCodecId int

const (
	FlvAvc  FlvVideoCodecId = 7
	FlvHevc FlvVideoCodecId = 12
)

const (
	AvcSequenceHeader = 0
	AvcNalu           = 1
)

const (
	AacSequenceHeader = 0
	AacRaw            = 1
)

type FlvSoundFormat int

const (
	FlvMp3   FlvSoundFormat = 2
	FlvG711a FlvSoundFormat = 7
	FlvG711u FlvSoundFormat = 8
	FlvAac   FlvSoundFormat = 10
)

// enhanced-rtmp Table 4
const (
	PacketTypeSequenceStart        = 0
	PacketTypeCodedFrames          = 1
	PacketTypeSequenceEnd          = 2
	PacketTypeCodedFramesX         = 3
	PacketTypeMetadata             = 4
	PacketTypeMPEG2TSSequenceStart = 5
)

func GetFLVVideoCodecId(data []byte) (cid FlvVideoCodecId) {
	isExHeader := data[0] & 0x80
	if isExHeader != 0 {
		// TODO av1和VP9
		if data[1] == 'h' && data[2] == 'v' && data[3] == 'c' && data[4] == '1' {
			// hevc
			cid = FlvHevc
		}
	} else {
		cid = FlvVideoCodecId(data[0] & 0x0F)
	}
	return cid
}

func (format FlvSoundFormat) ToMpegCodecId() codec.CodecID {
	switch {
	case format == FlvG711a:
		return codec.CodecidAudioG711a
	case format == FlvG711u:
		return codec.CodecidAudioG711u
	case format == FlvAac:
		return codec.CodecidAudioAac
	case format == FlvMp3:
		return codec.CodecidAudioMp3
	default:
		panic("unsupport sound format")
	}
}
