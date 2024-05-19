package flv

import (
	"github.com/timmattison/gomedia/go-codec"
)

func PutUint24(b []byte, v uint32) {
	_ = b[2]
	b[0] = byte(v >> 16)
	b[1] = byte(v >> 8)
	b[2] = byte(v)
}

func GetUint24(b []byte) (v uint32) {
	_ = b[2]
	v = uint32(b[0])
	v = (v << 8) | uint32(b[1])
	v = (v << 8) | uint32(b[2])
	return v
}

func CovertFlvVideoCodecId2MpegCodecId(cid FlvVideoCodecId) codec.CodecID {
	if cid == FlvAvc {
		return codec.CodecidVideoH264
	} else if cid == FlvHevc {
		return codec.CodecidVideoH265
	}
	return codec.CodecidUnrecognized
}

func CovertFlvAudioCodecId2MpegCodecId(cid FlvSoundFormat) codec.CodecID {
	if cid == FlvAac {
		return codec.CodecidAudioAac
	} else if cid == FlvG711a {
		return codec.CodecidAudioG711a
	} else if cid == FlvG711u {
		return codec.CodecidAudioG711u
	} else if cid == FlvMp3 {
		return codec.CodecidAudioMp3
	}
	return codec.CodecidUnrecognized
}

func CovertCodecId2FlvVideoCodecId(cid codec.CodecID) FlvVideoCodecId {
	if cid == codec.CodecidVideoH264 {
		return FlvAvc
	} else if cid == codec.CodecidVideoH265 {
		return FlvHevc
	} else {
		panic("unsupport flv video codec")
	}
}

func CovertCodecId2SoundFromat(cid codec.CodecID) FlvSoundFormat {
	if cid == codec.CodecidAudioAac {
		return FlvAac
	} else if cid == codec.CodecidAudioG711a {
		return FlvG711a
	} else if cid == codec.CodecidAudioG711u {
		return FlvG711u
	} else {
		panic("unsupport flv audio codec")
	}
}

func GetTagLenByAudioCodec(cid FlvSoundFormat) int {
	if cid == FlvAac {
		return 2
	} else {
		return 1
	}
}

func GetTagLenByVideoCodec(cid FlvVideoCodecId) int {
	if cid == FlvAvc || cid == FlvHevc {
		return 5
	} else {
		return 1
	}
}
