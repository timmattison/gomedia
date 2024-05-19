package mp4

import (
	"encoding/binary"
	"io"
)

var isom = [4]byte{'i', 's', 'o', 'm'}
var iso2 = [4]byte{'i', 's', 'o', '2'}
var iso3 = [4]byte{'i', 's', 'o', '3'}
var iso4 = [4]byte{'i', 's', 'o', '4'}
var iso5 = [4]byte{'i', 's', 'o', '5'}
var iso6 = [4]byte{'i', 's', 'o', '6'}
var avc1 = [4]byte{'a', 'v', 'c', '1'}
var mp41 = [4]byte{'m', 'p', '4', '1'}
var mp42 = [4]byte{'m', 'p', '4', '2'}
var dash = [4]byte{'d', 'a', 's', 'h'}
var msdh = [4]byte{'m', 's', 'd', 'h'}
var msix = [4]byte{'m', 's', 'i', 'x'}

func movTag(tag [4]byte) uint32 {
	return binary.LittleEndian.Uint32(tag[:])
}

type FileTypeBox struct {
	Box              *BasicBox
	MajorBrand       uint32
	MinorVersion     uint32
	CompatibleBrands []uint32
}

func NewFileTypeBox() *FileTypeBox {
	return &FileTypeBox{
		Box: NewBasicBox([4]byte{'f', 't', 'y', 'p'}),
	}
}

func NewSegmentTypeBox() *FileTypeBox {
	return &FileTypeBox{
		Box: NewBasicBox([4]byte{'s', 't', 'y', 'p'}),
	}
}

func (ftyp *FileTypeBox) Size() uint64 {
	return uint64(8 + len(ftyp.CompatibleBrands)*4 + 8)
}

func (ftyp *FileTypeBox) decode(r io.Reader, size uint32) (int, error) {
	buf := make([]byte, size-BasicBoxLen)
	if n, err := io.ReadFull(r, buf); err != nil {
		return n, err
	}
	ftyp.MajorBrand = binary.LittleEndian.Uint32(buf[0:])
	ftyp.MinorVersion = binary.BigEndian.Uint32(buf[4:])
	n := 8
	for ; BasicBoxLen+n < int(size); n += 4 {
		ftyp.CompatibleBrands = append(ftyp.CompatibleBrands, binary.LittleEndian.Uint32(buf[n:]))
	}
	return n, nil
}

func (ftyp *FileTypeBox) Encode() (int, []byte) {
	ftyp.Box.Size = ftyp.Size()
	offset, buf := ftyp.Box.Encode()
	binary.LittleEndian.PutUint32(buf[offset:], ftyp.MajorBrand)
	offset += 4
	binary.BigEndian.PutUint32(buf[offset:], ftyp.MinorVersion)
	offset += 4
	for i := 0; offset < int(ftyp.Box.Size); offset += 4 {
		binary.LittleEndian.PutUint32(buf[offset:], ftyp.CompatibleBrands[i])
		i++
	}
	return offset, buf
}

func decodeFtypBox(demuxer *MovDemuxer, size uint32) (err error) {
	ftyp := FileTypeBox{}
	if _, err = ftyp.decode(demuxer.reader, size); err != nil {
		return
	}
	demuxer.mp4Info.CompatibleBrands = ftyp.CompatibleBrands
	demuxer.mp4Info.MajorBrand = ftyp.MajorBrand
	demuxer.mp4Info.MinorVersion = ftyp.MinorVersion
	return
}

func makeFtypBox(major uint32, minor uint32, compatibleBrands []uint32) []byte {
	ftyp := NewFileTypeBox()
	ftyp.MajorBrand = major
	ftyp.MinorVersion = minor
	ftyp.CompatibleBrands = compatibleBrands
	_, boxData := ftyp.Encode()
	return boxData
}

func makeStypBox(major uint32, minor uint32, compatibleBrands []uint32) []byte {
	styp := NewSegmentTypeBox()
	styp.MajorBrand = major
	styp.MinorVersion = minor
	styp.CompatibleBrands = compatibleBrands
	_, boxData := styp.Encode()
	return boxData
}
