package mpeg2

import (
	"fmt"
	"os"

	"github.com/timmattison/gomedia/go-codec"
)

var H264AudNalu = []byte{0x00, 0x00, 0x00, 0x01, 0x09, 0xF0} //ffmpeg mpegtsenc.c mpegts_write_packet_internal
var H265AudNalu = []byte{0x00, 0x00, 0x00, 0x01, 0x46, 0x01, 0x50}

type PesStremaId int

const (
	PesStreamEnd        PesStremaId = 0xB9
	PesStreamStart      PesStremaId = 0xBA
	PesStreamSystemHead PesStremaId = 0xBB
	PesStreamMap        PesStremaId = 0xBC
	PesStreamPrivate    PesStremaId = 0xBD
	PesStreamAudio      PesStremaId = 0xC0
	PesStreamVideo      PesStremaId = 0xE0
)

func findPESIDByStreamType(cid TsStreamType) PesStremaId {

	switch cid {
	case TsStreamAac, TsStreamAudioMpeg1, TsStreamAudioMpeg2:
		return PesStreamAudio
	case TsStreamH264, TsStreamH265:
		return PesStreamVideo
	default:
		return PesStreamPrivate
	}
}

type PesPacket struct {
	StreamId               uint8
	PesPacketLength        uint16
	PesScramblingControl   uint8
	PesPriority            uint8
	DataAlignmentIndicator uint8
	Copyright              uint8
	OriginalOrCopy         uint8
	PtsDtsFlags            uint8
	EscrFlag               uint8
	EsRateFlag             uint8
	DsmTrickModeFlag       uint8
	AdditionalCopyInfoFlag uint8
	PesCrcFlag             uint8
	PesExtensionFlag       uint8
	PesHeaderDataLength    uint8
	Pts                    uint64
	Dts                    uint64
	EscrBase               uint64
	EscrExtension          uint16
	EsRate                 uint32
	TrickModeControl       uint8
	TrickValue             uint8
	AdditionalCopyInfo     uint8
	PreviousPesPacketCrc   uint16
	PesPayload             []byte
	//TODO
	//if ( PES_extension_flag == '1')
	// PES_private_data_flag                uint8
	// pack_header_field_flag               uint8
	// program_packet_sequence_counter_flag uint8
	// P_STD_buffer_flag                    uint8
	// PES_extension_flag_2                 uint8
	// PES_private_data                     [16]byte
}

func NewPesPacket() *PesPacket {
	return new(PesPacket)
}

func (pkg *PesPacket) PrettyPrint(file *os.File) {
	file.WriteString(fmt.Sprintf("stream_id:%d\n", pkg.StreamId))
	file.WriteString(fmt.Sprintf("PES_packet_length:%d\n", pkg.PesPacketLength))
	file.WriteString(fmt.Sprintf("PES_scrambling_control:%d\n", pkg.PesScramblingControl))
	file.WriteString(fmt.Sprintf("PES_priority:%d\n", pkg.PesPriority))
	file.WriteString(fmt.Sprintf("data_alignment_indicator:%d\n", pkg.DataAlignmentIndicator))
	file.WriteString(fmt.Sprintf("copyright:%d\n", pkg.Copyright))
	file.WriteString(fmt.Sprintf("original_or_copy:%d\n", pkg.OriginalOrCopy))
	file.WriteString(fmt.Sprintf("PTS_DTS_flags:%d\n", pkg.PtsDtsFlags))
	file.WriteString(fmt.Sprintf("ESCR_flag:%d\n", pkg.EscrFlag))
	file.WriteString(fmt.Sprintf("ES_rate_flag:%d\n", pkg.EsRateFlag))
	file.WriteString(fmt.Sprintf("DSM_trick_mode_flag:%d\n", pkg.DsmTrickModeFlag))
	file.WriteString(fmt.Sprintf("additional_copy_info_flag:%d\n", pkg.AdditionalCopyInfoFlag))
	file.WriteString(fmt.Sprintf("PES_CRC_flag:%d\n", pkg.PesCrcFlag))
	file.WriteString(fmt.Sprintf("PES_extension_flag:%d\n", pkg.PesExtensionFlag))
	file.WriteString(fmt.Sprintf("PES_header_data_length:%d\n", pkg.PesHeaderDataLength))
	if pkg.PtsDtsFlags&0x02 == 0x02 {
		file.WriteString(fmt.Sprintf("PTS:%d\n", pkg.Pts))
	}
	if pkg.PtsDtsFlags&0x03 == 0x03 {
		file.WriteString(fmt.Sprintf("DTS:%d\n", pkg.Dts))
	}

	if pkg.EscrFlag == 1 {
		file.WriteString(fmt.Sprintf("ESCR_base:%d\n", pkg.EscrBase))
		file.WriteString(fmt.Sprintf("ESCR_extension:%d\n", pkg.EscrExtension))
	}

	if pkg.EsRateFlag == 1 {
		file.WriteString(fmt.Sprintf("ES_rate:%d\n", pkg.EsRate))
	}

	if pkg.DsmTrickModeFlag == 1 {
		file.WriteString(fmt.Sprintf("trick_mode_control:%d\n", pkg.TrickModeControl))
	}

	if pkg.AdditionalCopyInfoFlag == 1 {
		file.WriteString(fmt.Sprintf("additional_copy_info:%d\n", pkg.AdditionalCopyInfo))
	}

	if pkg.PesCrcFlag == 1 {
		file.WriteString(fmt.Sprintf("previous_PES_packet_CRC:%d\n", pkg.PreviousPesPacketCrc))
	}
	file.WriteString("PES_packet_data_byte:\n")
	file.WriteString(fmt.Sprintf("  Size: %d\n", len(pkg.PesPayload)))
	file.WriteString("  data:")
	for i := 0; i < 12 && i < len(pkg.PesPayload); i++ {
		if i%4 == 0 {
			file.WriteString("\n")
			file.WriteString("      ")
		}
		file.WriteString(fmt.Sprintf("0x%02x ", pkg.PesPayload[i]))
	}
	file.WriteString("\n")
}

func (pkg *PesPacket) Decode(bs *codec.BitStream) error {
	if bs.RemainBytes() < 9 {
		return errNeedMore
	}
	bs.SkipBits(24)            //packet_start_code_prefix
	pkg.StreamId = bs.Uint8(8) //stream_id
	pkg.PesPacketLength = bs.Uint16(16)
	bs.SkipBits(2) //'10'
	pkg.PesScramblingControl = bs.Uint8(2)
	pkg.PesPriority = bs.Uint8(1)
	pkg.DataAlignmentIndicator = bs.Uint8(1)
	pkg.Copyright = bs.Uint8(1)
	pkg.OriginalOrCopy = bs.Uint8(1)
	pkg.PtsDtsFlags = bs.Uint8(2)
	pkg.EscrFlag = bs.Uint8(1)
	pkg.EsRateFlag = bs.Uint8(1)
	pkg.DsmTrickModeFlag = bs.Uint8(1)
	pkg.AdditionalCopyInfoFlag = bs.Uint8(1)
	pkg.PesCrcFlag = bs.Uint8(1)
	pkg.PesExtensionFlag = bs.Uint8(1)
	pkg.PesHeaderDataLength = bs.Uint8(8)
	if bs.RemainBytes() < int(pkg.PesHeaderDataLength) {
		bs.UnRead(9 * 8)
		return errNeedMore
	}
	bs.Markdot()
	if pkg.PtsDtsFlags&0x02 == 0x02 {
		bs.SkipBits(4)
		pkg.Pts = bs.GetBits(3)
		bs.SkipBits(1)
		pkg.Pts = (pkg.Pts << 15) | bs.GetBits(15)
		bs.SkipBits(1)
		pkg.Pts = (pkg.Pts << 15) | bs.GetBits(15)
		bs.SkipBits(1)
	}
	if pkg.PtsDtsFlags&0x03 == 0x03 {
		bs.SkipBits(4)
		pkg.Dts = bs.GetBits(3)
		bs.SkipBits(1)
		pkg.Dts = (pkg.Dts << 15) | bs.GetBits(15)
		bs.SkipBits(1)
		pkg.Dts = (pkg.Dts << 15) | bs.GetBits(15)
		bs.SkipBits(1)
	} else {
		pkg.Dts = pkg.Pts
	}

	if pkg.EscrFlag == 1 {
		bs.SkipBits(2)
		pkg.EscrBase = bs.GetBits(3)
		bs.SkipBits(1)
		pkg.EscrBase = (pkg.Pts << 15) | bs.GetBits(15)
		bs.SkipBits(1)
		pkg.EscrBase = (pkg.Pts << 15) | bs.GetBits(15)
		bs.SkipBits(1)
		pkg.EscrExtension = bs.Uint16(9)
		bs.SkipBits(1)
	}

	if pkg.EsRateFlag == 1 {
		bs.SkipBits(1)
		pkg.EsRate = bs.Uint32(22)
		bs.SkipBits(1)
	}

	if pkg.DsmTrickModeFlag == 1 {
		pkg.TrickModeControl = bs.Uint8(3)
		pkg.TrickValue = bs.Uint8(5)
	}

	if pkg.AdditionalCopyInfoFlag == 1 {
		pkg.AdditionalCopyInfo = bs.Uint8(7)
	}

	if pkg.PesCrcFlag == 1 {
		pkg.PreviousPesPacketCrc = bs.Uint16(16)
	}

	loc := bs.DistanceFromMarkDot()
	bs.SkipBits(int(pkg.PesHeaderDataLength)*8 - loc) // skip remaining header

	// the -3 bytes are the combined lengths
	// of all fields between PES_packet_length and PES_header_data_length (2 bytes)
	// and the PES_header_data_length itself (1 byte)
	dataLen := int(pkg.PesPacketLength - 3 - uint16(pkg.PesHeaderDataLength))

	if bs.RemainBytes() < dataLen {
		pkg.PesPayload = bs.RemainData()
		bs.UnRead((9 + int(pkg.PesHeaderDataLength)) * 8)
		return errNeedMore
	}

	if pkg.PesPacketLength == 0 || bs.RemainBytes() <= dataLen {
		pkg.PesPayload = bs.RemainData()
		bs.SkipBits(bs.RemainBits())
	} else {
		pkg.PesPayload = bs.RemainData()[:dataLen]
		bs.SkipBits(dataLen * 8)
	}

	return nil
}

func (pkg *PesPacket) DecodeMpeg1(bs *codec.BitStream) error {
	if bs.RemainBytes() < 6 {
		return errNeedMore
	}
	bs.SkipBits(24)            //packet_start_code_prefix
	pkg.StreamId = bs.Uint8(8) //stream_id
	pkg.PesPacketLength = bs.Uint16(16)
	if pkg.PesPacketLength != 0 && bs.RemainBytes() < int(pkg.PesPacketLength) {
		bs.UnRead(6 * 8)
		return errNeedMore
	}
	bs.Markdot()
	for bs.NextBits(8) == 0xFF {
		bs.SkipBits(8)
	}
	if bs.NextBits(2) == 0x01 {
		bs.SkipBits(16)
	}
	if bs.NextBits(4) == 0x02 {
		bs.SkipBits(4)
		pkg.Pts = bs.GetBits(3)
		bs.SkipBits(1)
		pkg.Pts = pkg.Pts<<15 | bs.GetBits(15)
		bs.SkipBits(1)
		pkg.Pts = pkg.Pts<<15 | bs.GetBits(15)
		bs.SkipBits(1)
	} else if bs.NextBits(4) == 0x03 {
		bs.SkipBits(4)
		pkg.Pts = bs.GetBits(3)
		bs.SkipBits(1)
		pkg.Pts = pkg.Pts<<15 | bs.GetBits(15)
		bs.SkipBits(1)
		pkg.Pts = pkg.Pts<<15 | bs.GetBits(15)
		bs.SkipBits(1)
		pkg.Dts = bs.GetBits(3)
		bs.SkipBits(1)
		pkg.Dts = pkg.Pts<<15 | bs.GetBits(15)
		bs.SkipBits(1)
		pkg.Dts = pkg.Pts<<15 | bs.GetBits(15)
		bs.SkipBits(1)
	} else if bs.NextBits(8) == 0x0F {
		bs.SkipBits(8)
	} else {
		return errParser
	}
	loc := bs.DistanceFromMarkDot() / 8
	if pkg.PesPacketLength < uint16(loc) {
		return errParser
	}
	if pkg.PesPacketLength == 0 ||
		bs.RemainBits() <= int(pkg.PesPacketLength-uint16(loc))*8 {
		pkg.PesPayload = bs.RemainData()
		bs.SkipBits(bs.RemainBits())
	} else {
		pkg.PesPayload = bs.RemainData()[:pkg.PesPacketLength-uint16(loc)]
		bs.SkipBits(int(pkg.PesPacketLength-uint16(loc)) * 8)
	}
	return nil
}

func (pkg *PesPacket) Encode(bsw *codec.BitStreamWriter) {
	bsw.PutBytes([]byte{0x00, 0x00, 0x01})
	bsw.PutByte(pkg.StreamId)
	bsw.PutUint16(pkg.PesPacketLength, 16)
	bsw.PutUint8(0x02, 2)
	bsw.PutUint8(pkg.PesScramblingControl, 2)
	bsw.PutUint8(pkg.PesPriority, 1)
	bsw.PutUint8(pkg.DataAlignmentIndicator, 1)
	bsw.PutUint8(pkg.Copyright, 1)
	bsw.PutUint8(pkg.OriginalOrCopy, 1)
	bsw.PutUint8(pkg.PtsDtsFlags, 2)
	bsw.PutUint8(pkg.EscrFlag, 1)
	bsw.PutUint8(pkg.EsRateFlag, 1)
	bsw.PutUint8(pkg.DsmTrickModeFlag, 1)
	bsw.PutUint8(pkg.AdditionalCopyInfoFlag, 1)
	bsw.PutUint8(pkg.PesCrcFlag, 1)
	bsw.PutUint8(pkg.PesExtensionFlag, 1)
	bsw.PutByte(pkg.PesHeaderDataLength)
	if pkg.PtsDtsFlags == 0x02 {
		bsw.PutUint8(0x02, 4)
		bsw.PutUint64(pkg.Pts>>30, 3)
		bsw.PutUint8(0x01, 1)
		bsw.PutUint64(pkg.Pts>>15, 15)
		bsw.PutUint8(0x01, 1)
		bsw.PutUint64(pkg.Pts, 15)
		bsw.PutUint8(0x01, 1)
	}

	if pkg.PtsDtsFlags == 0x03 {
		bsw.PutUint8(0x03, 4)
		bsw.PutUint64(pkg.Pts>>30, 3)
		bsw.PutUint8(0x01, 1)
		bsw.PutUint64(pkg.Pts>>15, 15)
		bsw.PutUint8(0x01, 1)
		bsw.PutUint64(pkg.Pts, 15)
		bsw.PutUint8(0x01, 1)
		bsw.PutUint8(0x01, 4)
		bsw.PutUint64(pkg.Dts>>30, 3)
		bsw.PutUint8(0x01, 1)
		bsw.PutUint64(pkg.Dts>>15, 15)
		bsw.PutUint8(0x01, 1)
		bsw.PutUint64(pkg.Dts, 15)
		bsw.PutUint8(0x01, 1)
	}

	if pkg.EscrFlag == 1 {
		bsw.PutUint8(0x03, 2)
		bsw.PutUint64(pkg.EscrBase>>30, 3)
		bsw.PutUint8(0x01, 1)
		bsw.PutUint64(pkg.EscrBase>>15, 15)
		bsw.PutUint8(0x01, 1)
		bsw.PutUint64(pkg.EscrBase, 15)
		bsw.PutUint8(0x01, 1)
	}
	bsw.PutBytes(pkg.PesPayload)
}
