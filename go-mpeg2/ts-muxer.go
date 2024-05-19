package mpeg2

import (
	"errors"

	"github.com/timmattison/gomedia/go-codec"
)

type pesStream struct {
	pid        uint16
	cc         uint8
	streamtype TsStreamType
}

func NewPESStream(pid uint16, cid TsStreamType) *pesStream {
	return &pesStream{
		pid:        pid,
		cc:         0,
		streamtype: cid,
	}
}

type tablePmt struct {
	pid           uint16
	cc            uint8
	pcrPid        uint16
	versionNumber uint8
	pm            uint16
	streams       []*pesStream
}

func NewTablePmt() *tablePmt {
	return &tablePmt{
		pid:           0,
		cc:            0,
		pcrPid:        0,
		versionNumber: 0,
		pm:            0,
		streams:       make([]*pesStream, 0, 2),
	}
}

type tablePat struct {
	cc            uint8
	versionNumber uint8
	pmts          []*tablePmt
}

func NewTablePat() *tablePat {
	return &tablePat{
		cc:            0,
		versionNumber: 0,
		pmts:          make([]*tablePmt, 0, 8),
	}
}

type TSMuxer struct {
	pat       *tablePat
	streamPid uint16
	pmtPid    uint16
	patPeriod uint64
	OnPacket  func(pkg []byte)
}

func NewTSMuxer() *TSMuxer {
	return &TSMuxer{
		pat:       NewTablePat(),
		streamPid: 0x100,
		pmtPid:    0x200,
		patPeriod: 0,
		OnPacket:  nil,
	}
}

func (mux *TSMuxer) AddStream(cid TsStreamType) uint16 {
	if mux.pat == nil {
		mux.pat = NewTablePat()
	}
	if len(mux.pat.pmts) == 0 {
		tmppmt := NewTablePmt()
		tmppmt.pid = mux.pmtPid
		tmppmt.pm = 1
		mux.pmtPid++
		mux.pat.pmts = append(mux.pat.pmts, tmppmt)
	}
	sid := mux.streamPid
	tmpstream := NewPESStream(sid, cid)
	mux.streamPid++
	mux.pat.pmts[0].streams = append(mux.pat.pmts[0].streams, tmpstream)
	return sid
}

// / Muxer audio/video stream data
// / pid: stream id by AddStream
// / pts: audio/video stream timestamp in ms
// / dts: audio/video stream timestamp in ms
func (mux *TSMuxer) Write(pid uint16, data []byte, pts uint64, dts uint64) error {
	var whichpmt *tablePmt = nil
	var whichstream *pesStream = nil
	for _, pmt := range mux.pat.pmts {
		for _, stream := range pmt.streams {
			if stream.pid == pid {
				whichpmt = pmt
				whichstream = stream
				break
			}
		}
	}
	if whichpmt == nil || whichstream == nil {
		return errors.New("not Found pid stream")
	}
	if whichpmt.pcrPid == 0 || (findPESIDByStreamType(whichstream.streamtype) == PesStreamVideo && whichpmt.pcrPid != pid) {
		whichpmt.pcrPid = pid
	}

	var withaud = false

	if whichstream.streamtype == TsStreamH264 || whichstream.streamtype == TsStreamH265 {
		codec.SplitFrame(data, func(nalu []byte) bool {
			if whichstream.streamtype == TsStreamH264 {
				naluType := codec.H264NaluTypeWithoutStartCode(nalu)
				if naluType == codec.H264NalAud {
					withaud = true
					return false
				} else if codec.IsH264VCLNaluType(naluType) {
					return false
				}
				return true
			} else {
				naluType := codec.H265NaluTypeWithoutStartCode(nalu)
				if naluType == codec.H265NalAud {
					withaud = true
					return false
				} else if codec.IsH265VCLNaluType(naluType) {
					return false
				}
				return true
			}
		})
	}

	if mux.patPeriod == 0 || mux.patPeriod+400 < dts {
		mux.patPeriod = dts
		if mux.patPeriod == 0 {
			mux.patPeriod = 1 //avoid write pat twice
		}
		tmppat := NewPat()
		tmppat.VersionNumber = mux.pat.versionNumber
		for _, pmt := range mux.pat.pmts {
			tmppm := PmtPair{
				ProgramNumber: pmt.pm,
				PID:           pmt.pid,
			}
			tmppat.Pmts = append(tmppat.Pmts, tmppm)
		}
		mux.writePat(tmppat)

		for _, pmt := range mux.pat.pmts {
			tmppmt := NewPmt()
			tmppmt.ProgramNumber = pmt.pm
			tmppmt.VersionNumber = pmt.versionNumber
			tmppmt.PcrPid = pmt.pcrPid
			for _, stream := range pmt.streams {
				var sp StreamPair
				sp.StreamType = uint8(stream.streamtype)
				sp.ElementaryPid = stream.pid
				sp.EsInfoLength = 0
				tmppmt.Streams = append(tmppmt.Streams, sp)
			}
			mux.writePmt(tmppmt, pmt)
		}
	}

	flag := false
	switch whichstream.streamtype {
	case TsStreamH264:
		flag = codec.IsH264IDRFrame(data)
	case TsStreamH265:
		flag = codec.IsH265IDRFrame(data)
	}

	mux.writePES(whichstream, whichpmt, data, pts*90, dts*90, flag, withaud)
	return nil
}

func (mux *TSMuxer) writePat(pat *Pat) {
	var tshdr TSPacket
	tshdr.PayloadUnitStartIndicator = 1
	tshdr.PID = 0
	tshdr.AdaptationFieldControl = 0x01
	tshdr.ContinuityCounter = mux.pat.cc
	mux.pat.cc = (mux.pat.cc + 1) % 16
	bsw := codec.NewBitStreamWriter(TsPakcetSize)
	tshdr.EncodeHeader(bsw)
	bsw.PutByte(0x00) //pointer
	pat.Encode(bsw)
	bsw.FillRemainData(0xff)
	if mux.OnPacket != nil {
		mux.OnPacket(bsw.Bits())
	}
}

func (mux *TSMuxer) writePmt(pmt *Pmt, tPmt *tablePmt) {
	var tshdr TSPacket
	tshdr.PayloadUnitStartIndicator = 1
	tshdr.PID = tPmt.pid
	tshdr.AdaptationFieldControl = 0x01
	tshdr.ContinuityCounter = tPmt.cc
	tPmt.cc = (tPmt.cc + 1) % 16
	bsw := codec.NewBitStreamWriter(TsPakcetSize)
	tshdr.EncodeHeader(bsw)
	bsw.PutByte(0x00) //pointer
	pmt.Encode(bsw)
	bsw.FillRemainData(0xff)
	if mux.OnPacket != nil {
		mux.OnPacket(bsw.Bits())
	}
}

func (mux *TSMuxer) writePES(pes *pesStream, pmt *tablePmt, data []byte, pts uint64, dts uint64, idrFlag bool, withaud bool) {
	var firstPesPacket = true
	bsw := codec.NewBitStreamWriter(TsPakcetSize)
	for {
		bsw.Reset()
		var tshdr TSPacket
		if firstPesPacket {
			tshdr.PayloadUnitStartIndicator = 1
		}
		tshdr.PID = pes.pid
		tshdr.AdaptationFieldControl = 0x01
		tshdr.ContinuityCounter = pes.cc
		headlen := 4
		pes.cc = (pes.cc + 1) % 16
		var adaptation *AdaptationField = nil
		if firstPesPacket && idrFlag {
			adaptation = new(AdaptationField)
			tshdr.AdaptationFieldControl = tshdr.AdaptationFieldControl | 0x20
			adaptation.RandomAccessIndicator = 1
			headlen += 2
		}

		if firstPesPacket && pes.pid == pmt.pcrPid {
			if adaptation == nil {
				adaptation = new(AdaptationField)
				headlen += 2
			}
			tshdr.AdaptationFieldControl = tshdr.AdaptationFieldControl | 0x20
			adaptation.PcrFlag = 1
			var pcrBase uint64 = 0
			var pcrExt uint16 = 0
			if dts == 0 {
				pcrBase = pts * 300 / 300
				pcrExt = uint16(pts * 300 % 300)
			} else {
				pcrBase = dts * 300 / 300
				pcrExt = uint16(dts * 300 % 300)
			}
			adaptation.ProgramClockReferenceBase = pcrBase
			adaptation.ProgramClockReferenceExtension = pcrExt
			headlen += 6
		}

		var payload []byte
		var pespkg *PesPacket = nil
		if firstPesPacket {
			oldheadlen := headlen
			headlen += 19
			if !withaud && pes.streamtype == TsStreamH264 {
				headlen += 6
				payload = append(payload, H264AudNalu...)
			} else if !withaud && pes.streamtype == TsStreamH265 {
				payload = append(payload, H265AudNalu...)
				headlen += 7
			}
			pespkg = NewPesPacket()
			pespkg.PtsDtsFlags = 0x03
			pespkg.PesHeaderDataLength = 10
			pespkg.Pts = pts
			pespkg.Dts = dts
			pespkg.StreamId = uint8(findPESIDByStreamType(pes.streamtype))
			if idrFlag {
				pespkg.DataAlignmentIndicator = 1
			}
			if headlen-oldheadlen-6+len(data) > 0xFFFF {
				pespkg.PesPacketLength = 0
			} else {
				pespkg.PesPacketLength = uint16(len(data) + headlen - oldheadlen - 6)
			}

		}

		if len(data)+headlen < TsPakcetSize {
			if adaptation == nil {
				adaptation = new(AdaptationField)
				headlen += 1
				if TsPakcetSize-len(data)-headlen >= 1 {
					headlen += 1
				} else {
					adaptation.SingleStuffingByte = true
				}
			}
			adaptation.StuffingByte = uint8(TsPakcetSize - len(data) - headlen)
			payload = append(payload, data...)
			data = data[:0]
		} else {
			payload = append(payload, data[0:TsPakcetSize-headlen]...)
			data = data[TsPakcetSize-headlen:]
		}

		if adaptation != nil {
			tshdr.Field = adaptation
			tshdr.AdaptationFieldControl |= 0x02
		}
		tshdr.EncodeHeader(bsw)
		if pespkg != nil {
			pespkg.PesPayload = payload
			pespkg.Encode(bsw)
		} else {
			bsw.PutBytes(payload)
		}
		firstPesPacket = false
		if mux.OnPacket != nil {
			if len(bsw.Bits()) != TsPakcetSize {
				panic("packet ts packet failed")
			}
			mux.OnPacket(bsw.Bits())
		}
		if len(data) == 0 {
			break
		}
	}
}
