package codec

import "errors"

// Table 31 – Profiles
// index      profile
//   0        Main profile
//   1        Low Complexity profile (LC)
//   2        Scalable Sampling Rate profile (SSR)
//   3        (reserved)

type AacProfile int

const (
	MAIN AacProfile = iota
	LC
	SSR
)

type AacSamplingFrequency int

const (
	AacSample96000 AacSamplingFrequency = iota
	AacSample88200
	AacSample64000
	AacSample48000
	AacSample44100
	AacSample32000
	AacSample24000
	AacSample22050
	AacSample16000
	AacSample12000
	AacSample11025
	AacSample8000
	AacSample7350
)

var AacSamplingIdx = [13]int{96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350}

// Table 4 – Syntax of adts_sequence()
// adts_sequence() {
//         while (nextbits() == syncword) {
//             adts_frame();
//         }
// }
// Table 5 – Syntax of adts_frame()
// adts_frame() {
//     adts_fixed_header();
//     adts_variable_header();
//     if (number_of_raw_data_blocks_in_frame == 0) {
//         adts_error_check();
//         raw_data_block();
//     }
//     else {
//         adts_header_error_check();
//         for (i = 0; i <= number_of_raw_data_blocks_in_frame;i++ {
//             raw_data_block();
//             adts_raw_data_block_error_check();
//         }
//     }
// }

// adts_fixed_header()
// {
//         syncword;                         12           bslbf
//         ID;                                1            bslbf
//         layer;                          2            uimsbf
//         protection_absent;              1            bslbf
//         profile;                        2            uimsbf
//         sampling_frequency_index;       4            uimsbf
//         private_bit;                    1            bslbf
//         channel_configuration;          3            uimsbf
//         original/copy;                  1            bslbf
//         home;                           1            bslbf
// }

type AdtsFixHeader struct {
	ID                     uint8
	Layer                  uint8
	ProtectionAbsent       uint8
	Profile                uint8
	SamplingFrequencyIndex uint8
	PrivateBit             uint8
	ChannelConfiguration   uint8
	Originalorcopy         uint8
	Home                   uint8
}

// adts_variable_header() {
//      copyright_identification_bit;               1      bslbf
//      copyright_identification_start;             1      bslbf
//      frame_length;                               13     bslbf
//      adts_buffer_fullness;                       11     bslbf
//      number_of_raw_data_blocks_in_frame;         2      uimsfb
// }

type AdtsVariableHeader struct {
	CopyrightIdentificationBit   uint8
	copyrightIdentificationStart uint8
	FrameLength                  uint16
	AdtsBufferFullness           uint16
	NumberOfRawDataBlocksInFrame uint8
}

type AdtsFrameHeader struct {
	FixHeader      AdtsFixHeader
	VariableHeader AdtsVariableHeader
}

func NewAdtsFrameHeader() *AdtsFrameHeader {
	return &AdtsFrameHeader{
		FixHeader: AdtsFixHeader{
			ID:                     0,
			Layer:                  0,
			ProtectionAbsent:       1,
			Profile:                uint8(MAIN),
			SamplingFrequencyIndex: uint8(AacSample44100),
			PrivateBit:             0,
			ChannelConfiguration:   0,
			Originalorcopy:         0,
			Home:                   0,
		},

		VariableHeader: AdtsVariableHeader{
			copyrightIdentificationStart: 0,
			CopyrightIdentificationBit:   0,
			FrameLength:                  0,
			AdtsBufferFullness:           0,
			NumberOfRawDataBlocksInFrame: 0,
		},
	}
}

func (frame *AdtsFrameHeader) Decode(aac []byte) {
	_ = aac[6]
	frame.FixHeader.ID = aac[1] >> 3
	frame.FixHeader.Layer = aac[1] >> 1 & 0x03
	frame.FixHeader.ProtectionAbsent = aac[1] & 0x01
	frame.FixHeader.Profile = aac[2] >> 6 & 0x03
	frame.FixHeader.SamplingFrequencyIndex = aac[2] >> 2 & 0x0F
	frame.FixHeader.PrivateBit = aac[2] >> 1 & 0x01
	frame.FixHeader.ChannelConfiguration = (aac[2] & 0x01 << 2) | (aac[3] >> 6)
	frame.FixHeader.Originalorcopy = aac[3] >> 5 & 0x01
	frame.FixHeader.Home = aac[3] >> 4 & 0x01
	frame.VariableHeader.CopyrightIdentificationBit = aac[3] >> 3 & 0x01
	frame.VariableHeader.copyrightIdentificationStart = aac[3] >> 2 & 0x01
	frame.VariableHeader.FrameLength = (uint16(aac[3]&0x03) << 11) | (uint16(aac[4]) << 3) | (uint16(aac[5]>>5) & 0x07)
	frame.VariableHeader.AdtsBufferFullness = (uint16(aac[5]&0x1F) << 6) | uint16(aac[6]>>2)
	frame.VariableHeader.NumberOfRawDataBlocksInFrame = aac[6] & 0x03
}

func (frame *AdtsFrameHeader) Encode() []byte {
	var hdr []byte
	if frame.FixHeader.ProtectionAbsent == 1 {
		hdr = make([]byte, 7)
	} else {
		hdr = make([]byte, 9)
	}
	hdr[0] = 0xFF
	hdr[1] = 0xF0
	hdr[1] = hdr[1] | (frame.FixHeader.ID << 3) | (frame.FixHeader.Layer << 1) | frame.FixHeader.ProtectionAbsent
	hdr[2] = frame.FixHeader.Profile<<6 | frame.FixHeader.SamplingFrequencyIndex<<2 | frame.FixHeader.PrivateBit<<1 | frame.FixHeader.ChannelConfiguration>>2
	hdr[3] = frame.FixHeader.ChannelConfiguration<<6 | frame.FixHeader.Originalorcopy<<5 | frame.FixHeader.Home<<4
	hdr[3] = hdr[3] | frame.VariableHeader.copyrightIdentificationStart<<3 | frame.VariableHeader.CopyrightIdentificationBit<<2 | byte(frame.VariableHeader.FrameLength<<11)
	hdr[4] = byte(frame.VariableHeader.FrameLength >> 3)
	hdr[5] = byte((frame.VariableHeader.FrameLength&0x07)<<5) | byte(frame.VariableHeader.AdtsBufferFullness>>3)
	hdr[6] = byte(frame.VariableHeader.AdtsBufferFullness&0x3F<<2) | frame.VariableHeader.NumberOfRawDataBlocksInFrame
	return hdr
}

func SampleToAACSampleIndex(sampling int) int {
	for i, v := range AacSamplingIdx {
		if v == sampling {
			return i
		}
	}
	panic("not Found AAC Sample Index")
}

func AACSampleIdxToSample(idx int) int {
	return AacSamplingIdx[idx]
}

// +--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
// |  audio object type(5 bits)  |  sampling frequency index(4 bits) |   channel configuration(4 bits)  | GA framelength flag(1 bits) |  GA Depends on core coder(1 bits) | GA Extension Flag(1 bits) |
// +--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+

type AudioSpecificConfiguration struct {
	AudioObjectType      uint8
	SampleFreqIndex      uint8
	ChannelConfiguration uint8
	GaFramelengthFlag    uint8
	GaDependsOnCoreCoder uint8
	GaExtensionFlag      uint8
}

func NewAudioSpecificConfiguration() *AudioSpecificConfiguration {
	return &AudioSpecificConfiguration{
		AudioObjectType:      0,
		SampleFreqIndex:      0,
		ChannelConfiguration: 0,
		GaFramelengthFlag:    0,
		GaDependsOnCoreCoder: 0,
		GaExtensionFlag:      0,
	}
}

func (asc *AudioSpecificConfiguration) Encode() []byte {
	buf := make([]byte, 2)
	buf[0] = (asc.AudioObjectType & 0x1f << 3) | (asc.SampleFreqIndex & 0x0F >> 1)
	buf[1] = (asc.SampleFreqIndex & 0x0F << 7) | (asc.ChannelConfiguration & 0x0F << 3) | (asc.GaFramelengthFlag & 0x01 << 2) | (asc.GaDependsOnCoreCoder & 0x01 << 1) | (asc.GaExtensionFlag & 0x01)
	return buf
}

func (asc *AudioSpecificConfiguration) Decode(buf []byte) error {

	if len(buf) < 2 {
		return errors.New("len of buf < 2 ")
	}

	asc.AudioObjectType = buf[0] >> 3
	asc.SampleFreqIndex = (buf[0] & 0x07 << 1) | (buf[1] >> 7)
	asc.ChannelConfiguration = buf[1] >> 3 & 0x0F
	asc.GaFramelengthFlag = buf[1] >> 2 & 0x01
	asc.GaDependsOnCoreCoder = buf[1] >> 1 & 0x01
	asc.GaExtensionFlag = buf[1] & 0x01
	return nil
}

func ConvertADTSToASC(frame []byte) (*AudioSpecificConfiguration, error) {
	if len(frame) < 7 {
		return nil, errors.New("len of frame < 7")
	}
	adts := NewAdtsFrameHeader()
	adts.Decode(frame)
	asc := NewAudioSpecificConfiguration()
	asc.AudioObjectType = adts.FixHeader.Profile + 1
	asc.ChannelConfiguration = adts.FixHeader.ChannelConfiguration
	asc.SampleFreqIndex = adts.FixHeader.SamplingFrequencyIndex
	return asc, nil
}

func ConvertASCToADTS(asc []byte, aacbytes int) (*AdtsFrameHeader, error) {
	aacAsc := NewAudioSpecificConfiguration()
	err := aacAsc.Decode(asc)
	if err != nil {
		return nil, err
	}
	aacAdts := NewAdtsFrameHeader()
	aacAdts.FixHeader.Profile = aacAsc.AudioObjectType - 1
	aacAdts.FixHeader.ChannelConfiguration = aacAsc.ChannelConfiguration
	aacAdts.FixHeader.SamplingFrequencyIndex = aacAsc.SampleFreqIndex
	aacAdts.FixHeader.ProtectionAbsent = 1
	aacAdts.VariableHeader.AdtsBufferFullness = 0x3F
	aacAdts.VariableHeader.FrameLength = uint16(aacbytes)
	return aacAdts, nil
}
