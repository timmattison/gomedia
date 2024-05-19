package mpeg2

import (
	"github.com/timmattison/gomedia/go-codec"
)

type psstream struct {
	sid       uint8
	cid       PsStreamType
	pts       uint64
	dts       uint64
	streamBuf []byte
}

func newpsstream(sid uint8, cid PsStreamType) *psstream {
	return &psstream{
		sid:       sid,
		cid:       cid,
		streamBuf: make([]byte, 0, 4096),
	}
}

type PSDemuxer struct {
	streamMap map[uint8]*psstream
	pkg       *PSPacket
	mpeg1     bool
	cache     []byte
	OnFrame   func(frame []byte, cid PsStreamType, pts uint64, dts uint64)
	//解ps包过程中，解码回调psm，system header，pes包等
	//decodeResult 解码ps包时的产生的错误
	//这个回调主要用于debug，查看是否ps包存在问题
	OnPacket func(pkg Display, decodeResult error)
}

func NewPSDemuxer() *PSDemuxer {
	return &PSDemuxer{
		streamMap: make(map[uint8]*psstream),
		pkg:       new(PSPacket),
		cache:     make([]byte, 0, 256),
		OnFrame:   nil,
		OnPacket:  nil,
	}
}

func (psdemuxer *PSDemuxer) Input(data []byte) error {
	var bs *codec.BitStream
	if len(psdemuxer.cache) > 0 {
		psdemuxer.cache = append(psdemuxer.cache, data...)
		bs = codec.NewBitStream(psdemuxer.cache)
	} else {
		bs = codec.NewBitStream(data)
	}

	saveReseved := func() {
		tmpcache := make([]byte, bs.RemainBytes())
		copy(tmpcache, bs.RemainData())
		psdemuxer.cache = tmpcache
	}

	var ret error = nil
	for !bs.EOS() {
		if mpegerr, ok := ret.(Error); ok {
			if mpegerr.NeedMore() {
				saveReseved()
			}
			break
		}
		if bs.RemainBits() < 32 {
			ret = errNeedMore
			saveReseved()
			break
		}
		prefixCode := bs.NextBits(32)
		switch prefixCode {
		case 0x000001BA: //pack header
			if psdemuxer.pkg.Header == nil {
				psdemuxer.pkg.Header = new(PSPackHeader)
			}
			ret = psdemuxer.pkg.Header.Decode(bs)
			psdemuxer.mpeg1 = psdemuxer.pkg.Header.IsMpeg1
			if psdemuxer.OnPacket != nil {
				psdemuxer.OnPacket(psdemuxer.pkg.Header, ret)
			}
		case 0x000001BB: //system header
			if psdemuxer.pkg.Header == nil {
				panic("psdemuxer.pkg.Header must not be nil")
			}
			if psdemuxer.pkg.System == nil {
				psdemuxer.pkg.System = new(SystemHeader)
			}
			ret = psdemuxer.pkg.System.Decode(bs)
			if psdemuxer.OnPacket != nil {
				psdemuxer.OnPacket(psdemuxer.pkg.System, ret)
			}
		case 0x000001BC: //program stream map
			if psdemuxer.pkg.Psm == nil {
				psdemuxer.pkg.Psm = new(ProgramStreamMap)
			}
			if ret = psdemuxer.pkg.Psm.Decode(bs); ret == nil {
				for _, streaminfo := range psdemuxer.pkg.Psm.StreamMap {
					if _, found := psdemuxer.streamMap[streaminfo.ElementaryStreamId]; !found {
						stream := newpsstream(streaminfo.ElementaryStreamId, PsStreamType(streaminfo.StreamType))
						psdemuxer.streamMap[stream.sid] = stream
					}
				}
			}
			if psdemuxer.OnPacket != nil {
				psdemuxer.OnPacket(psdemuxer.pkg.Psm, ret)
			}
		case 0x000001BD, 0x000001BE, 0x000001BF, 0x000001F0, 0x000001F1,
			0x000001F2, 0x000001F3, 0x000001F4, 0x000001F5, 0x000001F6,
			0x000001F7, 0x000001F8, 0x000001F9, 0x000001FA, 0x000001FB:
			if psdemuxer.pkg.CommPes == nil {
				psdemuxer.pkg.CommPes = new(CommonPesPacket)
			}
			ret = psdemuxer.pkg.CommPes.Decode(bs)
		case 0x000001FF: //program stream directory
			if psdemuxer.pkg.Psd == nil {
				psdemuxer.pkg.Psd = new(ProgramStreamDirectory)
			}
			ret = psdemuxer.pkg.Psd.Decode(bs)
		case 0x000001B9: //MPEG_program_end_code
			continue
		default:
			if prefixCode&0xFFFFFFE0 == 0x000001C0 || prefixCode&0xFFFFFFE0 == 0x000001E0 {
				if psdemuxer.pkg.Pes == nil {
					psdemuxer.pkg.Pes = NewPesPacket()
				}
				if psdemuxer.mpeg1 {
					ret = psdemuxer.pkg.Pes.DecodeMpeg1(bs)
				} else {
					ret = psdemuxer.pkg.Pes.Decode(bs)
				}
				if psdemuxer.OnPacket != nil {
					psdemuxer.OnPacket(psdemuxer.pkg.Pes, ret)
				}
				if ret == nil {
					if stream, found := psdemuxer.streamMap[psdemuxer.pkg.Pes.StreamId]; found {
						if psdemuxer.mpeg1 && stream.cid == PsStreamUnknow {
							psdemuxer.guessCodecid(stream)
						}
						psdemuxer.demuxPespacket(stream, psdemuxer.pkg.Pes)
					} else {
						if psdemuxer.mpeg1 {
							stream := newpsstream(psdemuxer.pkg.Pes.StreamId, PsStreamUnknow)
							psdemuxer.streamMap[stream.sid] = stream
							stream.streamBuf = append(stream.streamBuf, psdemuxer.pkg.Pes.PesPayload...)
							stream.pts = psdemuxer.pkg.Pes.Pts
							stream.dts = psdemuxer.pkg.Pes.Dts
						}
					}
				}
			} else {
				bs.SkipBits(8)
			}
		}
	}

	if ret == nil && len(psdemuxer.cache) > 0 {
		psdemuxer.cache = nil
	}

	return ret
}

func (psdemuxer *PSDemuxer) Flush() {
	for _, stream := range psdemuxer.streamMap {
		if len(stream.streamBuf) == 0 {
			continue
		}
		if psdemuxer.OnFrame != nil {
			psdemuxer.OnFrame(stream.streamBuf, stream.cid, stream.pts/90, stream.dts/90)
		}
	}
}

func (psdemuxer *PSDemuxer) guessCodecid(stream *psstream) {
	if stream.sid&0xE0 == uint8(PesStreamAudio) {
		stream.cid = PsStreamAac
	} else if stream.sid&0xE0 == uint8(PesStreamVideo) {
		h264score := 0
		h265score := 0
		codec.SplitFrame(stream.streamBuf, func(nalu []byte) bool {
			h264nalutype := codec.H264NaluTypeWithoutStartCode(nalu)
			h265nalutype := codec.H265NaluTypeWithoutStartCode(nalu)
			if h264nalutype == codec.H264NalPps ||
				h264nalutype == codec.H264NalSps ||
				h264nalutype == codec.H264NalISlice {
				h264score += 2
			} else if h264nalutype < 5 {
				h264score += 1
			} else if h264nalutype > 20 {
				h264score -= 1
			}

			if h265nalutype == codec.H265NalPps ||
				h265nalutype == codec.H265NalSps ||
				h265nalutype == codec.H265NalVps ||
				(h265nalutype >= codec.H265NalSliceBlaWLp && h265nalutype <= codec.H265NalSliceCra) {
				h265score += 2
			} else if h265nalutype >= codec.H265NalSliceTrailN && h265nalutype <= codec.H265NalSliceRaslR {
				h265score += 1
			} else if h265nalutype > 40 {
				h265score -= 1
			}
			if h264score > h265score && h264score >= 4 {
				stream.cid = PsStreamH264
			} else if h264score < h265score && h265score >= 4 {
				stream.cid = PsStreamH265
			}
			return true
		})
	}
}

func (psdemuxer *PSDemuxer) demuxPespacket(stream *psstream, pes *PesPacket) error {
	switch stream.cid {
	case PsStreamAac, PsStreamG711a, PsStreamG711u:
		return psdemuxer.demuxAudio(stream, pes)
	case PsStreamH264, PsStreamH265:
		return psdemuxer.demuxH26x(stream, pes)
	case PsStreamUnknow:
		if stream.pts != pes.Pts {
			stream.streamBuf = nil
		}
		stream.streamBuf = append(stream.streamBuf, pes.PesPayload...)
		stream.pts = pes.Pts
		stream.dts = pes.Dts
	}
	return nil
}

func (psdemuxer *PSDemuxer) demuxAudio(stream *psstream, pes *PesPacket) error {
	if stream.pts != pes.Pts && len(stream.streamBuf) > 0 {
		if psdemuxer.OnFrame != nil {
			psdemuxer.OnFrame(stream.streamBuf, stream.cid, stream.pts/90, stream.dts/90)
		}
		stream.streamBuf = stream.streamBuf[:0]
	}
	stream.streamBuf = append(stream.streamBuf, pes.PesPayload...)
	stream.pts = pes.Pts
	stream.dts = pes.Dts
	return nil
}

func (psdemuxer *PSDemuxer) demuxH26x(stream *psstream, pes *PesPacket) error {
	if len(stream.streamBuf) == 0 {
		stream.pts = pes.Pts
		stream.dts = pes.Dts
	}
	stream.streamBuf = append(stream.streamBuf, pes.PesPayload...)
	start, sc := codec.FindStartCode(stream.streamBuf, 0)
	for start >= 0 {
		end, sc2 := codec.FindStartCode(stream.streamBuf, start+int(sc))
		if end < 0 {
			break
		}
		if stream.cid == PsStreamH264 {
			naluType := codec.H264NaluType(stream.streamBuf[start:])
			if naluType != codec.H264NalAud {
				if psdemuxer.OnFrame != nil {
					psdemuxer.OnFrame(stream.streamBuf[start:end], stream.cid, stream.pts/90, stream.dts/90)
				}
			}
		} else if stream.cid == PsStreamH265 {
			naluType := codec.H265NaluType(stream.streamBuf[start:])
			if naluType != codec.H265NalAud {
				if psdemuxer.OnFrame != nil {
					psdemuxer.OnFrame(stream.streamBuf[start:end], stream.cid, stream.pts/90, stream.dts/90)
				}
			}
		}
		start = end
		sc = sc2
	}
	stream.streamBuf = stream.streamBuf[start:]
	stream.pts = pes.Pts
	stream.dts = pes.Dts
	return nil
}
