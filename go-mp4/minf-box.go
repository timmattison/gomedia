package mp4

func makeMinfBox(track *mp4track) []byte {
	var mhdbox []byte
	switch track.cid {
	case Mp4CodecH264, Mp4CodecH265:
		mhdbox = makeVmhdBox()
	case Mp4CodecG711a, Mp4CodecG711u, Mp4CodecAac,
		Mp4CodecMp2, Mp4CodecMp3, Mp4CodecOpus:
		mhdbox = makeSmhdBox()
	default:
		panic("unsupport codec id")
	}
	dinfbox := makeDefaultDinfBox()
	stblbox := makeStblBox(track)

	minf := BasicBox{Type: [4]byte{'m', 'i', 'n', 'f'}}
	minf.Size = 8 + uint64(len(mhdbox)+len(dinfbox)+len(stblbox))
	offset, minfbox := minf.Encode()
	copy(minfbox[offset:], mhdbox)
	offset += len(mhdbox)
	copy(minfbox[offset:], dinfbox)
	offset += len(dinfbox)
	copy(minfbox[offset:], stblbox)
	offset += len(stblbox)
	return minfbox
}
