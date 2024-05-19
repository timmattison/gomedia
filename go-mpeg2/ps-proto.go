package mpeg2

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/timmattison/gomedia/go-codec"
)

type Error interface {
	NeedMore() bool
	ParserError() bool
	StreamIdNotFound() bool
}

var errNeedMore error = &needmoreError{}

type needmoreError struct{}

func (e *needmoreError) Error() string          { return "need more bytes" }
func (e *needmoreError) NeedMore() bool         { return true }
func (e *needmoreError) ParserError() bool      { return false }
func (e *needmoreError) StreamIdNotFound() bool { return false }

var errParser error = &parserError{}

type parserError struct{}

func (e *parserError) Error() string          { return "parser packet error" }
func (e *parserError) NeedMore() bool         { return false }
func (e *parserError) ParserError() bool      { return true }
func (e *parserError) StreamIdNotFound() bool { return false }

var errNotFound error = &sidNotFoundError{}

type sidNotFoundError struct{}

func (e *sidNotFoundError) Error() string          { return "stream id not found" }
func (e *sidNotFoundError) NeedMore() bool         { return false }
func (e *sidNotFoundError) ParserError() bool      { return false }
func (e *sidNotFoundError) StreamIdNotFound() bool { return true }

type PsStreamType int

const (
	PsStreamUnknow PsStreamType = 0xFF
	PsStreamAac    PsStreamType = 0x0F
	PsStreamH264   PsStreamType = 0x1B
	PsStreamH265   PsStreamType = 0x24
	PsStreamG711a  PsStreamType = 0x90
	PsStreamG711u  PsStreamType = 0x91
)

// Table 2-33 – Program Stream pack header
// pack_header() {
//     pack_start_code                                     32      bslbf
//     '01'                                                2         bslbf
//     system_clock_reference_base [32..30]                 3         bslbf
//     marker_bit                                           1         bslbf
//     system_clock_reference_base [29..15]                 15         bslbf
//     marker_bit                                           1         bslbf
//     system_clock_reference_base [14..0]                  15         bslbf
//     marker_bit                                           1         bslbf
//     system_clock_reference_extension                     9         uimsbf
//     marker_bit                                           1         bslbf
//     program_mux_rate                                     22        uimsbf
//     marker_bit                                           1        bslbf
//     marker_bit                                           1        bslbf
//     reserved                                             5        bslbf
//     pack_stuffing_length                                 3        uimsbf
//     for (i = 0; i < pack_stuffing_length; i++) {
//             stuffing_byte                               8       bslbf
//     }
//     if (nextbits() == system_header_start_code) {
//             system_header ()
//     }
// }

type PSPackHeader struct {
	IsMpeg1                       bool
	SystemClockReferenceBase      uint64 //33 bits
	SystemClockReferenceExtension uint16 //9 bits
	ProgramMuxRate                uint32 //22 bits
	PackStuffingLength            uint8  //3 bitss
}

func (psPkgHdr *PSPackHeader) PrettyPrint(file *os.File) {
	file.WriteString(fmt.Sprintf("IsMpeg1:%t\n", psPkgHdr.IsMpeg1))
	file.WriteString(fmt.Sprintf("System_clock_reference_base:%d\n", psPkgHdr.SystemClockReferenceBase))
	file.WriteString(fmt.Sprintf("System_clock_reference_extension:%d\n", psPkgHdr.SystemClockReferenceExtension))
	file.WriteString(fmt.Sprintf("Program_mux_rate:%d\n", psPkgHdr.ProgramMuxRate))
	file.WriteString(fmt.Sprintf("Pack_stuffing_length:%d\n", psPkgHdr.PackStuffingLength))
}

func (psPkgHdr *PSPackHeader) Decode(bs *codec.BitStream) error {
	if bs.RemainBytes() < 5 {
		return errNeedMore
	}
	if bs.Uint32(32) != 0x000001BA {
		panic("ps header must start with 000001BA")
	}

	if bs.NextBits(2) == 0x01 { //mpeg2
		if bs.RemainBytes() < 10 {
			return errNeedMore
		}
		return psPkgHdr.decodeMpeg2(bs)
	} else if bs.NextBits(4) == 0x02 { //mpeg1
		if bs.RemainBytes() < 8 {
			return errNeedMore
		}
		psPkgHdr.IsMpeg1 = true
		return psPkgHdr.decodeMpeg1(bs)
	} else {
		return errParser
	}
}

func (psPkgHdr *PSPackHeader) decodeMpeg2(bs *codec.BitStream) error {
	bs.SkipBits(2)
	psPkgHdr.SystemClockReferenceBase = bs.GetBits(3)
	bs.SkipBits(1)
	psPkgHdr.SystemClockReferenceBase = psPkgHdr.SystemClockReferenceBase<<15 | bs.GetBits(15)
	bs.SkipBits(1)
	psPkgHdr.SystemClockReferenceBase = psPkgHdr.SystemClockReferenceBase<<15 | bs.GetBits(15)
	bs.SkipBits(1)
	psPkgHdr.SystemClockReferenceExtension = bs.Uint16(9)
	bs.SkipBits(1)
	psPkgHdr.ProgramMuxRate = bs.Uint32(22)
	bs.SkipBits(1)
	bs.SkipBits(1)
	bs.SkipBits(5)
	psPkgHdr.PackStuffingLength = bs.Uint8(3)
	if bs.RemainBytes() < int(psPkgHdr.PackStuffingLength) {
		bs.UnRead(10 * 8)
		return errNeedMore
	}
	bs.SkipBits(int(psPkgHdr.PackStuffingLength) * 8)
	return nil
}

func (psPkgHdr *PSPackHeader) decodeMpeg1(bs *codec.BitStream) error {
	bs.SkipBits(4)
	psPkgHdr.SystemClockReferenceBase = bs.GetBits(3)
	bs.SkipBits(1)
	psPkgHdr.SystemClockReferenceBase = psPkgHdr.SystemClockReferenceBase<<15 | bs.GetBits(15)
	bs.SkipBits(1)
	psPkgHdr.SystemClockReferenceBase = psPkgHdr.SystemClockReferenceBase<<15 | bs.GetBits(15)
	bs.SkipBits(1)
	psPkgHdr.SystemClockReferenceExtension = 1
	psPkgHdr.ProgramMuxRate = bs.Uint32(7)
	bs.SkipBits(1)
	psPkgHdr.ProgramMuxRate = psPkgHdr.ProgramMuxRate<<15 | bs.Uint32(15)
	bs.SkipBits(1)
	return nil
}

func (psPkgHdr *PSPackHeader) Encode(bsw *codec.BitStreamWriter) {
	bsw.PutBytes([]byte{0x00, 0x00, 0x01, 0xBA})
	bsw.PutUint8(1, 2)
	bsw.PutUint64(psPkgHdr.SystemClockReferenceBase>>30, 3)
	bsw.PutUint8(1, 1)
	bsw.PutUint64(psPkgHdr.SystemClockReferenceBase>>15, 15)
	bsw.PutUint8(1, 1)
	bsw.PutUint64(psPkgHdr.SystemClockReferenceBase, 15)
	bsw.PutUint8(1, 1)
	bsw.PutUint16(psPkgHdr.SystemClockReferenceExtension, 9)
	bsw.PutUint8(1, 1)
	bsw.PutUint32(psPkgHdr.ProgramMuxRate, 22)
	bsw.PutUint8(1, 1)
	bsw.PutUint8(1, 1)
	bsw.PutUint8(0x1F, 5)
	bsw.PutUint8(psPkgHdr.PackStuffingLength, 3)
	bsw.PutRepetValue(0xFF, int(psPkgHdr.PackStuffingLength))
}

type ElementaryStream struct {
	StreamId             uint8
	PStdBufferBoundScale uint8
	PStdBufferSizeBound  uint16
}

func NewelementaryStream(sid uint8) *ElementaryStream {
	return &ElementaryStream{
		StreamId: sid,
	}
}

// system_header () {
//     system_header_start_code         32 bslbf
//     header_length                     16 uimsbf
//     marker_bit                         1  bslbf
//     rate_bound                         22 uimsbf
//     marker_bit                         1  bslbf
//     audio_bound                     6  uimsbf
//     fixed_flag                         1  bslbf
//     CSPS_flag                         1  bslbf
//     system_audio_lock_flag             1  bslbf
//     system_video_lock_flag             1  bslbf
//     marker_bit                      1  bslbf
//     video_bound                     5  uimsbf
//     packet_rate_restriction_flag    1  bslbf
//     reserved_bits                     7  bslbf
//     while (nextbits () == '1') {
//         stream_id                     8  uimsbf
//         '11'                         2  bslbf
//         P-STD_buffer_bound_scale     1  bslbf
//         P-STD_buffer_size_bound     13 uimsbf
//     }
// }

type SystemHeader struct {
	HeaderLength              uint16
	RateBound                 uint32
	AudioBound                uint8
	FixedFlag                 uint8
	CspsFlag                  uint8
	SystemAudioLockFlag       uint8
	SystemVideoLockFlag       uint8
	VideoBound                uint8
	PacketRateRestrictionFlag uint8
	Streams                   []*ElementaryStream
}

func (sh *SystemHeader) PrettyPrint(file *os.File) {
	file.WriteString(fmt.Sprintf("Header_length:%d\n", sh.HeaderLength))
	file.WriteString(fmt.Sprintf("Rate_bound:%d\n", sh.RateBound))
	file.WriteString(fmt.Sprintf("Audio_bound:%d\n", sh.AudioBound))
	file.WriteString(fmt.Sprintf("Fixed_flag:%d\n", sh.FixedFlag))
	file.WriteString(fmt.Sprintf("CSPS_flag:%d\n", sh.CspsFlag))
	file.WriteString(fmt.Sprintf("System_audio_lock_flag:%d\n", sh.SystemAudioLockFlag))
	file.WriteString(fmt.Sprintf("System_video_lock_flag:%d\n", sh.SystemVideoLockFlag))
	file.WriteString(fmt.Sprintf("Video_bound:%d\n", sh.VideoBound))
	file.WriteString(fmt.Sprintf("Packet_rate_restriction_flag:%d\n", sh.PacketRateRestrictionFlag))
	for i, es := range sh.Streams {
		file.WriteString(fmt.Sprintf("----streams %d\n", i))
		file.WriteString(fmt.Sprintf("    Stream_id:%d\n", es.StreamId))
		file.WriteString(fmt.Sprintf("    P_STD_buffer_bound_scale:%d\n", es.PStdBufferBoundScale))
		file.WriteString(fmt.Sprintf("    P_STD_buffer_size_bound:%d\n", es.PStdBufferSizeBound))
	}
}

func (sh *SystemHeader) Encode(bsw *codec.BitStreamWriter) {
	bsw.PutBytes([]byte{0x00, 0x00, 0x01, 0xBB})
	loc := bsw.ByteOffset()
	bsw.PutUint16(0, 16)
	bsw.Markdot()
	bsw.PutUint8(1, 1)
	bsw.PutUint32(sh.RateBound, 22)
	bsw.PutUint8(1, 1)
	bsw.PutUint8(sh.AudioBound, 6)
	bsw.PutUint8(sh.FixedFlag, 1)
	bsw.PutUint8(sh.CspsFlag, 1)
	bsw.PutUint8(sh.SystemAudioLockFlag, 1)
	bsw.PutUint8(sh.SystemVideoLockFlag, 1)
	bsw.PutUint8(1, 1)
	bsw.PutUint8(sh.VideoBound, 5)
	bsw.PutUint8(sh.PacketRateRestrictionFlag, 1)
	bsw.PutUint8(0x7F, 7)
	for _, stream := range sh.Streams {
		bsw.PutUint8(stream.StreamId, 8)
		bsw.PutUint8(3, 2)
		bsw.PutUint8(stream.PStdBufferBoundScale, 1)
		bsw.PutUint16(stream.PStdBufferSizeBound, 13)
	}
	length := bsw.DistanceFromMarkDot() / 8
	bsw.SetUint16(uint16(length), loc)
}

func (sh *SystemHeader) Decode(bs *codec.BitStream) error {
	if bs.RemainBytes() < 12 {
		return errNeedMore
	}
	if bs.Uint32(32) != 0x000001BB {
		panic("system header must start with 000001BB")
	}
	sh.HeaderLength = bs.Uint16(16)
	if bs.RemainBytes() < int(sh.HeaderLength) {
		bs.UnRead(6 * 8)
		return errNeedMore
	}
	if sh.HeaderLength < 6 || (sh.HeaderLength-6)%3 != 0 {
		return errParser
	}
	bs.SkipBits(1)
	sh.RateBound = bs.Uint32(22)
	bs.SkipBits(1)
	sh.AudioBound = bs.Uint8(6)
	sh.FixedFlag = bs.Uint8(1)
	sh.CspsFlag = bs.Uint8(1)
	sh.SystemAudioLockFlag = bs.Uint8(1)
	sh.SystemVideoLockFlag = bs.Uint8(1)
	bs.SkipBits(1)
	sh.VideoBound = bs.Uint8(5)
	sh.PacketRateRestrictionFlag = bs.Uint8(1)
	bs.SkipBits(7)
	sh.Streams = sh.Streams[:0]
	least := sh.HeaderLength - 6
	for least > 0 && bs.NextBits(1) == 0x01 {
		es := new(ElementaryStream)
		es.StreamId = bs.Uint8(8)
		bs.SkipBits(2)
		es.PStdBufferBoundScale = bs.GetBit()
		es.PStdBufferSizeBound = bs.Uint16(13)
		sh.Streams = append(sh.Streams, es)
		least -= 3
	}
	if least > 0 {
		return errParser
	}
	return nil
}

type ElementaryStreamElem struct {
	StreamType                 uint8
	ElementaryStreamId         uint8
	ElementaryStreamInfoLength uint16
}

func NewelementaryStreamElem(stype uint8, esid uint8) *ElementaryStreamElem {
	return &ElementaryStreamElem{
		StreamType:         stype,
		ElementaryStreamId: esid,
	}
}

// program_stream_map() {
//     packet_start_code_prefix             24     bslbf
//     map_stream_id                         8     uimsbf
//     program_stream_map_length             16     uimsbf
//     current_next_indicator                 1     bslbf
//     reserved                             2     bslbf
//     program_stream_map_version             5     uimsbf
//     reserved                             7     bslbf
//     marker_bit                             1     bslbf
//     program_stream_info_length             16     uimsbf
//     for (i = 0; i < N; i++) {
//         descriptor()
//     }
//     elementary_stream_map_length         16     uimsbf
//     for (i = 0; i < N1; i++) {
//         stream_type                         8     uimsbf
//         elementary_stream_id             8     uimsbf
//         elementary_stream_info_length     16    uimsbf
//         for (i = 0; i < N2; i++) {
//             descriptor()
//         }
//     }
//     CRC_32                                 32     rpchof
// }

type ProgramStreamMap struct {
	MapStreamId               uint8
	ProgramStreamMapLength    uint16
	CurrentNextIndicator      uint8
	ProgramStreamMapVersion   uint8
	ProgramStreamInfoLength   uint16
	ElementaryStreamMapLength uint16
	StreamMap                 []*ElementaryStreamElem
}

func (psm *ProgramStreamMap) PrettyPrint(file *os.File) {
	file.WriteString(fmt.Sprintf("map_stream_id:%d\n", psm.MapStreamId))
	file.WriteString(fmt.Sprintf("program_stream_map_length:%d\n", psm.ProgramStreamMapLength))
	file.WriteString(fmt.Sprintf("current_next_indicator:%d\n", psm.CurrentNextIndicator))
	file.WriteString(fmt.Sprintf("program_stream_map_version:%d\n", psm.ProgramStreamMapVersion))
	file.WriteString(fmt.Sprintf("program_stream_info_length:%d\n", psm.ProgramStreamInfoLength))
	file.WriteString(fmt.Sprintf("elementary_stream_map_length:%d\n", psm.ElementaryStreamMapLength))
	for i, es := range psm.StreamMap {
		file.WriteString(fmt.Sprintf("----ES stream %d\n", i))
		if es.StreamType == uint8(PsStreamAac) {
			file.WriteString("    stream_type:AAC\n")
		} else if es.StreamType == uint8(PsStreamG711a) {
			file.WriteString("    stream_type:G711A\n")
		} else if es.StreamType == uint8(PsStreamG711u) {
			file.WriteString("    stream_type:G711U\n")
		} else if es.StreamType == uint8(PsStreamH264) {
			file.WriteString("    stream_type:H264\n")
		} else if es.StreamType == uint8(PsStreamH265) {
			file.WriteString("    stream_type:H265\n")
		}
		file.WriteString(fmt.Sprintf("    elementary_stream_id:%d\n", es.ElementaryStreamId))
		file.WriteString(fmt.Sprintf("    elementary_stream_info_length:%d\n", es.ElementaryStreamInfoLength))
	}
}

func (psm *ProgramStreamMap) Encode(bsw *codec.BitStreamWriter) {
	bsw.PutBytes([]byte{0x00, 0x00, 0x01, 0xBC})
	loc := bsw.ByteOffset()
	bsw.PutUint16(psm.ProgramStreamMapLength, 16)
	bsw.Markdot()
	bsw.PutUint8(psm.CurrentNextIndicator, 1)
	bsw.PutUint8(3, 2)
	bsw.PutUint8(psm.ProgramStreamMapVersion, 5)
	bsw.PutUint8(0x7F, 7)
	bsw.PutUint8(1, 1)
	bsw.PutUint16(0, 16)
	psm.ElementaryStreamMapLength = uint16(len(psm.StreamMap) * 4)
	bsw.PutUint16(psm.ElementaryStreamMapLength, 16)
	for _, streaminfo := range psm.StreamMap {
		bsw.PutUint8(streaminfo.StreamType, 8)
		bsw.PutUint8(streaminfo.ElementaryStreamId, 8)
		bsw.PutUint16(0, 16)
	}
	length := bsw.DistanceFromMarkDot()/8 + 4
	bsw.SetUint16(uint16(length), loc)
	crc := codec.CalcCrc32(0xffffffff, bsw.Bits()[bsw.ByteOffset()-int(length-4)-4:bsw.ByteOffset()])
	tmpcrc := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmpcrc, crc)
	bsw.PutBytes(tmpcrc)
}

func (psm *ProgramStreamMap) Decode(bs *codec.BitStream) error {
	if bs.RemainBytes() < 16 {
		return errNeedMore
	}
	if bs.Uint32(24) != 0x000001 {
		panic("program stream map must startwith 0x000001")
	}
	psm.MapStreamId = bs.Uint8(8)
	if psm.MapStreamId != 0xBC {
		panic("map stream id must be 0xBC")
	}
	psm.ProgramStreamMapLength = bs.Uint16(16)
	if bs.RemainBytes() < int(psm.ProgramStreamMapLength) {
		bs.UnRead(6 * 8)
		return errNeedMore
	}
	psm.CurrentNextIndicator = bs.Uint8(1)
	bs.SkipBits(2)
	psm.ProgramStreamMapVersion = bs.Uint8(5)
	bs.SkipBits(8)
	psm.ProgramStreamInfoLength = bs.Uint16(16)
	if bs.RemainBytes() < int(psm.ProgramStreamInfoLength)+2 {
		bs.UnRead(10 * 8)
		return errNeedMore
	}
	bs.SkipBits(int(psm.ProgramStreamInfoLength) * 8)
	psm.ElementaryStreamMapLength = bs.Uint16(16)
	if psm.ProgramStreamMapLength != 6+psm.ProgramStreamInfoLength+psm.ElementaryStreamMapLength+4 {
		return errParser
	}
	if bs.RemainBytes() < int(psm.ElementaryStreamMapLength)+4 {
		bs.UnRead(12*8 + int(psm.ProgramStreamInfoLength)*8)
		return errNeedMore
	}

	i := 0
	psm.StreamMap = psm.StreamMap[:0]
	for i < int(psm.ElementaryStreamMapLength) {
		elem := new(ElementaryStreamElem)
		elem.StreamType = bs.Uint8(8)
		elem.ElementaryStreamId = bs.Uint8(8)
		elem.ElementaryStreamInfoLength = bs.Uint16(16)
		//TODO Parser descriptor
		if bs.RemainBytes() < int(elem.ElementaryStreamInfoLength) {
			return errParser
		}
		bs.SkipBits(int(elem.ElementaryStreamInfoLength) * 8)
		i += int(4 + elem.ElementaryStreamInfoLength)
		psm.StreamMap = append(psm.StreamMap, elem)
	}

	if i != int(psm.ElementaryStreamMapLength) {
		return errParser
	}

	bs.SkipBits(32)
	return nil
}

type ProgramStreamDirectory struct {
	PesPacketLength uint16
}

func (psd *ProgramStreamDirectory) Decode(bs *codec.BitStream) error {
	if bs.RemainBytes() < 6 {
		return errNeedMore
	}
	if bs.Uint32(32) != 0x000001FF {
		panic("program stream directory 000001FF")
	}
	psd.PesPacketLength = bs.Uint16(16)
	if bs.RemainBytes() < int(psd.PesPacketLength) {
		bs.UnRead(6 * 8)
		return errNeedMore
	}
	//TODO Program Stream directory
	bs.SkipBits(int(psd.PesPacketLength) * 8)
	return nil
}

type CommonPesPacket struct {
	StreamId        uint8
	PesPacketLength uint16
}

func (compes *CommonPesPacket) Decode(bs *codec.BitStream) error {
	if bs.RemainBytes() < 6 {
		return errNeedMore
	}
	bs.SkipBits(24)
	compes.StreamId = bs.Uint8(8)
	compes.PesPacketLength = bs.Uint16(16)
	if bs.RemainBytes() < int(compes.PesPacketLength) {
		bs.UnRead(6 * 8)
		return errNeedMore
	}
	bs.SkipBits(int(compes.PesPacketLength) * 8)
	return nil
}

type PSPacket struct {
	Header  *PSPackHeader
	System  *SystemHeader
	Psm     *ProgramStreamMap
	Psd     *ProgramStreamDirectory
	CommPes *CommonPesPacket
	Pes     *PesPacket
}
