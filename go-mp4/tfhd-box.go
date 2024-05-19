package mp4

import (
	"encoding/binary"
	"io"
)

// aligned(8) class TrackFragmentHeaderBox extends FullBox(‘tfhd’, 0, tf_flags){
//     unsigned int(32) track_ID;
//     // all the following are optional fields
//     unsigned int(64) base_data_offset;
//     unsigned int(32) sample_description_index;
//     unsigned int(32) default_sample_duration;
//     unsigned int(32) default_sample_size;
//     unsigned int(32) default_sample_flags
// }

const (
	TfFlagBaseDataOffset                uint32 = 0x000001
	TfFlagSampleDescriptionIndexPresent uint32 = 0x000002
	TfFlagDefaultSampleDurationPresent  uint32 = 0x000008
	TfFlagDefaultSampleSizePresent      uint32 = 0x000010
	TfFlagDeaaultSampleFlagsPresent     uint32 = 0x000020
	TfFlagDurationIsEmpty               uint32 = 0x010000
	TfFlagDeaaultBaseIsMoof             uint32 = 0x020000

	//ffmpeg isom.h
	MovFragSampleFlagDegradationPriorityMask uint32 = 0x0000ffff
	MovFragSampleFlagIsNonSync               uint32 = 0x00010000
	MovFragSampleFlagPaddingMask             uint32 = 0x000e0000
	MovFragSampleFlagRedundancyMask          uint32 = 0x00300000
	MovFragSampleFlagDependedMask            uint32 = 0x00c00000
	MovFragSampleFlagDependsMask             uint32 = 0x03000000

	MovFragSampleFlagDependsNo  uint32 = 0x02000000
	MovFragSampleFlagDependsYes uint32 = 0x01000000
)

type TrackFragmentHeaderBox struct {
	Box                    *FullBox
	TrackId                uint32
	BaseDataOffset         uint64
	SampleDescriptionIndex uint32
	DefaultSampleDuration  uint32
	DefaultSampleSize      uint32
	DefaultSampleFlags     uint32
}

func NewTrackFragmentHeaderBox(trackid uint32) *TrackFragmentHeaderBox {
	return &TrackFragmentHeaderBox{
		Box:                    NewFullBox([4]byte{'t', 'f', 'h', 'd'}, 0),
		TrackId:                trackid,
		SampleDescriptionIndex: 1,
	}
}

func (tfhd *TrackFragmentHeaderBox) Size() uint64 {
	n := tfhd.Box.Size()
	thfdFlags := uint32(tfhd.Box.Flags[0])<<16 | uint32(tfhd.Box.Flags[1])<<8 | uint32(tfhd.Box.Flags[2])
	n += 4
	if thfdFlags&TfFlagBaseDataOffset > 0 {
		n += 8
	}
	if thfdFlags&TfFlagSampleDescriptionIndexPresent > 0 {
		n += 4
	}
	if thfdFlags&TfFlagDefaultSampleDurationPresent > 0 {
		n += 4
	}
	if thfdFlags&TfFlagDefaultSampleSizePresent > 0 {
		n += 4
	}
	if thfdFlags&TfFlagDeaaultSampleFlagsPresent > 0 {
		n += 4
	}
	return n
}

func (tfhd *TrackFragmentHeaderBox) Decode(r io.Reader, size uint32, moofOffset uint64) (offset int, err error) {
	if offset, err = tfhd.Box.Decode(r); err != nil {
		return
	}
	buf := make([]byte, size-12)
	if _, err = io.ReadFull(r, buf); err != nil {
		return 0, err
	}
	n := 0
	tfhd.TrackId = binary.BigEndian.Uint32(buf[n:])
	n += 4
	tfhdFlags := uint32(tfhd.Box.Flags[0])<<16 | uint32(tfhd.Box.Flags[1])<<8 | uint32(tfhd.Box.Flags[2])
	if tfhdFlags&uint32(TfFlagBaseDataOffset) > 0 {
		tfhd.BaseDataOffset = binary.BigEndian.Uint64(buf[n:])
		n += 8
	} else if tfhdFlags&uint32(TfFlagDeaaultBaseIsMoof) > 0 {
		tfhd.BaseDataOffset = moofOffset
	} else {
		//TODO,In some cases, it is wrong
		tfhd.BaseDataOffset = moofOffset
	}

	if tfhdFlags&uint32(TfFlagSampleDescriptionIndexPresent) > 0 {
		tfhd.SampleDescriptionIndex = binary.BigEndian.Uint32(buf[n:])
		n += 4
	}
	if tfhdFlags&uint32(TfFlagDefaultSampleDurationPresent) > 0 {
		tfhd.DefaultSampleDuration = binary.BigEndian.Uint32(buf[n:])
		n += 4
	}
	if tfhdFlags&uint32(TfFlagDefaultSampleSizePresent) > 0 {
		tfhd.DefaultSampleSize = binary.BigEndian.Uint32(buf[n:])
		n += 4
	}
	if tfhdFlags&uint32(TfFlagDeaaultSampleFlagsPresent) > 0 {
		tfhd.DefaultSampleFlags = binary.BigEndian.Uint32(buf[n:])
		n += 4
	}
	offset += n
	return
}

func (tfhd *TrackFragmentHeaderBox) Encode() (int, []byte) {
	tfhd.Box.Box.Size = tfhd.Size()
	offset, buf := tfhd.Box.Encode()
	binary.BigEndian.PutUint32(buf[offset:], tfhd.TrackId)
	offset += 4
	thfdFlags := uint32(tfhd.Box.Flags[0])<<16 | uint32(tfhd.Box.Flags[1])<<8 | uint32(tfhd.Box.Flags[2])
	if thfdFlags&uint32(TfFlagBaseDataOffset) > 0 {
		binary.BigEndian.PutUint64(buf[offset:], tfhd.BaseDataOffset)
		offset += 8
	}
	if thfdFlags&uint32(TfFlagSampleDescriptionIndexPresent) > 0 {
		binary.BigEndian.PutUint32(buf[offset:], tfhd.SampleDescriptionIndex)
		offset += 4
	}
	if thfdFlags&uint32(TfFlagDefaultSampleDurationPresent) > 0 {
		binary.BigEndian.PutUint32(buf[offset:], tfhd.DefaultSampleDuration)
		offset += 4
	}
	if thfdFlags&uint32(TfFlagDefaultSampleSizePresent) > 0 {
		binary.BigEndian.PutUint32(buf[offset:], tfhd.DefaultSampleSize)
		offset += 4
	}
	if thfdFlags&uint32(TfFlagDeaaultSampleFlagsPresent) > 0 {
		binary.BigEndian.PutUint32(buf[offset:], tfhd.DefaultSampleFlags)
		offset += 4
	}
	return offset, buf
}

func decodeTfhdBox(demuxer *MovDemuxer, size uint32) error {
	tfhd := &TrackFragmentHeaderBox{Box: new(FullBox)}
	_, err := tfhd.Decode(demuxer.reader, size, uint64(demuxer.moofOffset))
	for i := 0; i < len(demuxer.tracks); i++ {
		if demuxer.tracks[i].trackId != tfhd.TrackId {
			continue
		}
		demuxer.currentTrack = demuxer.tracks[i]
		demuxer.tracks[i].defaultDuration = tfhd.DefaultSampleDuration
		demuxer.tracks[i].defaultSize = tfhd.DefaultSampleSize
		demuxer.tracks[i].baseDataOffset = tfhd.BaseDataOffset
	}
	return err
}

func makeTfhdBox(track *mp4track, offset uint64) []byte {
	tfFlags := TfFlagSampleDescriptionIndexPresent
	tfFlags |= TfFlagDeaaultBaseIsMoof
	tfhd := NewTrackFragmentHeaderBox(track.trackId)
	tfhd.BaseDataOffset = offset
	if len(track.samplelist) > 1 {
		tfhd.DefaultSampleDuration = uint32(track.samplelist[1].dts - track.samplelist[0].dts)
	} else if len(track.samplelist) == 1 && len(track.fragments) > 0 {
		tfhd.DefaultSampleDuration = uint32(track.samplelist[0].dts - track.fragments[len(track.fragments)-1].lastDts)
	} else {
		tfhd.DefaultSampleDuration = 0
		tfFlags |= TfFlagDurationIsEmpty
	}
	if len(track.samplelist) > 0 {
		tfFlags |= TfFlagDeaaultSampleFlagsPresent
		tfFlags |= TfFlagDefaultSampleDurationPresent
		tfFlags |= TfFlagDefaultSampleSizePresent
		tfhd.DefaultSampleSize = uint32(track.samplelist[0].size)
	} else {
		tfhd.DefaultSampleSize = 0
	}
	tfhd.Box.Flags[0] = uint8(tfFlags >> 16)
	tfhd.Box.Flags[1] = uint8(tfFlags >> 8)
	tfhd.Box.Flags[2] = uint8(tfFlags)

	//ffmpeg movenc.c mov_write_tfhd_tag
	if isVideo(track.cid) {
		tfhd.DefaultSampleFlags = MovFragSampleFlagDependsYes | MovFragSampleFlagIsNonSync
	} else {
		tfhd.DefaultSampleFlags = MovFragSampleFlagDependsNo
	}
	track.defaultDuration = tfhd.DefaultSampleDuration
	track.defaultSize = tfhd.DefaultSampleSize
	track.defaultSampleFlags = tfhd.DefaultSampleFlags
	_, boxData := tfhd.Encode()
	return boxData
}
