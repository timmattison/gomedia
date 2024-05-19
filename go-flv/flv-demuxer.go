package flv

import (
	"encoding/binary"
	"errors"

	"github.com/timmattison/gomedia/go-codec"
)

type OnVideoFrameCallBack func(codecid codec.CodecID, frame []byte, cts int)
type VideoTagDemuxer interface {
	Decode(data []byte) error
	OnFrame(onframe OnVideoFrameCallBack)
}

type AVCTagDemuxer struct {
	spss    map[uint64][]byte
	ppss    map[uint64][]byte
	onframe OnVideoFrameCallBack
}

func NewAVCTagDemuxer() *AVCTagDemuxer {
	return &AVCTagDemuxer{
		spss:    make(map[uint64][]byte),
		ppss:    make(map[uint64][]byte),
		onframe: nil,
	}
}

func (demuxer *AVCTagDemuxer) OnFrame(onframe OnVideoFrameCallBack) {
	demuxer.onframe = onframe
}

func (demuxer *AVCTagDemuxer) Decode(data []byte) error {

	if len(data) < 5 {
		return errors.New("avc tag size < 5")
	}

	vtag := VideoTag{}
	vtag.Decode(data[0:5])
	data = data[5:]
	if vtag.AVCPacketType == AvcSequenceHeader {
		tmpspss, tmpppss := codec.CovertExtradata(data)
		for _, sps := range tmpspss {
			spsid := codec.GetSPSId(sps)
			tmpsps := make([]byte, len(sps))
			copy(tmpsps, sps)
			demuxer.spss[spsid] = tmpsps
		}
		for _, pps := range tmpppss {
			ppsid := codec.GetPPSId(pps)
			tmppps := make([]byte, len(pps))
			copy(tmppps, pps)
			demuxer.ppss[ppsid] = tmppps
		}
	} else {
		var hassps bool
		var haspps bool
		var idr bool
		tmpdata := data
		for len(tmpdata) > 0 {
			naluSize := binary.BigEndian.Uint32(tmpdata)
			codec.CovertAVCCToAnnexB(tmpdata)
			naluType := codec.H264NaluType(tmpdata)
			if naluType == codec.H264NalISlice {
				idr = true
			} else if naluType == codec.H264NalSps {
				hassps = true
			} else if naluType == codec.H264NalPps {
				haspps = true
			} else if naluType < codec.H264NalISlice {
				sh := codec.SliceHeader{}
				sh.Decode(codec.NewBitStream(tmpdata[5:]))
				if sh.SliceType == 2 || sh.SliceType == 7 {
					idr = true
				}
			}
			tmpdata = tmpdata[4+naluSize:]
		}

		if idr && (!hassps || !haspps) {
			var nalus = make([]byte, 0, 2048)
			for _, sps := range demuxer.spss {
				nalus = append(nalus, sps...)
			}
			for _, pps := range demuxer.ppss {
				nalus = append(nalus, pps...)
			}
			nalus = append(nalus, data...)
			if demuxer.onframe != nil {
				demuxer.onframe(codec.CodecidVideoH264, nalus, int(vtag.CompositionTime))
			}
		} else {
			if demuxer.onframe != nil && len(data) > 0 {
				demuxer.onframe(codec.CodecidVideoH264, data, int(vtag.CompositionTime))
			}
		}
	}
	return nil
}

type HevcTagDemuxer struct {
	SpsPpsVps []byte
	onframe   OnVideoFrameCallBack
}

func NewHevcTagDemuxer() *HevcTagDemuxer {
	return &HevcTagDemuxer{
		SpsPpsVps: make([]byte, 0),
		onframe:   nil,
	}
}

func (demuxer *HevcTagDemuxer) OnFrame(onframe OnVideoFrameCallBack) {
	demuxer.onframe = onframe
}

func (demuxer *HevcTagDemuxer) Decode(data []byte) error {

	if len(data) < 5 {
		return errors.New("hevc tag size < 5")
	}

	vtag := VideoTag{}

	isExHeader := data[0] & 0x80
	if isExHeader != 0 {
		// enhanced flv
		vtag.Decode(data[0:8])
		if vtag.AVCPacketType == PacketTypeSequenceStart {
			data = data[5:]
			hvcc := codec.NewHEVCRecordConfiguration()
			hvcc.Decode(data)
			demuxer.SpsPpsVps = hvcc.ToNalus()
		} else if vtag.AVCPacketType == PacketTypeCodedFrames {
			data = data[8:]
			demuxer.decodeNalus(data, vtag.CompositionTime)
		} else if vtag.AVCPacketType == PacketTypeCodedFramesX {
			data = data[5:]
			demuxer.decodeNalus(data, vtag.CompositionTime)
		}
	} else {
		vtag.Decode(data[0:5])
		data = data[5:]
		if vtag.AVCPacketType == AvcSequenceHeader {
			hvcc := codec.NewHEVCRecordConfiguration()
			hvcc.Decode(data)
			demuxer.SpsPpsVps = hvcc.ToNalus()
		} else {
			demuxer.decodeNalus(data, vtag.CompositionTime)
		}
	}

	return nil
}

func (demuxer *HevcTagDemuxer) decodeNalus(data []byte, CompositionTime int32) error {
	var hassps bool
	var haspps bool
	var hasvps bool
	var idr bool
	tmpdata := data
	for len(tmpdata) > 0 {
		naluSize := binary.BigEndian.Uint32(tmpdata)
		codec.CovertAVCCToAnnexB(tmpdata)
		naluType := codec.H265NaluType(tmpdata)
		if naluType >= 16 && naluType <= 21 {
			idr = true
		} else if naluType == codec.H265NalSps {
			hassps = true
		} else if naluType == codec.H265NalPps {
			haspps = true
		} else if naluType == codec.H265NalVps {
			hasvps = true
		}
		tmpdata = tmpdata[4+naluSize:]
	}

	if idr && (!hassps || !haspps || !hasvps) {
		var nalus = make([]byte, 0, 2048)
		nalus = append(demuxer.SpsPpsVps, data...)
		if demuxer.onframe != nil {
			demuxer.onframe(codec.CodecidVideoH265, nalus, int(CompositionTime))
		}
	} else {
		if demuxer.onframe != nil {
			demuxer.onframe(codec.CodecidVideoH265, data, int(CompositionTime))
		}
	}

	return nil
}

type OnAudioFrameCallBack func(codecid codec.CodecID, frame []byte)

type AudioTagDemuxer interface {
	Decode(data []byte) error
	OnFrame(onframe OnAudioFrameCallBack)
}

type AACTagDemuxer struct {
	asc     []byte
	onframe OnAudioFrameCallBack
}

func NewAACTagDemuxer() *AACTagDemuxer {
	return &AACTagDemuxer{
		asc:     make([]byte, 0, 2),
		onframe: nil,
	}
}

func (demuxer *AACTagDemuxer) OnFrame(onframe OnAudioFrameCallBack) {
	demuxer.onframe = onframe
}

func (demuxer *AACTagDemuxer) Decode(data []byte) error {

	if len(data) < 2 {
		return errors.New("aac tag size < 2")
	}

	atag := AudioTag{}
	err := atag.Decode(data[0:2])
	if err != nil {
		return err
	}
	data = data[2:]
	if atag.AACPacketType == AacSequenceHeader {
		demuxer.asc = make([]byte, len(data))
		copy(demuxer.asc, data)
	} else {
		adts, err := codec.ConvertASCToADTS(demuxer.asc, len(data)+7)
		if err != nil {
			return err
		}
		adtsFrame := append(adts.Encode(), data...)
		if demuxer.onframe != nil {
			demuxer.onframe(codec.CodecidAudioAac, adtsFrame)
		}
	}
	return nil
}

type G711Demuxer struct {
	format  FlvSoundFormat
	onframe OnAudioFrameCallBack
}

func NewG711Demuxer(format FlvSoundFormat) *G711Demuxer {
	return &G711Demuxer{
		format:  format,
		onframe: nil,
	}
}

func (demuxer *G711Demuxer) OnFrame(onframe OnAudioFrameCallBack) {
	demuxer.onframe = onframe
}

func (demuxer *G711Demuxer) Decode(data []byte) error {

	if len(data) < 1 {
		return errors.New("audio tag size < 1")
	}

	atag := AudioTag{}
	err := atag.Decode(data[0:1])
	if err != nil {
		return err
	}
	data = data[1:]

	if demuxer.onframe != nil {
		demuxer.onframe(demuxer.format.ToMpegCodecId(), data)
	}
	return nil
}

func CreateAudioTagDemuxer(formats FlvSoundFormat) (demuxer AudioTagDemuxer) {
	switch formats {
	case FlvG711a, FlvG711u, FlvMp3:
		demuxer = NewG711Demuxer(formats)
	case FlvAac:
		demuxer = NewAACTagDemuxer()
	default:
		panic("unsupport audio codec id")
	}
	return
}

func CreateFlvVideoTagHandle(cid FlvVideoCodecId) (demuxer VideoTagDemuxer) {
	switch cid {
	case FlvAvc:
		demuxer = NewAVCTagDemuxer()
	case FlvHevc:
		demuxer = NewHevcTagDemuxer()
	default:
		panic("unsupport audio codec id")
	}
	return
}
