package mpeg2

import (
	"errors"
	"io"

	"github.com/timmattison/gomedia/go-codec"
)

type pakcetT struct {
	payload []byte
	pts     uint64
	dts     uint64
}

func newpacketT(size uint32) *pakcetT {
	return &pakcetT{
		payload: make([]byte, 0, size),
		pts:     0,
		dts:     0,
	}
}

type tsstream struct {
	cid    TsStreamType
	pesSid PesStremaId
	pesPkg *PesPacket
	pkg    *pakcetT
}

type tsprogram struct {
	pn      uint16
	streams map[uint16]*tsstream
}

type TSDemuxer struct {
	programs   map[uint16]*tsprogram
	OnFrame    func(cid TsStreamType, frame []byte, pts uint64, dts uint64)
	OnTSPacket func(pkg *TSPacket)
}

func NewTSDemuxer() *TSDemuxer {
	return &TSDemuxer{
		programs:   make(map[uint16]*tsprogram),
		OnFrame:    nil,
		OnTSPacket: nil,
	}
}

func (demuxer *TSDemuxer) Input(r io.Reader) error {
	var err error = nil
	var buf []byte
	for {
		if len(buf) > TsPakcetSize {
			buf = buf[TsPakcetSize:]
		} else {
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
					break
				}
				return err
			}
			buf, err = demuxer.probe(r)
			if err != nil && buf == nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return err
			}
		}

		bs := codec.NewBitStream(buf[:TsPakcetSize])
		var pkg TSPacket
		if err := pkg.DecodeHeader(bs); err != nil {
			return err
		}
		if pkg.PID == uint16(TsPidPat) {
			if pkg.PayloadUnitStartIndicator == 1 {
				bs.SkipBits(8)
			}
			pkg.Payload, err = ReadSection(TsTidPas, bs)
			if err != nil {
				return err
			}
			pat := pkg.Payload.(*Pat)
			for _, pmt := range pat.Pmts {
				if pmt.ProgramNumber != 0x0000 {
					if _, found := demuxer.programs[pmt.PID]; !found {
						demuxer.programs[pmt.PID] = &tsprogram{pn: 0, streams: make(map[uint16]*tsstream)}
					}
				}
			}
		} else if pkg.PID == TsPidNil {
			continue
		} else {
			for p, s := range demuxer.programs {
				if p == pkg.PID { // pmt table
					if pkg.PayloadUnitStartIndicator == 1 {
						bs.SkipBits(8) //pointer filed
					}
					pkg.Payload, err = ReadSection(TsTidPms, bs)
					if err != nil {
						return err
					}
					pmt := pkg.Payload.(*Pmt)
					s.pn = pmt.ProgramNumber
					for _, ps := range pmt.Streams {
						if _, found := s.streams[ps.ElementaryPid]; !found {
							s.streams[ps.ElementaryPid] = &tsstream{
								cid:    TsStreamType(ps.StreamType),
								pesSid: findPESIDByStreamType(TsStreamType(ps.StreamType)),
								pesPkg: NewPesPacket(),
							}
						}
					}
				} else {
					for sid, stream := range s.streams {
						if sid != pkg.PID {
							continue
						}
						if pkg.PayloadUnitStartIndicator == 1 {
							err := stream.pesPkg.Decode(bs)
							// ignore error if it was a short payload read, next ts packet should append missing data
							if err != nil && !(errors.Is(err, errNeedMore) && stream.pesPkg.PesPayload != nil) {
								return err
							}
							pkg.Payload = stream.pesPkg
						} else {
							stream.pesPkg.PesPayload = bs.RemainData()
							pkg.Payload = bs.RemainData()
						}
						stype := findPESIDByStreamType(stream.cid)
						if stype == PesStreamAudio {
							demuxer.doAudioPesPacket(stream, pkg.PayloadUnitStartIndicator)
						} else if stype == PesStreamVideo {
							demuxer.doVideoPesPacket(stream, pkg.PayloadUnitStartIndicator)
						}
					}
				}
			}
		}
		if demuxer.OnTSPacket != nil {
			demuxer.OnTSPacket(&pkg)
		}
	}
	demuxer.flush()
	return nil
}

func (demuxer *TSDemuxer) probe(r io.Reader) ([]byte, error) {
	buf := make([]byte, TsPakcetSize, 2*TsPakcetSize)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	if buf[0] == 0x47 {
		return buf, nil
	}
	buf = buf[:2*TsPakcetSize]
	if _, err := io.ReadFull(r, buf[TsPakcetSize:]); err != nil {
		return nil, err
	}
LOOP:
	i := 0
	for ; i < TsPakcetSize; i++ {
		if buf[i] == 0x47 && buf[i+TsPakcetSize] == 0x47 {
			break
		}
	}
	if i == 0 {
		return buf, nil
	} else if i < TsPakcetSize {
		copy(buf, buf[i:])
		if _, err := io.ReadFull(r, buf[2*TsPakcetSize-i:]); err != nil {
			return buf[:TsPakcetSize], err
		} else {
			return buf, nil
		}
	} else {
		copy(buf, buf[TsPakcetSize:])
		if _, err := io.ReadFull(r, buf[TsPakcetSize:]); err != nil {
			return buf[:TsPakcetSize], err
		}
		goto LOOP
	}
}

func (demuxer *TSDemuxer) flush() {
	for _, pm := range demuxer.programs {
		for _, stream := range pm.streams {
			if stream.pkg == nil || len(stream.pkg.payload) == 0 {
				continue
			}

			if demuxer.OnFrame == nil {
				continue
			}
			if stream.cid == TsStreamH264 || stream.cid == TsStreamH265 {
				audLen := 0
				codec.SplitFrameWithStartCode(stream.pkg.payload, func(nalu []byte) bool {
					if stream.cid == TsStreamH264 {
						if codec.H264NaluType(nalu) == codec.H264NalAud {
							audLen += len(nalu)
						}
					} else {
						if codec.H265NaluType(nalu) == codec.H265NalAud {
							audLen += len(nalu)
						}
					}
					return false
				})
				demuxer.OnFrame(stream.cid, stream.pkg.payload[audLen:], stream.pkg.pts/90, stream.pkg.dts/90)
			} else {
				demuxer.OnFrame(stream.cid, stream.pkg.payload, stream.pkg.pts/90, stream.pkg.dts/90)
			}
			stream.pkg = nil
		}
	}
}

func (demuxer *TSDemuxer) doVideoPesPacket(stream *tsstream, start uint8) {
	if stream.cid != TsStreamH264 && stream.cid != TsStreamH265 {
		return
	}
	if stream.pkg == nil {
		stream.pkg = newpacketT(1024)
		stream.pkg.pts = stream.pesPkg.Pts
		stream.pkg.dts = stream.pesPkg.Dts
	}
	stream.pkg.payload = append(stream.pkg.payload, stream.pesPkg.PesPayload...)
	update := false
	if stream.cid == TsStreamH264 {
		update = demuxer.splitH264Frame(stream)
	} else {
		update = demuxer.splitH265Frame(stream)
	}
	if update {
		stream.pkg.pts = stream.pesPkg.Pts
		stream.pkg.dts = stream.pesPkg.Dts
	}
}

func (demuxer *TSDemuxer) doAudioPesPacket(stream *tsstream, start uint8) {
	if stream.cid != TsStreamAac && stream.cid != TsStreamAudioMpeg1 && stream.cid != TsStreamAudioMpeg2 {
		return
	}

	if stream.pkg == nil {
		stream.pkg = newpacketT(1024)
		stream.pkg.pts = stream.pesPkg.Pts
		stream.pkg.dts = stream.pesPkg.Dts
	}

	if len(stream.pkg.payload) > 0 && (start == 1 || stream.pesPkg.Pts != stream.pkg.pts) {
		if demuxer.OnFrame != nil {
			demuxer.OnFrame(stream.cid, stream.pkg.payload, stream.pkg.pts/90, stream.pkg.dts/90)
		}
		stream.pkg.payload = stream.pkg.payload[:0]
	}
	stream.pkg.payload = append(stream.pkg.payload, stream.pesPkg.PesPayload...)
	stream.pkg.pts = stream.pesPkg.Pts
	stream.pkg.dts = stream.pesPkg.Dts
}

func (demuxer *TSDemuxer) splitH264Frame(stream *tsstream) bool {
	data := stream.pkg.payload
	start, sct := codec.FindStartCode(data, 0)
	datalen := len(data)
	vcl := 0
	newAcessUnit := false
	needUpdate := false
	frameBeg := start
	if frameBeg < 0 {
		frameBeg = 0
	}
	for start < datalen {
		if start < 0 || len(data)-start <= int(sct)+1 {
			break
		}

		naluType := codec.H264NaluTypeWithoutStartCode(data[start+int(sct):])
		switch naluType {
		case codec.H264NalAud, codec.H264NalSps,
			codec.H264NalPps, codec.H264NalSei:
			if vcl > 0 {
				newAcessUnit = true
			}
		case codec.H264NalISlice, codec.H264NalPSlice,
			codec.H264NalSliceA, codec.H264NalSliceB, codec.H264NalSliceC:
			if vcl > 0 {
				// bs := codec.NewBitStream(data[start+int(sct)+1:])
				// sliceHdr := &codec.SliceHeader{}
				// sliceHdr.Decode(bs)
				if data[start+int(sct)+1]&0x80 > 0 {
					newAcessUnit = true
				}
			} else {
				vcl++
			}
		}

		if vcl > 0 && newAcessUnit {
			if demuxer.OnFrame != nil {
				audLen := 0
				codec.SplitFrameWithStartCode(data[frameBeg:start], func(nalu []byte) bool {
					if codec.H264NaluType(nalu) == codec.H264NalAud {
						audLen += len(nalu)
					}
					return false
				})
				demuxer.OnFrame(stream.cid, data[frameBeg+audLen:start], stream.pkg.pts/90, stream.pkg.dts/90)
			}
			frameBeg = start
			needUpdate = true
			vcl = 0
			newAcessUnit = false
		}
		end, sct2 := codec.FindStartCode(data, start+3)
		if end < 0 {
			break
		}
		start = end
		sct = sct2
	}

	if frameBeg == 0 {
		return needUpdate
	}
	copy(stream.pkg.payload, data[frameBeg:datalen])
	stream.pkg.payload = stream.pkg.payload[0 : datalen-frameBeg]
	return needUpdate
}

func (demuxer *TSDemuxer) splitH265Frame(stream *tsstream) bool {
	data := stream.pkg.payload
	start, sct := codec.FindStartCode(data, 0)
	datalen := len(data)
	vcl := 0
	newAcessUnit := false
	needUpdate := false
	frameBeg := start
	for start < datalen {
		if len(data)-start <= int(sct)+2 {
			break
		}
		naluType := codec.H265NaluTypeWithoutStartCode(data[start+int(sct):])
		switch naluType {
		case codec.H265NalAud, codec.H265NalSps,
			codec.H265NalPps, codec.H265NalVps, codec.H265NalSei:
			if vcl > 0 {
				newAcessUnit = true
			}
		case codec.H265NalSliceTrailN, codec.H265NalLiceTrailR,
			codec.H265NalSliceTsaN, codec.H265NalSliceTsaR,
			codec.H265NalSliceStsaN, codec.H265NalSliceStsaR,
			codec.H265NalSliceRadlN, codec.H265NalSliceRadlR,
			codec.H265NalSliceRaslN, codec.H265NalSliceRaslR,
			codec.H265NalSliceBlaWLp, codec.H265NalSliceBlaWRadl,
			codec.H265NalSliceBlaNLp, codec.H265NalSliceIdrWRadl,
			codec.H265NalSliceIdrNLp, codec.H265NalSliceCra:
			if vcl > 0 {
				// bs := codec.NewBitStream(data[start+int(sct)+2:])
				// sliceHdr := &codec.SliceHeader{}
				// sliceHdr.Decode(bs)
				// if sliceHdr.First_mb_in_slice == 0 {
				//     newAcessUnit = true
				// }
				if data[start+int(sct)+2]&0x80 > 0 {
					newAcessUnit = true
				}
			} else {
				vcl++
			}
		}

		if vcl > 0 && newAcessUnit {
			if demuxer.OnFrame != nil {
				audLen := 0
				codec.SplitFrameWithStartCode(data[frameBeg:start], func(nalu []byte) bool {
					if codec.H265NaluType(nalu) == codec.H265NalAud {
						audLen = len(nalu)
					}
					return false
				})
				demuxer.OnFrame(stream.cid, data[frameBeg+audLen:start], stream.pkg.pts/90, stream.pkg.dts/90)
			}
			frameBeg = start
			needUpdate = true
			vcl = 0
			newAcessUnit = false
		}

		end, sct2 := codec.FindStartCode(data, start+3)
		if end < 0 {
			break
		}
		start = end
		sct = sct2
	}

	if frameBeg == 0 {
		return needUpdate
	}
	copy(stream.pkg.payload, data[frameBeg:datalen])
	stream.pkg.payload = stream.pkg.payload[0 : datalen-frameBeg]
	return needUpdate
}
