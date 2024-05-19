package mpeg2

import "github.com/timmattison/gomedia/go-codec"

type PSMuxer struct {
	system     *SystemHeader
	psm        *ProgramStreamMap
	OnPacket   func(pkg []byte)
	firstframe bool
}

func NewPsMuxer() *PSMuxer {
	muxer := new(PSMuxer)
	muxer.firstframe = true
	muxer.system = new(SystemHeader)
	muxer.system.RateBound = 26234
	muxer.psm = new(ProgramStreamMap)
	muxer.psm.CurrentNextIndicator = 1
	muxer.psm.ProgramStreamMapVersion = 1
	muxer.OnPacket = nil
	return muxer
}

func (muxer *PSMuxer) AddStream(cid PsStreamType) uint8 {
	if cid == PsStreamH265 || cid == PsStreamH264 {
		es := NewelementaryStream(uint8(PesStreamVideo) + muxer.system.VideoBound)
		es.PStdBufferBoundScale = 1
		es.PStdBufferSizeBound = 400
		muxer.system.Streams = append(muxer.system.Streams, es)
		muxer.system.VideoBound++
		muxer.psm.StreamMap = append(muxer.psm.StreamMap, NewelementaryStreamElem(uint8(cid), es.StreamId))
		muxer.psm.ProgramStreamMapVersion++
		return es.StreamId
	} else {
		es := NewelementaryStream(uint8(PesStreamAudio) + muxer.system.AudioBound)
		es.PStdBufferBoundScale = 0
		es.PStdBufferSizeBound = 32
		muxer.system.Streams = append(muxer.system.Streams, es)
		muxer.system.AudioBound++
		muxer.psm.StreamMap = append(muxer.psm.StreamMap, NewelementaryStreamElem(uint8(cid), es.StreamId))
		muxer.psm.ProgramStreamMapVersion++
		return es.StreamId
	}
}

func (muxer *PSMuxer) Write(sid uint8, frame []byte, pts uint64, dts uint64) error {
	var stream *ElementaryStreamElem = nil
	for _, es := range muxer.psm.StreamMap {
		if es.ElementaryStreamId == sid {
			stream = es
			break
		}
	}
	if stream == nil {
		return errNotFound
	}
	if len(frame) <= 0 {
		return nil
	}
	var withaud = false
	var idrFlag = false
	var first = true
	var vcl = false
	if stream.StreamType == uint8(PsStreamH264) || stream.StreamType == uint8(PsStreamH265) {
		codec.SplitFrame(frame, func(nalu []byte) bool {
			if stream.StreamType == uint8(PsStreamH264) {
				naluType := codec.H264NaluTypeWithoutStartCode(nalu)
				if naluType == codec.H264NalAud {
					withaud = true
					return false
				} else if codec.IsH264VCLNaluType(naluType) {
					if naluType == codec.H264NalISlice {
						idrFlag = true
					}
					vcl = true
					return false
				}
				return true
			} else {
				naluType := codec.H265NaluTypeWithoutStartCode(nalu)
				if naluType == codec.H265NalAud {
					withaud = true
					return false
				} else if codec.IsH265VCLNaluType(naluType) {
					if naluType >= codec.H265NalSliceBlaWLp && naluType <= codec.H265NalSliceCra {
						idrFlag = true
					}
					vcl = true
					return false
				}
				return true
			}
		})
	}

	dts = dts * 90
	pts = pts * 90
	bsw := codec.NewBitStreamWriter(1024)
	var pack PSPackHeader
	pack.SystemClockReferenceBase = dts - 3600
	pack.SystemClockReferenceExtension = 0
	pack.ProgramMuxRate = 6106
	pack.Encode(bsw)
	if muxer.firstframe || idrFlag {
		muxer.system.Encode(bsw)
		muxer.psm.Encode(bsw)
		muxer.firstframe = false
	}
	pespkg := NewPesPacket()
	for len(frame) > 0 {
		peshdrlen := 13
		pespkg.StreamId = sid
		pespkg.PtsDtsFlags = 0x03
		pespkg.PesHeaderDataLength = 10
		pespkg.Pts = pts
		pespkg.Dts = dts
		if idrFlag {
			pespkg.DataAlignmentIndicator = 1
		}
		if first && !withaud && vcl {
			if stream.StreamType == uint8(PsStreamH264) {
				pespkg.PesPayload = append(pespkg.PesPayload, H264AudNalu...)
				peshdrlen += 6
			} else if stream.StreamType == uint8(PsStreamH265) {
				pespkg.PesPayload = append(pespkg.PesPayload, H265AudNalu...)
				peshdrlen += 7
			}
		}
		if peshdrlen+len(frame) >= 0xFFFF {
			pespkg.PesPacketLength = 0xFFFF
			pespkg.PesPayload = append(pespkg.PesPayload, frame[0:0xFFFF-peshdrlen]...)
			frame = frame[0xFFFF-peshdrlen:]
		} else {
			pespkg.PesPacketLength = uint16(peshdrlen + len(frame))
			pespkg.PesPayload = append(pespkg.PesPayload, frame[0:]...)
			frame = frame[:0]
		}
		pespkg.Encode(bsw)
		pespkg.PesPayload = pespkg.PesPayload[:0]
		if muxer.OnPacket != nil {
			muxer.OnPacket(bsw.Bits())
		}
		bsw.Reset()
		first = false
	}
	return nil
}
