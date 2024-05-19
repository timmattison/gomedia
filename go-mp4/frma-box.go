package mp4

import "io"

func decodeFrmaBox(demuxer *MovDemuxer, size uint32) (err error) {
	buf := make([]byte, size-BasicBoxLen)
	if _, err = io.ReadFull(demuxer.reader, buf); err != nil {
		return
	}
	var format [4]byte
	copy(format[:], buf)

	track := demuxer.tracks[len(demuxer.tracks)-1]
	switch movTag(format) {
	case movTag([4]byte{'a', 'v', 'c', '1'}):
		track.cid = Mp4CodecH264
		if track.extra == nil {
			track.extra = new(h264ExtraData)
		}
		return
	case movTag([4]byte{'m', 'p', '4', 'a'}):
		track.cid = Mp4CodecAac
		if track.extra == nil {
			track.extra = new(aacExtraData)
		}
		return
	}

	return
}
