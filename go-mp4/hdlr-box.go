package mp4

import (
	"bytes"
	"io"
)

// Box Type: 'hdlr'
// Container: Media Box (‘mdia’) or Meta Box (‘meta’)
// Mandatory: Yes
// Quantity: Exactly one

// aligned(8) class HandlerBox extends FullBox(‘hdlr’, version = 0, 0) {
//  unsigned int(32) pre_defined = 0;
// 	unsigned int(32) handler_type;
// 	const unsigned int(32)[3] reserved = 0;
// 	   string   name;
// 	}

// handler_type
// value from a derived specification:
// ‘vide’ Video track
// ‘soun’ Audio track
// ‘hint’ Hint track
// ‘meta’ Timed Metadata track
// ‘auxv’ Auxiliary Video track

type HandlerType [4]byte

var vide = HandlerType{'v', 'i', 'd', 'e'}
var soun = HandlerType{'s', 'o', 'u', 'n'}
var hint = HandlerType{'h', 'i', 'n', 't'}
var meta = HandlerType{'m', 'e', 't', 'a'}
var auxv = HandlerType{'a', 'u', 'x', 'v'}

func (ht HandlerType) equal(other HandlerType) bool {
	return bytes.Equal(ht[:], other[:])
}

type HandlerBox struct {
	Box         *FullBox
	HandlerType HandlerType
	Name        string
}

func NewHandlerBox(handlerType HandlerType, name string) *HandlerBox {
	return &HandlerBox{
		Box:         NewFullBox([4]byte{'h', 'd', 'l', 'r'}, 0),
		HandlerType: handlerType,
		Name:        name,
	}
}

func (hdlr *HandlerBox) Size() uint64 {
	return hdlr.Box.Size() + 20 + uint64(len(hdlr.Name)+1)
}

func (hdlr *HandlerBox) Decode(r io.Reader, size uint64) (offset int, err error) {
	if _, err = hdlr.Box.Decode(r); err != nil {
		return 0, err
	}
	buf := make([]byte, size-FullBoxLen)
	if _, err = io.ReadFull(r, buf); err != nil {
		return 0, err
	}
	offset = 0
	hdlr.HandlerType[0] = buf[offset]
	hdlr.HandlerType[1] = buf[offset+1]
	hdlr.HandlerType[2] = buf[offset+2]
	hdlr.HandlerType[3] = buf[offset+3]
	offset += 4
	hdlr.Name = string(buf[offset : size-FullBoxLen])
	offset = int(size - FullBoxLen)
	return
}

func (hdlr *HandlerBox) Encode() (int, []byte) {
	hdlr.Box.Box.Size = hdlr.Size()
	offset, buf := hdlr.Box.Encode()
	offset += 4
	buf[offset] = hdlr.HandlerType[0]
	buf[offset+1] = hdlr.HandlerType[1]
	buf[offset+2] = hdlr.HandlerType[2]
	buf[offset+3] = hdlr.HandlerType[3]
	offset += 16
	copy(buf[offset:], []byte(hdlr.Name))
	return offset + len(hdlr.Name), buf
}

func getHandlerType(cid Mp4CodecType) HandlerType {
	switch cid {
	case Mp4CodecH264, Mp4CodecH265:
		return vide
	case Mp4CodecAac, Mp4CodecG711a, Mp4CodecG711u,
		Mp4CodecMp2, Mp4CodecMp3, Mp4CodecOpus:
		return soun
	default:
		panic("unsupport codec id")
	}
}

func makeHdlrBox(hdt HandlerType) []byte {
	var hdlr *HandlerBox = nil
	if hdt.equal(vide) {
		hdlr = NewHandlerBox(hdt, "VideoHandler")
	} else if hdt.equal(soun) {
		hdlr = NewHandlerBox(hdt, "SoundHandler")
	} else {
		hdlr = NewHandlerBox(hdt, "")
	}
	_, boxdata := hdlr.Encode()
	return boxdata
}

func decodeHdlrBox(demuxer *MovDemuxer, size uint64) (err error) {
	hdlr := HandlerBox{Box: new(FullBox)}
	_, err = hdlr.Decode(demuxer.reader, size)
	return
}
