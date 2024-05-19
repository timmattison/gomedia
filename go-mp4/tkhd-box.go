package mp4

import (
	"encoding/binary"
	"io"
	"time"
)

// aligned(8) class TrackHeaderBox
//    extends FullBox(‘tkhd’, version, flags){
//    if (version==1) {
//       unsigned int(64)  creation_time;
//       unsigned int(64)  modification_time;
//       unsigned int(32)  track_ID;
//       const unsigned int(32)  reserved = 0;
//       unsigned int(64)  duration;
//    } else { // version==0
//       unsigned int(32)  creation_time;
//       unsigned int(32)  modification_time;
//       unsigned int(32)  track_ID;
//       const unsigned int(32)  reserved = 0;
//       unsigned int(32)  duration;
// }
// const unsigned int(32)[2] reserved = 0;
// template int(16) layer = 0;
// template int(16) alternate_group = 0;
// template int(16) volume = {if track_is_audio 0x0100 else 0};
// const unsigned int(16) reserved = 0;
// template int(32)[9] matrix=
// { 0x00010000,0,0,0,0x00010000,0,0,0,0x40000000 };
//       // unity matrix
//    unsigned int(32) width;
//    unsigned int(32) height;
// }

type TrackHeaderBox struct {
	Box              *FullBox
	CreationTime     uint64
	ModificationTime uint64
	TrackId          uint32
	Duration         uint64
	Layer            uint16
	AlternateGroup   uint16
	Volume           uint16
	Matrix           [9]uint32
	Width            uint32
	Height           uint32
}

func NewTrackHeaderBox() *TrackHeaderBox {
	_, offset := time.Now().Zone()
	return &TrackHeaderBox{
		Box:              NewFullBox([4]byte{'t', 'k', 'h', 'd'}, 0),
		CreationTime:     uint64(time.Now().Unix() + int64(offset) + 0x7C25B080),
		ModificationTime: uint64(time.Now().Unix() + int64(offset) + 0x7C25B080),
		Layer:            0,
		AlternateGroup:   0,
		Matrix:           [9]uint32{0x00010000, 0, 0, 0, 0x00010000, 0, 0, 0, 0x40000000},
	}
}

func (tkhd *TrackHeaderBox) Size() uint64 {
	if tkhd.Box.Version == 1 {
		return tkhd.Box.Size() + 92
	} else {
		return tkhd.Box.Size() + 80
	}
}

func (tkhd *TrackHeaderBox) Decode(r io.Reader) (offset int, err error) {
	if offset, err = tkhd.Box.Decode(r); err != nil {
		return 0, err
	}
	boxsize := 0
	if tkhd.Box.Version == 0 {
		boxsize = 80
	} else {
		boxsize = 92
	}
	buf := make([]byte, boxsize)
	if _, err = io.ReadFull(r, buf); err != nil {
		return 0, err
	}
	n := 0
	if tkhd.Box.Version == 1 {
		tkhd.CreationTime = binary.BigEndian.Uint64(buf[n:])
		n += 8
		tkhd.ModificationTime = binary.BigEndian.Uint64(buf[n:])
		n += 8
		tkhd.TrackId = binary.BigEndian.Uint32(buf[n:])
		n += 8
		tkhd.Duration = binary.BigEndian.Uint64(buf[n:])
		n += 8
	} else {
		tkhd.CreationTime = uint64(binary.BigEndian.Uint32(buf[n:]))
		n += 4
		tkhd.ModificationTime = uint64(binary.BigEndian.Uint32(buf[n:]))
		n += 4
		tkhd.TrackId = binary.BigEndian.Uint32(buf[n:])
		n += 8
		tkhd.Duration = uint64(binary.BigEndian.Uint32(buf[n:]))
		n += 4
	}
	n += 8
	tkhd.Layer = binary.BigEndian.Uint16(buf[n:])
	n += 2
	tkhd.AlternateGroup = binary.BigEndian.Uint16(buf[n:])
	n += 2
	tkhd.Volume = binary.BigEndian.Uint16(buf[n:])
	n += 4
	for i := 0; i < 9; i++ {
		tkhd.Matrix[i] = binary.BigEndian.Uint32(buf[n:])
		n += 4
	}
	tkhd.Width = binary.BigEndian.Uint32(buf[n:])
	tkhd.Height = binary.BigEndian.Uint32(buf[n+4:])
	offset += n + 8
	return
}

func (tkhd *TrackHeaderBox) Encode() (int, []byte) {
	tkhd.Box.Box.Size = tkhd.Size()
	if tkhd.Duration > 0xFFFFFFFF {
		tkhd.Box.Version = 1
	}
	offset, buf := tkhd.Box.Encode()
	if tkhd.Box.Version == 1 {
		binary.BigEndian.PutUint64(buf[offset:], tkhd.CreationTime)
		offset += 8
		binary.BigEndian.PutUint64(buf[offset:], tkhd.CreationTime)
		offset += 8
		binary.BigEndian.PutUint32(buf[offset:], tkhd.TrackId)
		offset += 8
		binary.BigEndian.PutUint64(buf[offset:], tkhd.Duration)
		offset += 8
	} else {
		binary.BigEndian.PutUint32(buf[offset:], uint32(tkhd.CreationTime))
		offset += 4
		binary.BigEndian.PutUint32(buf[offset:], uint32(tkhd.CreationTime))
		offset += 4
		binary.BigEndian.PutUint32(buf[offset:], tkhd.TrackId)
		offset += 8
		binary.BigEndian.PutUint32(buf[offset:], uint32(tkhd.Duration))
		offset += 4
	}
	offset += 8
	binary.BigEndian.PutUint16(buf[offset:], tkhd.Layer)
	offset += 2
	binary.BigEndian.PutUint16(buf[offset:], tkhd.AlternateGroup)
	offset += 2
	binary.BigEndian.PutUint16(buf[offset:], tkhd.Volume)
	offset += 4
	for i := range tkhd.Matrix {
		binary.BigEndian.PutUint32(buf[offset:], tkhd.Matrix[i])
		offset += 4
	}
	binary.BigEndian.PutUint32(buf[offset:], uint32(tkhd.Width))
	offset += 4
	binary.BigEndian.PutUint32(buf[offset:], uint32(tkhd.Height))
	return offset + 4, buf
}

func makeTkhdBox(track *mp4track) []byte {
	tkhd := NewTrackHeaderBox()
	tkhd.Duration = uint64(track.duration)
	tkhd.TrackId = track.trackId
	//  flags is a 24-bit integer with flags; the following values are defined:
	// Track_enabled: Indicates that the track is enabled. Flag value is 0x000001. A disabled track (the low bit is zero) is treated as if it were not present.
	// Track_in_movie: Indicates that the track is used in the presentation. Flag value is 0x000002.
	// Track_in_preview: Indicates that the track is used when previewing the presentation. Flag value is 0x000004.
	tkhd.Box.Flags[2] = 0x03 //Track_enabled | Track_in_movie
	if track.cid == Mp4CodecAac || track.cid == Mp4CodecG711a || track.cid == Mp4CodecG711u || track.cid == Mp4CodecOpus {
		tkhd.Volume = 0x0100
	} else {
		tkhd.Width = track.width << 16
		tkhd.Height = track.height << 16
	}
	_, tkhdbox := tkhd.Encode()
	return tkhdbox
}

func decodeTkhdBox(demuxer *MovDemuxer) (err error) {
	tkhd := TrackHeaderBox{Box: new(FullBox)}
	if _, err = tkhd.Decode(demuxer.reader); err != nil {
		return err
	}
	track := demuxer.tracks[len(demuxer.tracks)-1]
	track.duration = uint32(tkhd.Duration)
	track.trackId = tkhd.TrackId
	return
}
