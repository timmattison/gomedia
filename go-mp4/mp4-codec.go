package mp4

import (
	"github.com/timmattison/gomedia/go-codec"
)

type Mp4CodecType int

const (
	Mp4CodecH264 Mp4CodecType = iota + 1
	Mp4CodecH265

	Mp4CodecAac Mp4CodecType = iota + 100
	Mp4CodecG711a
	Mp4CodecG711u
	Mp4CodecMp2
	Mp4CodecMp3
	Mp4CodecOpus
)

func isVideo(cid Mp4CodecType) bool {
	return cid == Mp4CodecH264 || cid == Mp4CodecH265
}

func isAudio(cid Mp4CodecType) bool {
	return cid == Mp4CodecAac || cid == Mp4CodecG711a || cid == Mp4CodecG711u ||
		cid == Mp4CodecMp2 || cid == Mp4CodecMp3 || cid == Mp4CodecOpus
}

func getCodecNameWithCodecId(cid Mp4CodecType) [4]byte {
	switch cid {
	case Mp4CodecH264:
		return [4]byte{'a', 'v', 'c', '1'}
	case Mp4CodecH265:
		return [4]byte{'h', 'v', 'c', '1'}
	case Mp4CodecAac, Mp4CodecMp2, Mp4CodecMp3:
		return [4]byte{'m', 'p', '4', 'a'}
	case Mp4CodecG711a:
		return [4]byte{'a', 'l', 'a', 'w'}
	case Mp4CodecG711u:
		return [4]byte{'u', 'l', 'a', 'w'}
	case Mp4CodecOpus:
		return [4]byte{'o', 'p', 'u', 's'}
	default:
		panic("unsupport codec id")
	}
}

// ffmpeg isom.c const AVCodecTag ff_mp4_obj_type[]
func getBojecttypeWithCodecId(cid Mp4CodecType) uint8 {
	switch cid {
	case Mp4CodecH264:
		return 0x21
	case Mp4CodecH265:
		return 0x23
	case Mp4CodecAac:
		return 0x40
	case Mp4CodecG711a:
		return 0xfd
	case Mp4CodecG711u:
		return 0xfe
	case Mp4CodecMp2:
		return 0x6b
	case Mp4CodecMp3:
		return 0x69
	default:
		panic("unsupport codec id")
	}
}

func getCodecIdByObjectType(objType uint8) Mp4CodecType {
	switch objType {
	case 0x21:
		return Mp4CodecH264
	case 0x23:
		return Mp4CodecH265
	case 0x40:
		return Mp4CodecAac
	case 0xfd:
		return Mp4CodecG711a
	case 0xfe:
		return Mp4CodecG711u
	case 0x6b, 0x69:
		return Mp4CodecMp3
	default:
		panic("unsupport object type")
	}
}

func isH264NewAccessUnit(nalu []byte) bool {
	naluType := codec.H264NaluType(nalu)
	switch naluType {
	case codec.H264NalAud, codec.H264NalSps,
		codec.H264NalPps, codec.H264NalSei:
		return true
	case codec.H264NalISlice, codec.H264NalPSlice,
		codec.H264NalSliceA, codec.H264NalSliceB, codec.H264NalSliceC:
		firstMbInSlice := codec.GetH264FirstMbInSlice(nalu)
		if firstMbInSlice == 0 {
			return true
		}
	}
	return false
}

func isH265NewAccessUnit(nalu []byte) bool {
	naluType := codec.H265NaluType(nalu)
	switch naluType {
	case codec.H265NalAud, codec.H265NalSps,
		codec.H265NalPps, codec.H265NalSei, codec.H265NalVps:
		return true
	case codec.H265NalSliceTrailN, codec.H265NalLiceTrailR,
		codec.H265NalSliceTsaN, codec.H265NalSliceTsaR,
		codec.H265NalSliceStsaN, codec.H265NalSliceStsaR,
		codec.H265NalSliceRadlN, codec.H265NalSliceRadlR,
		codec.H265NalSliceRaslN, codec.H265NalSliceRaslR,
		codec.H265NalSliceBlaWLp, codec.H265NalSliceBlaWRadl,
		codec.H265NalSliceBlaNLp, codec.H265NalSliceIdrWRadl,
		codec.H265NalSliceIdrNLp, codec.H265NalSliceCra:
		firstMbInSlice := codec.GetH265FirstMbInSlice(nalu)
		if firstMbInSlice == 0 {
			return true
		}
	}
	return false
}
