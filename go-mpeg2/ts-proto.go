package mpeg2

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"

	"github.com/timmattison/gomedia/go-codec"
)

// PID
type TsPid int

const (
	TsPidPat TsPid = 0x0000
	TsPidCat
	TsPidTsdt
	TsPidIpmp
	TsPidNil = 0x1FFF
)

// Table id
type PatTid int

const (
	TsTidPas       PatTid = 0x00 // program_association_section
	TsTidCas              = 0x01 // conditional_access_section(CA_section)
	TsTidPms              = 0x02 // TS_program_map_section
	TsTidSds              = 0x03 //TS_description_section
	TsTidForbidden PatTid = 0xFF
)

type TsStreamType int

const (
	TsStreamAudioMpeg1 TsStreamType = 0x03
	TsStreamAudioMpeg2 TsStreamType = 0x04
	TsStreamAac        TsStreamType = 0x0F
	TsStreamH264       TsStreamType = 0x1B
	TsStreamH265       TsStreamType = 0x24
)

const (
	TsPakcetSize = 188
)

type Display interface {
	PrettyPrint(file *os.File)
}

// transport_packet(){
//     sync_byte                                                                                         8                      bslbf
//     transport_error_indicator                                                                         1                      bslbf
//     payload_unit_start_indicator                                                                     1                      bslbf
//     transport_priority                                                                                 1                      bslbf
//     PID                                                                                             13                     uimsbf
//     transport_scrambling_control                                                                     2                      bslbf
//     adaptation_field_control                                                                        2                      bslbf
//     continuity_counter                                                                                 4                      uimsbf
//     if(adaptation_field_control = = '10' || adaptation_field_control = = '11'){
//          adaptation_field()
//      }
//      if(adaptation_field_control = = '01' || adaptation_field_control = = '11') {
//          for (i = 0; i < N; i++){
//              data_byte                                                                                 8                      bslbf
//          }
//     }
// }

type TSPacket struct {
	TransportErrorIndicator    uint8
	PayloadUnitStartIndicator  uint8
	TransportPriority          uint8
	PID                        uint16
	TransportScramblingControl uint8
	AdaptationFieldControl     uint8
	ContinuityCounter          uint8
	Field                      *AdaptationField
	Payload                    interface{}
}

func (pkg *TSPacket) PrettyPrint(file *os.File) {
	file.WriteString(fmt.Sprintf("Transport_error_indicator:%d\n", pkg.TransportErrorIndicator))
	file.WriteString(fmt.Sprintf("Payload_unit_start_indicator:%d\n", pkg.PayloadUnitStartIndicator))
	file.WriteString(fmt.Sprintf("Transport_priority:%d\n", pkg.TransportPriority))
	file.WriteString(fmt.Sprintf("PID:%d\n", pkg.PID))
	file.WriteString(fmt.Sprintf("Transport_scrambling_control:%d\n", pkg.TransportScramblingControl))
	file.WriteString(fmt.Sprintf("Adaptation_field_control:%d\n", pkg.AdaptationFieldControl))
	file.WriteString(fmt.Sprintf("Continuity_counter:%d\n", pkg.ContinuityCounter))
}

func (pkg *TSPacket) EncodeHeader(bsw *codec.BitStreamWriter) {
	bsw.PutByte(0x47)
	bsw.PutUint8(pkg.TransportErrorIndicator, 1)
	bsw.PutUint8(pkg.PayloadUnitStartIndicator, 1)
	bsw.PutUint8(pkg.TransportPriority, 1)
	bsw.PutUint16(pkg.PID, 13)
	bsw.PutUint8(pkg.TransportScramblingControl, 2)
	bsw.PutUint8(pkg.AdaptationFieldControl, 2)
	bsw.PutUint8(pkg.ContinuityCounter, 4)
	if pkg.Field != nil && (pkg.AdaptationFieldControl&0x02) != 0 {
		pkg.Field.Encode(bsw)
	}
}

func (pkg *TSPacket) DecodeHeader(bs *codec.BitStream) error {
	syncByte := bs.Uint8(8)
	if syncByte != 0x47 {
		return errors.New("ts packet must start with 0x47")
	}
	pkg.TransportErrorIndicator = bs.GetBit()
	pkg.PayloadUnitStartIndicator = bs.GetBit()
	pkg.TransportPriority = bs.GetBit()
	pkg.PID = bs.Uint16(13)
	pkg.TransportScramblingControl = bs.Uint8(2)
	pkg.AdaptationFieldControl = bs.Uint8(2)
	pkg.ContinuityCounter = bs.Uint8(4)
	if pkg.PID == TsPidNil {
		return nil
	}
	if pkg.AdaptationFieldControl == 0x02 || pkg.AdaptationFieldControl == 0x03 {
		if pkg.Field == nil {
			pkg.Field = new(AdaptationField)
		}
		err := pkg.Field.Decode(bs)
		if err != nil {
			return err
		}
	}
	return nil
}

//
// adaptation_field() {
// adaptation_field_length
// if (adaptation_field_length > 0) {
//     discontinuity_indicator
//     random_access_indicator
//     elementary_stream_priority_indicator
//     PCR_flag
//     OPCR_flag
//     splicing_point_flag
//     transport_private_data_flag
//     adaptation_field_extension_flag
//     if (PCR_flag == '1') {
//         program_clock_reference_base
//         reserved
//         program_clock_reference_extension
//     }
//     if (OPCR_flag == '1') {
//         original_program_clock_reference_base
//         reserved
//         original_program_clock_reference_extension
//     }
//     if (splicing_point_flag == '1') {
//         splice_countdown
//     }
//     if (transport_private_data_flag == '1') {
//         transport_private_data_length
//         for (i = 0; i < transport_private_data_length; i++) {
//             private_data_byte
//         }
//     }
//     if (adaptation_field_extension_flag == '1') {
//         adaptation_field_extension_length
//         ltw_flag piecewise_rate_flag
//         seamless_splice_flag
//         reserved
//         if (ltw_flag == '1') {
//             ltw_valid_flag
//             ltw_offset
//         }
//         if (piecewise_rate_flag == '1') {
//             reserved
//             piecewise_rate
//         }
//         if (seamless_splice_flag == '1') {
//             splice_type
//             DTS_next_AU[32..30]
//             marker_bit
//             DTS_next_AU[29..15]
//             marker_bit
//             DTS_next_AU[14..0]
//             marker_bit 1
//         }
//         for (i = 0; i < N; i++) {
//             reserved 8
//         }
//     }
//     for (i = 0; i < N; i++) {
//         stuffing_byte 8
//     }
// }

type AdaptationField struct {
	SingleStuffingByte                     bool   // The value 0 is for inserting a single stuffing byte in a Transport Stream packet
	AdaptationFieldLength                  uint8  //8   uimsbf
	DiscontinuityIndicator                 uint8  //1   bslbf
	RandomAccessIndicator                  uint8  //1   bslbf
	ElementaryStreamPriorityIndicator      uint8  //1   bslbf
	PcrFlag                                uint8  //1   bslbf
	OpcrFlag                               uint8  //1   bslbf
	SplicingPointFlag                      uint8  //1   bslbf
	TransportPrivateDataFlag               uint8  //1   bslbf
	AdaptationFieldExtensionFlag           uint8  //1   bslbf
	ProgramClockReferenceBase              uint64 //33  uimsbf
	ProgramClockReferenceExtension         uint16 //9   uimsbf
	OriginalProgramClockReferenceBase      uint64 //33  uimsbf
	OriginalProgramClockReferenceExtension uint16 //9   uimsbf
	SpliceCountdown                        uint8  //8   uimsbf
	TransportPrivateDataLength             uint8  //8   uimsbf
	AdaptationFieldExtensionLength         uint8  //8   uimsbf
	LtwFlag                                uint8  //1   bslbf
	PiecewiseRateFlag                      uint8  //1   bslbf
	SeamlessSpliceFlag                     uint8  //1   bslbf
	LtwValidFlag                           uint8  //1   bslbf
	LtwOffset                              uint16 //15  uimsbf
	PiecewiseRate                          uint32 //22  uimsbf
	SpliceType                             uint8  //4   uimsbf
	DtsNextAu                              uint64
	StuffingByte                           uint8
}

func (adaptation *AdaptationField) PrettyPrint(file *os.File) {
	file.WriteString(fmt.Sprintf("Adaptation_field_length:%d\n", adaptation.AdaptationFieldLength))
	file.WriteString(fmt.Sprintf("Discontinuity_indicator:%d\n", adaptation.DiscontinuityIndicator))
	file.WriteString(fmt.Sprintf("Random_access_indicator:%d\n", adaptation.RandomAccessIndicator))
	file.WriteString(fmt.Sprintf("Elementary_stream_priority_indicator:%d\n", adaptation.ElementaryStreamPriorityIndicator))
	file.WriteString(fmt.Sprintf("PCR_flag:%d\n", adaptation.PcrFlag))
	file.WriteString(fmt.Sprintf("OPCR_flag:%d\n", adaptation.OpcrFlag))
	file.WriteString(fmt.Sprintf("Splicing_point_flag:%d\n", adaptation.SplicingPointFlag))
	file.WriteString(fmt.Sprintf("Transport_private_data_flag:%d\n", adaptation.TransportPrivateDataFlag))
	file.WriteString(fmt.Sprintf("Adaptation_field_extension_flag:%d\n", adaptation.AdaptationFieldExtensionFlag))
	if adaptation.PcrFlag == 1 {
		file.WriteString(fmt.Sprintf("Program_clock_reference_base:%d\n", adaptation.ProgramClockReferenceBase))
		file.WriteString(fmt.Sprintf("Program_clock_reference_extension:%d\n", adaptation.ProgramClockReferenceExtension))
	}
	if adaptation.OpcrFlag == 1 {
		file.WriteString(fmt.Sprintf("Original_program_clock_reference_base:%d\n", adaptation.OriginalProgramClockReferenceBase))
		file.WriteString(fmt.Sprintf("Original_program_clock_reference_extension:%d\n", adaptation.OriginalProgramClockReferenceExtension))
	}
	if adaptation.SplicingPointFlag == 1 {
		file.WriteString(fmt.Sprintf("Splice_countdown:%d\n", adaptation.SpliceCountdown))
	}
	if adaptation.TransportPrivateDataFlag == 1 {
		file.WriteString(fmt.Sprintf("Transport_private_data_length:%d\n", adaptation.TransportPrivateDataLength))
	}
	if adaptation.AdaptationFieldExtensionFlag == 1 {
		file.WriteString(fmt.Sprintf("Adaptation_field_extension_length:%d\n", adaptation.AdaptationFieldExtensionLength))
		file.WriteString(fmt.Sprintf("Ltw_flag:%d\n", adaptation.LtwFlag))
		file.WriteString(fmt.Sprintf("Piecewise_rate_flag:%d\n", adaptation.PiecewiseRateFlag))
		file.WriteString(fmt.Sprintf("Seamless_splice_flag:%d\n", adaptation.SeamlessSpliceFlag))
		if adaptation.LtwFlag == 1 {
			file.WriteString(fmt.Sprintf("Ltw_valid_flag:%d\n", adaptation.LtwValidFlag))
			file.WriteString(fmt.Sprintf("Ltw_offset:%d\n", adaptation.LtwOffset))
		}
		if adaptation.PiecewiseRateFlag == 1 {
			file.WriteString(fmt.Sprintf("Piecewise_rate:%d\n", adaptation.PiecewiseRate))
		}
		if adaptation.SeamlessSpliceFlag == 1 {
			file.WriteString(fmt.Sprintf("Splice_type:%d\n", adaptation.SpliceType))
			file.WriteString(fmt.Sprintf("DTS_next_AU:%d\n", adaptation.DtsNextAu))
		}
	}
}

func (adaptation *AdaptationField) Encode(bsw *codec.BitStreamWriter) {
	loc := bsw.ByteOffset()
	bsw.PutUint8(adaptation.AdaptationFieldLength, 8)
	if adaptation.SingleStuffingByte {
		return
	}
	bsw.Markdot()
	bsw.PutUint8(adaptation.DiscontinuityIndicator, 1)
	bsw.PutUint8(adaptation.RandomAccessIndicator, 1)
	bsw.PutUint8(adaptation.ElementaryStreamPriorityIndicator, 1)
	bsw.PutUint8(adaptation.PcrFlag, 1)
	bsw.PutUint8(adaptation.OpcrFlag, 1)
	bsw.PutUint8(adaptation.SplicingPointFlag, 1)
	bsw.PutUint8(0 /*adaptation.Transport_private_data_flag*/, 1)
	bsw.PutUint8(0 /*adaptation.Adaptation_field_extension_flag*/, 1)
	if adaptation.PcrFlag == 1 {
		bsw.PutUint64(adaptation.ProgramClockReferenceBase, 33)
		bsw.PutUint8(0, 6)
		bsw.PutUint16(adaptation.ProgramClockReferenceExtension, 9)
	}
	if adaptation.OpcrFlag == 1 {
		bsw.PutUint64(adaptation.OriginalProgramClockReferenceBase, 33)
		bsw.PutUint8(0, 6)
		bsw.PutUint16(adaptation.OriginalProgramClockReferenceExtension, 9)
	}
	if adaptation.SplicingPointFlag == 1 {
		bsw.PutUint8(adaptation.SpliceCountdown, 8)
	}
	//TODO
	// if adaptation.Transport_private_data_flag == 0 {
	// }
	// if adaptation.Adaptation_field_extension_flag == 0 {
	// }
	adaptation.AdaptationFieldLength = uint8(bsw.DistanceFromMarkDot() / 8)
	bsw.PutRepetValue(0xff, int(adaptation.StuffingByte))
	adaptation.AdaptationFieldLength += adaptation.StuffingByte
	bsw.SetByte(adaptation.AdaptationFieldLength, loc)
}

func (adaptation *AdaptationField) Decode(bs *codec.BitStream) error {
	if bs.RemainBytes() < 1 {
		return errors.New("len of data < 1 byte")
	}
	adaptation.AdaptationFieldLength = bs.Uint8(8)
	startoffset := bs.ByteOffset()
	//fmt.Printf("Adaptation_field_length=%d\n", adaptation.Adaptation_field_length)
	if bs.RemainBytes() < int(adaptation.AdaptationFieldLength) {
		return errors.New("len of data < Adaptation_field_length")
	}
	if adaptation.AdaptationFieldLength == 0 {
		return nil
	}
	adaptation.DiscontinuityIndicator = bs.GetBit()
	adaptation.RandomAccessIndicator = bs.GetBit()
	adaptation.ElementaryStreamPriorityIndicator = bs.GetBit()
	adaptation.PcrFlag = bs.GetBit()
	adaptation.OpcrFlag = bs.GetBit()
	adaptation.SplicingPointFlag = bs.GetBit()
	adaptation.TransportPrivateDataFlag = bs.GetBit()
	adaptation.AdaptationFieldExtensionFlag = bs.GetBit()
	if adaptation.PcrFlag == 1 {
		adaptation.ProgramClockReferenceBase = bs.GetBits(33)
		bs.SkipBits(6)
		adaptation.ProgramClockReferenceExtension = uint16(bs.GetBits(9))
	}
	if adaptation.OpcrFlag == 1 {
		adaptation.OriginalProgramClockReferenceBase = bs.GetBits(33)
		bs.SkipBits(6)
		adaptation.OriginalProgramClockReferenceExtension = uint16(bs.GetBits(9))
	}
	if adaptation.SplicingPointFlag == 1 {
		adaptation.SpliceCountdown = bs.Uint8(8)
	}
	if adaptation.TransportPrivateDataFlag == 1 {
		adaptation.TransportPrivateDataLength = bs.Uint8(8)
		bs.SkipBits(8 * int(adaptation.TransportPrivateDataLength))
	}
	if adaptation.AdaptationFieldExtensionFlag == 1 {
		adaptation.AdaptationFieldExtensionLength = bs.Uint8(8)
		bs.Markdot()
		adaptation.LtwFlag = bs.GetBit()
		adaptation.PiecewiseRateFlag = bs.GetBit()
		adaptation.SeamlessSpliceFlag = bs.GetBit()
		bs.SkipBits(5)
		if adaptation.LtwFlag == 1 {
			adaptation.LtwValidFlag = bs.GetBit()
			adaptation.LtwOffset = uint16(bs.GetBits(15))
		}
		if adaptation.PiecewiseRateFlag == 1 {
			bs.SkipBits(2)
			adaptation.PiecewiseRate = uint32(bs.GetBits(22))
		}
		if adaptation.SeamlessSpliceFlag == 1 {
			adaptation.SpliceType = uint8(bs.GetBits(4))
			adaptation.DtsNextAu = bs.GetBits(3)
			bs.SkipBits(1)
			adaptation.DtsNextAu = adaptation.DtsNextAu<<15 | bs.GetBits(15)
			bs.SkipBits(1)
			adaptation.DtsNextAu = adaptation.DtsNextAu<<15 | bs.GetBits(15)
			bs.SkipBits(1)
		}
		bitscount := bs.DistanceFromMarkDot()
		if bitscount%8 > 0 {
			panic("maybe parser ts file failed")
		}
		bs.SkipBits(int(adaptation.AdaptationFieldExtensionLength*8 - uint8(bitscount)))
	}
	endoffset := bs.ByteOffset()
	bs.SkipBits((int(adaptation.AdaptationFieldLength) - (endoffset - startoffset)) * 8)
	return nil
}

type PmtPair struct {
	ProgramNumber uint16
	PID           uint16
}

func ReadSection(sectionType PatTid, bs *codec.BitStream) (interface{}, error) {
	//  VLC libdvbpsi.c dvbpsi_packet_push
	//  /* A TS packet may contain any number of sections, only the first
	//   * new one is flagged by the pointer_field. If the next payload
	//   * byte isn't 0xff then a new section starts. */
	//   if (p_new_pos == NULL && i_available && *p_payload_pos != 0xff)
	//       p_new_pos = p_payload_pos;
	for bs.NextBits(8) == 0xff {
		bs.SkipBits(8)
		if bs.RemainBytes() <= 0 {
			return nil, errors.New("illegal section")
		}
		continue
	}

	switch sectionType {
	case TsTidPas:
		pat := NewPat()
		if err := pat.Decode(bs); err != nil {
			return nil, err
		}
		return pat, nil
	case TsTidPms:
		pmt := NewPmt()
		if err := pmt.Decode(bs); err != nil {
			return nil, err
		}
		return pmt, nil
	}
	return nil, nil
}

type Pat struct {
	TableId                uint8  //8  uimsbf
	SectionSyntaxIndicator uint8  //1  bslbf
	SectionLength          uint16 //12 uimsbf
	TransportStreamId      uint16 //16 uimsbf
	VersionNumber          uint8  //5  uimsbf
	CurrentNextIndicator   uint8  //1  bslbf
	SectionNumber          uint8  //8  uimsbf
	LastSectionNumber      uint8  //8  uimsbf
	Pmts                   []PmtPair
}

func NewPat() *Pat {
	return &Pat{
		TableId:                uint8(TsTidPas),
		SectionSyntaxIndicator: 1,
		CurrentNextIndicator:   1,
		Pmts:                   make([]PmtPair, 0, 8),
	}
}

func (pat *Pat) PrettyPrint(file *os.File) {
	file.WriteString(fmt.Sprintf("Table id:%d\n", pat.TableId))
	file.WriteString(fmt.Sprintf("Section_syntax_indicator:%d\n", pat.SectionSyntaxIndicator))
	file.WriteString(fmt.Sprintf("Section_length:%d\n", pat.SectionLength))
	file.WriteString(fmt.Sprintf("Transport_stream_id:%d\n", pat.TransportStreamId))
	file.WriteString(fmt.Sprintf("Version_number:%d\n", pat.VersionNumber))
	file.WriteString(fmt.Sprintf("Current_next_indicator:%d\n", pat.CurrentNextIndicator))
	file.WriteString(fmt.Sprintf("Section_number:%d\n", pat.SectionNumber))
	file.WriteString(fmt.Sprintf("Last_section_number:%d\n", pat.LastSectionNumber))
	for i, pmt := range pat.Pmts {
		file.WriteString(fmt.Sprintf("----pmt %d\n", i))
		file.WriteString(fmt.Sprintf("    program_number:%d\n", pmt.ProgramNumber))
		if pmt.ProgramNumber == 0x0000 {
			file.WriteString(fmt.Sprintf("    network_PID:%d\n", pmt.PID))
		} else {
			file.WriteString(fmt.Sprintf("    program_map_PID:%d\n", pmt.PID))
		}
	}
}

func (pat *Pat) Encode(bsw *codec.BitStreamWriter) {
	bsw.PutUint8(0x00, 8)
	loc := bsw.ByteOffset()
	bsw.PutUint8(pat.SectionSyntaxIndicator, 1)
	bsw.PutUint8(0x00, 1)
	bsw.PutUint8(0x03, 2)
	bsw.PutUint16(0, 12)
	bsw.Markdot()
	bsw.PutUint16(pat.TransportStreamId, 16)
	bsw.PutUint8(0x03, 2)
	bsw.PutUint8(pat.VersionNumber, 5)
	bsw.PutUint8(pat.CurrentNextIndicator, 1)
	bsw.PutUint8(pat.SectionNumber, 8)
	bsw.PutUint8(pat.LastSectionNumber, 8)
	for _, pms := range pat.Pmts {
		bsw.PutUint16(pms.ProgramNumber, 16)
		bsw.PutUint8(0x07, 3)
		bsw.PutUint16(pms.PID, 13)
	}
	length := bsw.DistanceFromMarkDot()
	//|Section_syntax_indicator|'0'|reserved|Section_length|
	pat.SectionLength = uint16(length)/8 + 4
	bsw.SetUint16(pat.SectionLength&0x0FFF|(uint16(pat.SectionSyntaxIndicator)<<15)|0x3000, loc)
	crc := codec.CalcCrc32(0xffffffff, bsw.Bits()[bsw.ByteOffset()-int(pat.SectionLength-4)-3:bsw.ByteOffset()])
	tmpcrc := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmpcrc, crc)
	bsw.PutBytes(tmpcrc)
}

func (pat *Pat) Decode(bs *codec.BitStream) error {
	pat.TableId = bs.Uint8(8)
	if pat.TableId != uint8(TsTidPas) {
		return errors.New("table id is Not TS_TID_PAS")
	}
	pat.SectionSyntaxIndicator = bs.Uint8(1)
	bs.SkipBits(3)
	pat.SectionLength = bs.Uint16(12)
	pat.TransportStreamId = bs.Uint16(16)
	bs.SkipBits(2)
	pat.VersionNumber = bs.Uint8(5)
	pat.CurrentNextIndicator = bs.Uint8(1)
	pat.SectionNumber = bs.Uint8(8)
	pat.LastSectionNumber = bs.Uint8(8)
	for i := 0; i+4 <= int(pat.SectionLength)-5-4; i = i + 4 {
		tmp := PmtPair{
			ProgramNumber: 0,
			PID:           0,
		}
		tmp.ProgramNumber = bs.Uint16(16)
		bs.SkipBits(3)
		tmp.PID = bs.Uint16(13)
		pat.Pmts = append(pat.Pmts, tmp)
	}
	return nil
}

type StreamPair struct {
	StreamType    uint8  //8 uimsbf
	ElementaryPid uint16 //13 uimsbf
	EsInfoLength  uint16 //12 uimsbf
}

type Pmt struct {
	TableId                uint8  //8  uimsbf
	SectionSyntaxIndicator uint8  //1  bslbf
	SectionLength          uint16 //12 uimsbf
	ProgramNumber          uint16 //16 uimsbf
	VersionNumber          uint8  //5  uimsbf
	CurrentNextIndicator   uint8  //1  bslbf
	SectionNumber          uint8  //8  uimsbf
	LastSectionNumber      uint8  //8  uimsbf
	PcrPid                 uint16 //13 uimsbf
	ProgramInfoLength      uint16 //12 uimsbf
	Streams                []StreamPair
}

func NewPmt() *Pmt {
	return &Pmt{
		TableId:                uint8(TsTidPms),
		SectionSyntaxIndicator: 1,
		CurrentNextIndicator:   1,
		Streams:                make([]StreamPair, 0, 8),
	}
}

func (pmt *Pmt) PrettyPrint(file *os.File) {
	file.WriteString(fmt.Sprintf("Table id:%d\n", pmt.TableId))
	file.WriteString(fmt.Sprintf("Section_syntax_indicator:%d\n", pmt.SectionSyntaxIndicator))
	file.WriteString(fmt.Sprintf("Section_length:%d\n", pmt.SectionLength))
	file.WriteString(fmt.Sprintf("Program_number:%d\n", pmt.ProgramNumber))
	file.WriteString(fmt.Sprintf("Version_number:%d\n", pmt.VersionNumber))
	file.WriteString(fmt.Sprintf("Current_next_indicator:%d\n", pmt.CurrentNextIndicator))
	file.WriteString(fmt.Sprintf("Section_number:%d\n", pmt.SectionNumber))
	file.WriteString(fmt.Sprintf("Last_section_number:%d\n", pmt.LastSectionNumber))
	file.WriteString(fmt.Sprintf("PCR_PID:%d\n", pmt.PcrPid))
	file.WriteString(fmt.Sprintf("program_info_length:%d\n", pmt.ProgramInfoLength))
	for i, stream := range pmt.Streams {
		file.WriteString(fmt.Sprintf("----stream %d\n", i))
		if stream.StreamType == uint8(TsStreamAac) {
			file.WriteString("    stream_type:AAC\n")
		} else if stream.StreamType == uint8(TsStreamAudioMpeg1) {
			file.WriteString("    stream_type:MPEG1\n")
		} else if stream.StreamType == uint8(TsStreamAudioMpeg2) {
			file.WriteString("    stream_type:MPEG2,mp3\n")
		} else if stream.StreamType == uint8(TsStreamH264) {
			file.WriteString("    stream_type:H264\n")
		} else if stream.StreamType == uint8(TsStreamH265) {
			file.WriteString("    stream_type:H265\n")
		} else {
			file.WriteString(fmt.Sprintf("    stream_type:UnSupport streamtype:%d\n", stream.StreamType))
		}
		file.WriteString(fmt.Sprintf("    elementary_PID:%d\n", stream.ElementaryPid))
		file.WriteString(fmt.Sprintf("    ES_info_length:%d\n", stream.EsInfoLength))
	}
}

func (pmt *Pmt) Encode(bsw *codec.BitStreamWriter) {
	bsw.PutUint8(pmt.TableId, 8)
	loc := bsw.ByteOffset()
	bsw.PutUint8(pmt.SectionSyntaxIndicator, 1)
	bsw.PutUint8(0x00, 1)
	bsw.PutUint8(0x03, 2)
	bsw.PutUint16(pmt.SectionLength, 12)
	bsw.Markdot()
	bsw.PutUint16(pmt.ProgramNumber, 16)
	bsw.PutUint8(0x03, 2)
	bsw.PutUint8(pmt.VersionNumber, 5)
	bsw.PutUint8(pmt.CurrentNextIndicator, 1)
	bsw.PutUint8(pmt.SectionNumber, 8)
	bsw.PutUint8(pmt.LastSectionNumber, 8)
	bsw.PutUint8(0x07, 3)
	bsw.PutUint16(pmt.PcrPid, 13)
	bsw.PutUint8(0x0f, 4)
	//TODO Program info length
	bsw.PutUint16(0x0000 /*pmt.Program_info_length*/, 12)
	for _, stream := range pmt.Streams {
		bsw.PutUint8(stream.StreamType, 8)
		bsw.PutUint8(0x00, 3)
		bsw.PutUint16(stream.ElementaryPid, 13)
		bsw.PutUint8(0x00, 4)
		//TODO ES_info
		bsw.PutUint8(0 /*ES_info_length*/, 12)
	}
	length := bsw.DistanceFromMarkDot()
	pmt.SectionLength = uint16(length)/8 + 4
	bsw.SetUint16(pmt.SectionLength&0x0FFF|(uint16(pmt.SectionSyntaxIndicator)<<15)|0x3000, loc)
	crc := codec.CalcCrc32(0xffffffff, bsw.Bits()[bsw.ByteOffset()-int(pmt.SectionLength-4)-3:bsw.ByteOffset()])
	tmpcrc := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmpcrc, crc)
	bsw.PutBytes(tmpcrc)
}

func (pmt *Pmt) Decode(bs *codec.BitStream) error {
	pmt.TableId = bs.Uint8(8)
	if pmt.TableId != uint8(TsTidPms) {
		return errors.New("table id is Not TS_TID_PAS")
	}
	pmt.SectionSyntaxIndicator = bs.Uint8(1)
	bs.SkipBits(3)
	pmt.SectionLength = bs.Uint16(12)
	pmt.ProgramNumber = bs.Uint16(16)
	bs.SkipBits(2)
	pmt.VersionNumber = bs.Uint8(5)
	pmt.CurrentNextIndicator = bs.Uint8(1)
	pmt.SectionNumber = bs.Uint8(8)
	pmt.LastSectionNumber = bs.Uint8(8)
	bs.SkipBits(3)
	pmt.PcrPid = bs.Uint16(13)
	bs.SkipBits(4)
	pmt.ProgramInfoLength = bs.Uint16(12)
	//TODO N loop descriptors
	bs.SkipBits(int(pmt.ProgramInfoLength) * 8)
	//fmt.Printf("section length %d pmt.Pogram_info_length=%d\n", pmt.Section_length, pmt.Pogram_info_length)
	for i := 0; i < int(pmt.SectionLength)-9-int(pmt.ProgramInfoLength)-4; {
		tmp := StreamPair{
			StreamType:    0,
			ElementaryPid: 0,
			EsInfoLength:  0,
		}
		tmp.StreamType = bs.Uint8(8)
		bs.SkipBits(3)
		tmp.ElementaryPid = bs.Uint16(13)
		bs.SkipBits(4)
		tmp.EsInfoLength = bs.Uint16(12)
		//TODO N loop descriptors
		bs.SkipBits(int(tmp.EsInfoLength) * 8)
		pmt.Streams = append(pmt.Streams, tmp)
		i += 5 + int(tmp.EsInfoLength)
	}
	return nil
}
