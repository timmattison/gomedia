package flv

import (
	"bytes"
	"errors"

	"github.com/timmattison/gomedia/go-codec"
)

func WriteAudioTag(data []byte, cid FlvSoundFormat, sampleRate int, channelCount int, isSequenceHeader bool) []byte {
	var atag AudioTag
	atag.SoundFormat = uint8(cid)
	if cid == FlvAac {
		atag.SoundRate = uint8(FlvSample44000)
		atag.SoundSize = 1
		atag.SoundType = 1
	} else {
		switch sampleRate {
		case 5500:
			atag.SoundRate = uint8(FlvSample5500)
		case 11025:
			atag.SoundRate = uint8(FlvSample11000)
		case 22050:
			atag.SoundRate = uint8(FlvSample22000)
		case 44100:
			atag.SoundRate = uint8(FlvSample44000)
		default:
			atag.SoundRate = uint8(FlvSample44000)
		}
		atag.SoundSize = 1
		if channelCount > 1 {
			atag.SoundType = 1
		} else {
			atag.SoundType = 0
		}
	}

	if isSequenceHeader {
		atag.AACPacketType = 0
	} else {
		atag.AACPacketType = 1
	}
	tagData := atag.Encode()
	tagData = append(tagData, data...)
	return tagData
}

func WriteVideoTag(data []byte, isKey bool, cid FlvVideoCodecId, cts int32, isSequenceHeader bool) []byte {
	var vtag VideoTag
	vtag.CodecId = uint8(cid)
	vtag.CompositionTime = cts
	if isKey {
		vtag.FrameType = uint8(KeyFrame)
	} else {
		vtag.FrameType = uint8(InterFrame)
	}
	if isSequenceHeader {
		vtag.AVCPacketType = uint8(AvcSequenceHeader)
	} else {
		vtag.AVCPacketType = uint8(AvcNalu)
	}
	tagData := vtag.Encode()
	tagData = append(tagData, data...)
	return tagData
}

type AVTagMuxer interface {
	Write(frames []byte, pts uint32, dts uint32) [][]byte
}

type AVCMuxer struct {
	spsset map[uint64][]byte
	ppsset map[uint64][]byte
	cache  []byte
	first  bool
}

func NewAVCMuxer() *AVCMuxer {
	return &AVCMuxer{
		spsset: make(map[uint64][]byte),
		ppsset: make(map[uint64][]byte),
		cache:  make([]byte, 0, 1024),
		first:  true,
	}
}

func (muxer *AVCMuxer) Write(frames []byte, pts uint32, dts uint32) [][]byte {
	var vcl = false
	var isKey = false
	codec.SplitFrameWithStartCode(frames, func(nalu []byte) bool {
		naltype := codec.H264NaluType(nalu)
		switch naltype {
		case codec.H264NalSps:
			spsid := codec.GetSPSIdWithStartCode(nalu)
			s, found := muxer.spsset[spsid]
			if !found || !bytes.Equal(s, nalu) {
				naluCopy := make([]byte, len(nalu))
				copy(naluCopy, nalu)
				muxer.spsset[spsid] = naluCopy
				muxer.cache = append(muxer.cache, codec.ConvertAnnexBToAVCC(nalu)...)
			}
		case codec.H264NalPps:
			ppsid := codec.GetPPSIdWithStartCode(nalu)
			muxer.ppsset[ppsid] = nalu
			s, found := muxer.ppsset[ppsid]
			if !found || !bytes.Equal(s, nalu) {
				naluCopy := make([]byte, len(nalu))
				copy(naluCopy, nalu)
				muxer.ppsset[ppsid] = naluCopy
				muxer.cache = append(muxer.cache, codec.ConvertAnnexBToAVCC(nalu)...)
			}
		default:
			if naltype <= codec.H264NalISlice {
				vcl = true
				if naltype == codec.H264NalISlice {
					isKey = true
				}
			}
			muxer.cache = append(muxer.cache, codec.ConvertAnnexBToAVCC(nalu)...)
		}
		return true
	})
	var tags [][]byte
	if muxer.first && len(muxer.ppsset) > 0 && len(muxer.spsset) > 0 {
		spss := make([][]byte, len(muxer.spsset))
		idx := 0
		for _, sps := range muxer.spsset {
			spss[idx] = sps
			idx++
		}
		idx = 0
		ppss := make([][]byte, len(muxer.ppsset))
		for _, pps := range muxer.ppsset {
			ppss[idx] = pps
			idx++
		}
		extraData, _ := codec.CreateH264AVCCExtradata(spss, ppss)
		tags = append(tags, WriteVideoTag(extraData, true, FlvAvc, 0, true))
		muxer.first = false
	}

	if vcl {
		tags = append(tags, WriteVideoTag(muxer.cache, isKey, FlvAvc, int32(pts-dts), false))
		muxer.cache = muxer.cache[:0]
	}
	return tags
}

type HevcMuxer struct {
	hvcc  *codec.HEVCRecordConfiguration
	cache []byte
	first bool
}

func NewHevcMuxer() *HevcMuxer {
	return &HevcMuxer{
		hvcc:  codec.NewHEVCRecordConfiguration(),
		cache: make([]byte, 0, 1024),
		first: true,
	}
}

func (muxer *HevcMuxer) Write(frames []byte, pts uint32, dts uint32) [][]byte {
	var vcl = false
	var isKey = false
	codec.SplitFrameWithStartCode(frames, func(nalu []byte) bool {
		naltype := codec.H265NaluType(nalu)
		switch naltype {
		case codec.H265NalSps:
			muxer.hvcc.UpdateSPS(nalu)
			muxer.cache = append(muxer.cache, codec.ConvertAnnexBToAVCC(nalu)...)
		case codec.H265NalPps:
			muxer.hvcc.UpdatePPS(nalu)
			muxer.cache = append(muxer.cache, codec.ConvertAnnexBToAVCC(nalu)...)
		case codec.H265NalVps:
			muxer.hvcc.UpdateVPS(nalu)
			muxer.cache = append(muxer.cache, codec.ConvertAnnexBToAVCC(nalu)...)
		default:
			if naltype >= 16 && naltype <= 21 {
				isKey = true
			}
			vcl = codec.IsH265VCLNaluType(naltype)
			muxer.cache = append(muxer.cache, codec.ConvertAnnexBToAVCC(nalu)...)
		}
		return true
	})
	var tags [][]byte
	if muxer.first && len(muxer.hvcc.Arrays) > 0 {
		extraData, _ := muxer.hvcc.Encode()
		tags = append(tags, WriteVideoTag(extraData, true, FlvHevc, 0, true))
		muxer.first = false
	}
	if vcl {
		tags = append(tags, WriteVideoTag(muxer.cache, isKey, FlvHevc, int32(pts-dts), false))
		muxer.cache = muxer.cache[:0]
	}
	return tags
}

func CreateVideoMuxer(cid FlvVideoCodecId) AVTagMuxer {
	if cid == FlvAvc {
		return NewAVCMuxer()
	} else if cid == FlvHevc {
		return NewHevcMuxer()
	}
	return nil
}

type AACMuxer struct {
	updateSequence bool
}

func NewAACMuxer() *AACMuxer {
	return &AACMuxer{updateSequence: true}
}

func (muxer *AACMuxer) Write(frames []byte, pts uint32, dts uint32) [][]byte {
	var tags [][]byte
	codec.SplitAACFrame(frames, func(aac []byte) {
		hdr := codec.NewAdtsFrameHeader()
		hdr.Decode(aac)
		if muxer.updateSequence {
			asc, _ := codec.ConvertADTSToASC(aac)
			tags = append(tags, WriteAudioTag(asc.Encode(), FlvAac, 0, 0, true))
			muxer.updateSequence = false
		}
		tags = append(tags, WriteAudioTag(aac[7:], FlvAac, 0, 0, false))
	})
	return tags
}

type G711AMuxer struct {
	channelCount int
	sampleRate   int
}

func NewG711AMuxer(channelCount int, sampleRate int) *G711AMuxer {
	return &G711AMuxer{
		channelCount: channelCount,
		sampleRate:   sampleRate,
	}
}

func (muxer *G711AMuxer) Write(frames []byte, pts uint32, dts uint32) [][]byte {
	tags := make([][]byte, 1)
	tags[0] = WriteAudioTag(frames, FlvG711a, muxer.sampleRate, muxer.channelCount, true)
	return tags
}

type G711UMuxer struct {
	channelCount int
	sampleRate   int
}

func NewG711UMuxer(channelCount int, sampleRate int) *G711UMuxer {
	return &G711UMuxer{
		channelCount: channelCount,
		sampleRate:   sampleRate,
	}
}

func (muxer *G711UMuxer) Write(frames []byte, pts uint32, dts uint32) [][]byte {
	tags := make([][]byte, 1)
	tags[0] = WriteAudioTag(frames, FlvG711u, muxer.sampleRate, muxer.channelCount, true)
	return tags
}

type Mp3Muxer struct {
}

func (muxer *Mp3Muxer) Write(frames []byte, pts uint32, dts uint32) [][]byte {
	tags := make([][]byte, 1)
	codec.SplitMp3Frames(frames, func(head *codec.MP3FrameHead, frame []byte) {
		tags = append(tags, WriteAudioTag(frames, FlvMp3, head.GetSampleRate(), head.GetChannelCount(), true))
	})
	return tags
}

func CreateAudioMuxer(cid FlvSoundFormat) AVTagMuxer {
	if cid == FlvAac {
		return &AACMuxer{updateSequence: true}
	} else if cid == FlvG711a {
		return NewG711AMuxer(1, 5500)
	} else if cid == FlvG711u {
		return NewG711UMuxer(1, 5500)
	} else if cid == FlvMp3 {
		return new(Mp3Muxer)
	} else {
		return nil
	}
}

type FlvMuxer struct {
	videoMuxer AVTagMuxer
	audioMuxer AVTagMuxer
}

func NewFlvMuxer(vid FlvVideoCodecId, aid FlvSoundFormat) *FlvMuxer {
	return &FlvMuxer{
		videoMuxer: CreateVideoMuxer(vid),
		audioMuxer: CreateAudioMuxer(aid),
	}
}

func (muxer *FlvMuxer) SetVideoCodeId(cid FlvVideoCodecId) {
	muxer.videoMuxer = CreateVideoMuxer(cid)
}

func (muxer *FlvMuxer) SetAudioCodeId(cid FlvSoundFormat) {
	muxer.audioMuxer = CreateAudioMuxer(cid)
}

func (muxer *FlvMuxer) WriteVideo(frames []byte, pts uint32, dts uint32) ([][]byte, error) {
	if muxer.videoMuxer == nil {
		return nil, errors.New("video Muxer is Nil")
	}
	return muxer.WriteFrames(VideoTagType, frames, pts, dts)
}

func (muxer *FlvMuxer) WriteAudio(frames []byte, pts uint32, dts uint32) ([][]byte, error) {
	if muxer.audioMuxer == nil {
		return nil, errors.New("audio Muxer is Nil")
	}
	return muxer.WriteFrames(AudioTagType, frames, pts, dts)
}

func (muxer *FlvMuxer) WriteFrames(frameType TagType, frames []byte, pts uint32, dts uint32) ([][]byte, error) {

	var ftag FlvTag
	var tags [][]byte
	if frameType == AudioTagType {
		ftag.TagType = uint8(AudioTagType)
		tags = muxer.audioMuxer.Write(frames, pts, dts)
	} else if frameType == VideoTagType {
		ftag.TagType = uint8(VideoTagType)
		tags = muxer.videoMuxer.Write(frames, pts, dts)
	} else {
		return nil, errors.New("unsupport Frame Type")
	}
	ftag.Timestamp = dts & 0x00FFFFFF
	ftag.TimestampExtended = uint8(dts >> 24 & 0xFF)

	tmptags := make([][]byte, 0, 1)
	for _, tag := range tags {
		ftag.DataSize = uint32(len(tag))
		vtag := ftag.Encode()
		vtag = append(vtag, tag...)
		tmptags = append(tmptags, vtag)
	}
	return tmptags, nil
}
