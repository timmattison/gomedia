package codec

import (
	"bytes"
	"errors"
)

// nal_unit_header() {
//     forbidden_zero_bit      f(1)
//     nal_unit_type           u(6)
//     nuh_layer_id            u(6)
//     nuh_temporal_id_plus1   u(3)
// }

type H265NaluHdr struct {
	ForbiddenZeroBit   uint8
	NalUnitType        uint8
	NuhLayerId         uint8
	NuhTemporalIdPlus1 uint8
}

func (hdr *H265NaluHdr) Decode(bs *BitStream) {
	hdr.ForbiddenZeroBit = bs.GetBit()
	hdr.NalUnitType = bs.Uint8(6)
	hdr.NuhLayerId = bs.Uint8(6)
	hdr.NuhTemporalIdPlus1 = bs.Uint8(3)
}

type VPS struct {
	VpsVideoParameterSetId             uint8
	VpsBaseLayerInternalFlag           uint8
	VpsBaseLayerAvailableFlag          uint8
	VpsMaxLayersMinus1                 uint8
	VpsMaxSubLayersMinus1              uint8
	VpsTemporalIdNestingFlag           uint8
	VpsReserved0xffff16bits            uint16
	Ptl                                ProfileTierLevel
	VpsSubLayerOrderingInfoPresentFlag uint8
	VpsMaxDecPicBufferingMinus1        [8]uint64
	VpsMaxNumReorderPics               [8]uint64
	VpsMaxLatencyIncreasePlus1         [8]uint64
	VpsMaxLayerId                      uint8
	VpsNumLayerSetsMinus1              uint64
	LayerIdIncludedFlag                [][]uint8
	VpsTimingInfoPresentFlag           uint8
	TimeInfo                           VPSTimeInfo
	// Vps_extension_flag                       uint8
}

type VPSTimeInfo struct {
	VpsNumUnitsInTick              uint32
	VpsTimeScale                   uint32
	VpsPocProportionalToTimingFlag uint8
	VpsNumTicksPocDiffOneMinus1    uint64
	VpsNumHrdParameters            uint64
	HrdLayerSetIdx                 []uint64
	CprmsPresentFlag               []uint8
}

type ProfileTierLevel struct {
	GeneralProfileSpace             uint8
	GeneralTierFlag                 uint8
	GeneralProfileIdc               uint8
	GeneralProfileCompatibilityFlag uint32
	GeneralConstraintIndicatorFlag  uint64
	GeneralLevelIdc                 uint8
	SubLayerProfilePresentFlag      [8]uint8
	SubLayerLevelPresentFlag        [8]uint8
}

// nalu without startcode
func (vps *VPS) Decode(nalu []byte) {
	sodb := CovertRbspToSodb(nalu)
	bs := NewBitStream(sodb)
	hdr := H265NaluHdr{}
	hdr.Decode(bs)
	vps.VpsVideoParameterSetId = bs.Uint8(4)
	vps.VpsBaseLayerInternalFlag = bs.Uint8(1)
	vps.VpsBaseLayerAvailableFlag = bs.Uint8(1)
	vps.VpsMaxLayersMinus1 = bs.Uint8(6)
	vps.VpsMaxSubLayersMinus1 = bs.Uint8(3)
	vps.VpsTemporalIdNestingFlag = bs.Uint8(1)
	vps.VpsReserved0xffff16bits = bs.Uint16(16)
	vps.Ptl = GetProfileTierLevel(1, vps.VpsMaxSubLayersMinus1, bs)
	vps.VpsSubLayerOrderingInfoPresentFlag = bs.Uint8(1)
	var i int
	if vps.VpsSubLayerOrderingInfoPresentFlag > 0 {
		i = 0
	} else {
		i = int(vps.VpsMaxSubLayersMinus1)
	}
	for ; i <= int(vps.VpsMaxSubLayersMinus1); i++ {
		vps.VpsMaxDecPicBufferingMinus1[i] = bs.ReadUE()
		vps.VpsMaxNumReorderPics[i] = bs.ReadUE()
		vps.VpsMaxLatencyIncreasePlus1[i] = bs.ReadUE()
	}
	vps.VpsMaxLayerId = bs.Uint8(6)
	vps.VpsNumLayerSetsMinus1 = bs.ReadUE()
	vps.LayerIdIncludedFlag = make([][]uint8, vps.VpsNumLayerSetsMinus1)
	for i := 1; i <= int(vps.VpsNumLayerSetsMinus1); i++ {
		vps.LayerIdIncludedFlag[i] = make([]uint8, vps.VpsMaxLayerId)
		for j := 0; j <= int(vps.VpsMaxLayerId); j++ {
			vps.LayerIdIncludedFlag[i][j] = bs.Uint8(1)
		}
	}
	vps.VpsTimingInfoPresentFlag = bs.Uint8(1)
	if vps.VpsTimingInfoPresentFlag == 1 {
		vps.TimeInfo = ParserVPSTimeinfo(bs)
	}
}

// ffmpeg hevc.c
// static void hvcc_parse_ptl(GetBitContext *gb,HEVCDecoderConfigurationRecord *hvcc,unsigned int max_sub_layers_minus1)
func GetProfileTierLevel(profilePresentFlag uint8, maxNumSubLayersMinus1 uint8, bs *BitStream) ProfileTierLevel {
	var ptl ProfileTierLevel
	ptl.GeneralProfileSpace = bs.Uint8(2)
	ptl.GeneralTierFlag = bs.Uint8(1)
	ptl.GeneralProfileIdc = bs.Uint8(5)
	ptl.GeneralProfileCompatibilityFlag = bs.Uint32(32)
	ptl.GeneralConstraintIndicatorFlag = bs.GetBits(48)
	ptl.GeneralLevelIdc = bs.Uint8(8)
	for i := 0; i < int(maxNumSubLayersMinus1); i++ {
		ptl.SubLayerProfilePresentFlag[i] = bs.GetBit()
		ptl.SubLayerLevelPresentFlag[i] = bs.GetBit()
	}
	if maxNumSubLayersMinus1 > 0 {
		for i := maxNumSubLayersMinus1; i < 8; i++ {
			bs.SkipBits(2)
		}
	}

	for i := 0; i < int(maxNumSubLayersMinus1); i++ {
		if ptl.SubLayerProfilePresentFlag[i] == 1 {
			/*
			 * sub_layer_profile_space[i]                     u(2)
			 * sub_layer_tier_flag[i]                         u(1)
			 * sub_layer_profile_idc[i]                       u(5)
			 * sub_layer_profile_compatibility_flag[i][0..31] u(32)
			 * sub_layer_progressive_source_flag[i]           u(1)
			 * sub_layer_interlaced_source_flag[i]            u(1)
			 * sub_layer_non_packed_constraint_flag[i]        u(1)
			 * sub_layer_frame_only_constraint_flag[i]        u(1)
			 * sub_layer_reserved_zero_44bits[i]              u(44)
			 */
			bs.SkipBits(88)
		}
		if ptl.SubLayerLevelPresentFlag[i] == 1 {
			bs.SkipBits(8)
		}
	}
	return ptl
}

func ParserVPSTimeinfo(bs *BitStream) VPSTimeInfo {
	var ti VPSTimeInfo
	ti.VpsNumUnitsInTick = bs.Uint32(32)
	ti.VpsTimeScale = bs.Uint32(32)
	ti.VpsPocProportionalToTimingFlag = bs.Uint8(1)
	if ti.VpsPocProportionalToTimingFlag == 1 {
		ti.VpsNumTicksPocDiffOneMinus1 = bs.ReadUE()
	}
	ti.VpsNumHrdParameters = bs.ReadUE()
	// for i := 0; i < int(ti.Vps_num_hrd_parameters); i++ {
	//     ti.Hrd_layer_set_idx[i] = bs.ReadUE()
	//     if i > 0 {
	//         ti.Cprms_present_flag[i] = bs.Uint8(1)
	//     }
	//     //Hrd_parameters(ti.Cprms_present_flag[i])
	// }
	return ti
}

type H265RawSPS struct {
	SpsVideoParameterSetId             uint8
	SpsMaxSubLayersMinus1              uint8
	SpsTemporalIdNestingFlag           uint8
	Ptl                                ProfileTierLevel
	SpsSeqParameterSetId               uint64
	ChromaFormatIdc                    uint64
	PicWidthInLumaSamples              uint64
	PicHeightInLumaSamples             uint64
	ConformanceWindowFlag              uint8
	ConfWinLeftOffset                  uint64
	ConfWinRightOffset                 uint64
	ConfWinTopOffset                   uint64
	ConfWinBottomOffset                uint64
	BitDepthLumaMinus8                 uint64
	BitDepthChromaMinus8               uint64
	Log2MaxPicOrderCntLsbMinus4        uint64
	SpsSubLayerOrderingInfoPresentFlag uint8
	VuiParametersPresentFlag           uint8
	Vui                                VuiParameters
}

// nalu without startcode
func (sps *H265RawSPS) Decode(nalu []byte) {
	sodb := CovertRbspToSodb(nalu)
	bs := NewBitStream(sodb)
	hdr := H265NaluHdr{}
	hdr.Decode(bs)
	sps.SpsVideoParameterSetId = bs.Uint8(4)
	sps.SpsMaxSubLayersMinus1 = bs.Uint8(3)
	sps.SpsTemporalIdNestingFlag = bs.Uint8(1)
	sps.Ptl = GetProfileTierLevel(1, sps.SpsMaxSubLayersMinus1, bs)
	sps.SpsSeqParameterSetId = bs.ReadUE()
	sps.ChromaFormatIdc = bs.ReadUE()
	if sps.ChromaFormatIdc == 3 {
		bs.SkipBits(1)
	}
	sps.PicWidthInLumaSamples = bs.ReadUE()
	sps.PicHeightInLumaSamples = bs.ReadUE()
	sps.ConformanceWindowFlag = bs.Uint8(1)
	if sps.ConformanceWindowFlag == 1 {
		sps.ConfWinLeftOffset = bs.ReadUE()
		sps.ConfWinRightOffset = bs.ReadUE()
		sps.ConfWinTopOffset = bs.ReadUE()
		sps.ConfWinBottomOffset = bs.ReadUE()
	}
	sps.BitDepthLumaMinus8 = bs.ReadUE()
	sps.BitDepthChromaMinus8 = bs.ReadUE()
	sps.Log2MaxPicOrderCntLsbMinus4 = bs.ReadUE()
	sps.SpsSubLayerOrderingInfoPresentFlag = bs.Uint8(1)
	i := 0
	if sps.SpsSubLayerOrderingInfoPresentFlag == 0 {
		i = int(sps.SpsMaxSubLayersMinus1)
	}
	for ; i <= int(sps.SpsMaxSubLayersMinus1); i++ {
		bs.ReadUE()
		bs.ReadUE()
		bs.ReadUE()
	}

	bs.ReadUE() // log2_min_luma_coding_block_size_minus3
	bs.ReadUE() // log2_diff_max_min_luma_coding_block_size
	bs.ReadUE() // log2_min_transform_block_size_minus2
	bs.ReadUE() // log2_diff_max_min_transform_block_size
	bs.ReadUE() // max_transform_hierarchy_depth_inter
	bs.ReadUE() // max_transform_hierarchy_depth_intra
	scalingListEnabledFlag := bs.GetBit()
	if scalingListEnabledFlag > 0 {
		spsScalingListDataPresentFlag := bs.GetBit()
		if spsScalingListDataPresentFlag > 0 {
			scalingListData(bs)
		}
	}

	bs.SkipBits(1)
	bs.SkipBits(1)
	if bs.GetBit() == 1 {
		bs.GetBits(4)
		bs.GetBits(4)
		bs.ReadUE()
		bs.ReadUE()
		bs.GetBit()
	}
	numShortTermRefPicSets := bs.ReadUE()
	if numShortTermRefPicSets > 64 {
		panic("beyond HEVC_MAX_SHORT_TERM_REF_PIC_SETS")
	}
	var numDeltaPocs [64]uint32
	for i := 0; i < int(numShortTermRefPicSets); i++ {
		parseRps(i, numShortTermRefPicSets, numDeltaPocs, bs)
	}
	if bs.GetBit() == 1 {
		numLongTermRefPicsSps := bs.ReadUE()
		for i := 0; i < int(numLongTermRefPicsSps); i++ {
			length := Min(int(sps.Log2MaxPicOrderCntLsbMinus4+4), 16)
			bs.SkipBits(length)
			bs.SkipBits(1)
		}
	}
	bs.SkipBits(1)
	bs.SkipBits(1)
	sps.VuiParametersPresentFlag = bs.GetBit()
	if sps.VuiParametersPresentFlag == 1 {
		sps.Vui.Decode(bs, sps.SpsMaxSubLayersMinus1)
	}
}

type VuiParameters struct {
	AspectRatioInfoPresentFlag         uint8
	OverscanInfoPresentFlag            uint8
	ChromaLocInfoPresentFlag           uint8
	NeutralChromaIndicationFlag        uint8
	FieldSeqFlag                       uint8
	FrameFieldInfoPresentFlag          uint8
	DefaultDisplayWindowFlag           uint8
	VuiTimingInfoPresentFlag           uint8
	VuiNumUnitsInTick                  uint32
	VuiTimeScale                       uint32
	VuiPocProportionalToTimingFlag     uint8
	VuiHrdParametersPresentFlag        uint8
	BitstreamRestrictionFlag           uint8
	TilesFixedStructureFlag            uint8
	MotionVectorsOverPicBoundariesFlag uint8
	RestrictedRefPicListsFlag          uint8
	MinSpatialSegmentationIdc          uint64
	MaxBytesPerPicDenom                uint64
	MaxBitsPerMinCuDenom               uint64
	Log2MaxMvLengthHorizontal          uint64
	Log2MaxMvLengthVertical            uint64
}

func (vui *VuiParameters) Decode(bs *BitStream, maxSubLayersMinus1 uint8) {
	vui.AspectRatioInfoPresentFlag = bs.Uint8(1)
	if vui.AspectRatioInfoPresentFlag == 1 {
		if bs.Uint8(8) == 255 {
			bs.SkipBits(32)
		}
	}
	vui.OverscanInfoPresentFlag = bs.Uint8(1)
	if vui.OverscanInfoPresentFlag == 1 {
		bs.SkipBits(1)
	}
	if bs.GetBit() == 1 {
		bs.SkipBits(4)
		if bs.GetBit() == 1 {
			bs.SkipBits(24)
		}
	}
	vui.ChromaLocInfoPresentFlag = bs.GetBit()
	if vui.ChromaLocInfoPresentFlag == 1 {
		bs.ReadUE()
		bs.ReadUE()
	}
	vui.NeutralChromaIndicationFlag = bs.GetBit()
	vui.FieldSeqFlag = bs.GetBit()
	vui.FrameFieldInfoPresentFlag = bs.GetBit()
	vui.DefaultDisplayWindowFlag = bs.GetBit()
	if vui.DefaultDisplayWindowFlag == 1 {
		bs.ReadUE()
		bs.ReadUE()
		bs.ReadUE()
		bs.ReadUE()
	}
	vui.VuiTimingInfoPresentFlag = bs.GetBit()
	if vui.VuiTimingInfoPresentFlag == 1 {
		vui.VuiNumUnitsInTick = bs.Uint32(32)
		vui.VuiTimeScale = bs.Uint32(32)
		vui.VuiPocProportionalToTimingFlag = bs.GetBit()
		if vui.VuiPocProportionalToTimingFlag == 1 {
			bs.ReadUE()
		}
		vui.VuiHrdParametersPresentFlag = bs.GetBit()
		if vui.VuiHrdParametersPresentFlag == 1 {
			skipHrdParameters(1, uint32(maxSubLayersMinus1), bs)
		}
	}
	vui.BitstreamRestrictionFlag = bs.GetBit()
	if vui.BitstreamRestrictionFlag == 1 {
		vui.TilesFixedStructureFlag = bs.GetBit()
		vui.MotionVectorsOverPicBoundariesFlag = bs.GetBit()
		vui.RestrictedRefPicListsFlag = bs.GetBit()
		vui.MinSpatialSegmentationIdc = bs.ReadUE()
		vui.MaxBytesPerPicDenom = bs.ReadUE()
		vui.MaxBitsPerMinCuDenom = bs.ReadUE()
		vui.Log2MaxMvLengthHorizontal = bs.ReadUE()
		vui.Log2MaxMvLengthVertical = bs.ReadUE()
	}
}

func skipHrdParameters(cprmsPresentFlag uint8, maxSubLayersMinus1 uint32, bs *BitStream) {
	nalHrdParametersPresentFlag := uint8(0)
	vclHrdParametersPresentFlag := uint8(0)
	subPicHrdParamsPresentFlag := uint8(0)
	if cprmsPresentFlag == 1 {
		nalHrdParametersPresentFlag = bs.GetBit()
		vclHrdParametersPresentFlag = bs.GetBit()

		if nalHrdParametersPresentFlag == 1 || vclHrdParametersPresentFlag == 1 {
			subPicHrdParamsPresentFlag = bs.GetBit()

			if subPicHrdParamsPresentFlag == 1 {
				/*
				 * tick_divisor_minus2                          u(8)
				 * du_cpb_removal_delay_increment_length_minus1 u(5)
				 * sub_pic_cpb_params_in_pic_timing_sei_flag    u(1)
				 * dpb_output_delay_du_length_minus1            u(5)
				 */
				bs.SkipBits(19)
			}

			bs.SkipBits(8)

			if subPicHrdParamsPresentFlag == 1 {
				// cpb_size_du_scale
				bs.SkipBits(4)
			}

			/*
			 * initial_cpb_removal_delay_length_minus1 u(5)
			 * au_cpb_removal_delay_length_minus1      u(5)
			 * dpb_output_delay_length_minus1          u(5)
			 */
			bs.SkipBits(15)
		}
	}
	for i := 0; i <= int(maxSubLayersMinus1); i++ {
		fixedPicRateGeneralFlag := bs.GetBit()
		fixedPicRateWithinCvsFlag := uint8(0)
		lowDelayHrdFlag := uint8(0)
		cpbCntMinus1 := uint32(0)
		if fixedPicRateGeneralFlag == 0 {
			fixedPicRateWithinCvsFlag = bs.GetBit()
		}
		if fixedPicRateWithinCvsFlag == 1 {
			bs.ReadUE()
		} else {
			lowDelayHrdFlag = bs.GetBit()
		}
		if lowDelayHrdFlag == 0 {
			cpbCntMinus1 = uint32(bs.ReadUE())
			if cpbCntMinus1 > 31 {
				panic("cpb_cnt_minus1 > 31")
			}
		}
		skipSubLayerHrdParameters := func() {
			for i := 0; i < int(cpbCntMinus1); i++ {
				bs.ReadUE()
				bs.ReadUE()
				if subPicHrdParamsPresentFlag == 1 {
					bs.ReadUE()
					bs.ReadUE()
				}
				bs.SkipBits(1)
			}
		}
		if nalHrdParametersPresentFlag == 1 {
			skipSubLayerHrdParameters()
		}
		if vclHrdParametersPresentFlag == 1 {
			skipSubLayerHrdParameters()
		}
	}
}

func scalingListData(bs *BitStream) {
	for i := 0; i < 4; i++ {
		maxj := 6
		if i == 3 {
			maxj = 2
		}
		for j := 0; j < maxj; j++ {
			if bs.GetBit() == 0 {
				bs.ReadUE()
			} else {
				numCoeffs := Min(64, 1<<(4+(i<<1)))
				if i > 1 {
					bs.ReadSE()
				}
				for k := 0; k < numCoeffs; k++ {
					bs.ReadSE()
				}
			}
		}
	}
}

func parseRps(rpsIdx int, numsRps uint64, numDeltaPocs [64]uint32, bs *BitStream) {
	if rpsIdx > 0 && bs.GetBit() > 0 {
		if rpsIdx > int(numsRps) {
			panic("rps_idx > int(nums_rps)")
		}
		bs.SkipBits(1)
		bs.ReadUE()
		numDeltaPocs[rpsIdx] = 0
		for i := uint32(0); i <= numDeltaPocs[rpsIdx-1]; i++ {
			var useDeltaFlag uint8
			var usedByCurrPicFlag = bs.GetBit()
			if usedByCurrPicFlag == 0 {
				useDeltaFlag = bs.GetBit()
			}
			if useDeltaFlag > 0 || usedByCurrPicFlag > 0 {
				numDeltaPocs[rpsIdx]++
			}
		}
	} else {
		numNegativePics := bs.ReadUE()
		numPositivePics := bs.ReadUE()
		if (numNegativePics+numPositivePics)*2 > uint64(bs.RemainBits()) {
			panic("(num_negative_pics + num_positive_pics) * 2> uint64(bs.RemainBits())")
		}
		for i := 0; i < int(numNegativePics); i++ {
			bs.ReadUE()
			bs.SkipBits(1)
		}
		for i := 0; i < int(numPositivePics); i++ {
			bs.ReadUE()
			bs.SkipBits(1)
		}
	}
}

type H265RawPPS struct {
	PpsPicParameterSetId               uint64
	PpsSeqParameterSetId               uint64
	DependentSliceSegmentsEnabledFlag  uint8
	OutputFlagPresentFlag              uint8
	NumExtraSliceHeaderBits            uint8
	SignDataHidingEnabledFlag          uint8
	CabacInitPresentFlag               uint8
	NumRefIdxL0DefaultActiveMinus1     uint64
	NumRefIdxL1DefaultActiveMinus1     uint64
	InitQpMinus26                      int64
	ConstrainedIntraPredFlag           uint8
	TransformSkipEnabledFlag           uint8
	CuQpDeltaEnabledFlag               uint8
	DiffCuQpDeltaDepth                 uint64
	PpsCbQpOffset                      int64
	PpsCrQpOffset                      int64
	PpsSliceChromaQpOffsetsPresentFlag uint8
	WeightedPredFlag                   uint8
	WeightedBipredFlag                 uint8
	TransquantBypassEnabledFlag        uint8
	TilesEnabledFlag                   uint8
	EntropyCodingSyncEnabledFlag       uint8
}

// nalu without startcode
func (pps *H265RawPPS) Decode(nalu []byte) {
	sodb := CovertRbspToSodb(nalu)
	bs := NewBitStream(sodb)
	hdr := H265NaluHdr{}
	hdr.Decode(bs)
	pps.PpsPicParameterSetId = bs.ReadUE()
	pps.PpsSeqParameterSetId = bs.ReadUE()
	pps.DependentSliceSegmentsEnabledFlag = bs.GetBit()
	pps.OutputFlagPresentFlag = bs.GetBit()
	pps.NumExtraSliceHeaderBits = bs.Uint8(3)
	pps.SignDataHidingEnabledFlag = bs.GetBit()
	pps.CabacInitPresentFlag = bs.GetBit()
	pps.NumRefIdxL0DefaultActiveMinus1 = bs.ReadUE()
	pps.NumRefIdxL1DefaultActiveMinus1 = bs.ReadUE()
	pps.InitQpMinus26 = bs.ReadSE()
	pps.ConstrainedIntraPredFlag = bs.GetBit()
	pps.TransformSkipEnabledFlag = bs.GetBit()
	pps.CuQpDeltaEnabledFlag = bs.GetBit()
	if pps.CuQpDeltaEnabledFlag == 1 {
		pps.DiffCuQpDeltaDepth = bs.ReadUE()
	}
	pps.PpsCbQpOffset = bs.ReadSE()
	pps.PpsCrQpOffset = bs.ReadSE()
	pps.PpsSliceChromaQpOffsetsPresentFlag = bs.GetBit()
	pps.WeightedPredFlag = bs.GetBit()
	pps.WeightedBipredFlag = bs.GetBit()
	pps.TransquantBypassEnabledFlag = bs.GetBit()
	pps.TilesEnabledFlag = bs.GetBit()
	pps.EntropyCodingSyncEnabledFlag = bs.GetBit()
}

func GetH265Resolution(sps []byte) (width uint32, height uint32) {
	start, sc := FindStartCode(sps, 0)
	h265sps := H265RawSPS{}
	h265sps.Decode(sps[start+int(sc):])
	width = uint32(h265sps.PicWidthInLumaSamples)
	height = uint32(h265sps.PicHeightInLumaSamples)
	return
}

func GetVPSIdWithStartCode(vps []byte) uint8 {
	start, sc := FindStartCode(vps, 0)
	return GetVPSId(vps[start+int(sc):])
}

func GetVPSId(vps []byte) uint8 {
	var rawvps VPS
	rawvps.Decode(vps)
	return rawvps.VpsVideoParameterSetId
}

func GetH265SPSIdWithStartCode(sps []byte) uint64 {
	start, sc := FindStartCode(sps, 0)
	return GetH265SPSId(sps[start+int(sc):])
}

func GetH265SPSId(sps []byte) uint64 {
	var rawsps H265RawSPS
	rawsps.Decode(sps)
	return rawsps.SpsSeqParameterSetId
}

func GetH265PPSIdWithStartCode(pps []byte) uint64 {
	start, sc := FindStartCode(pps, 0)
	return GetH265SPSId(pps[start+int(sc):])
}

func GetH265PPSId(pps []byte) uint64 {
	var rawpps H265RawPPS
	rawpps.Decode(pps)
	return rawpps.PpsPicParameterSetId
}

/*
ISO/IEC 14496-15:2017(E) 8.3.3.1.2 Syntax (p71)

aligned(8) class HEVCDecoderConfigurationRecord {
    unsigned int(8) configurationVersion = 1;
    unsigned int(2) general_profile_space;
    unsigned int(1) general_tier_flag;
    unsigned int(5) general_profile_idc;
    unsigned int(32) general_profile_compatibility_flags;
    unsigned int(48) general_constraint_indicator_flags;
    unsigned int(8) general_level_idc;
    bit(4) reserved = '1111'b;
    unsigned int(12) min_spatial_segmentation_idc;
    bit(6) reserved = '111111'b;
    unsigned int(2) parallelismType;
    bit(6) reserved = '111111'b;
    unsigned int(2) chromaFormat;
    bit(5) reserved = '11111'b;
    unsigned int(3) bitDepthLumaMinus8;
    bit(5) reserved = '11111'b;
    unsigned int(3) bitDepthChromaMinus8;
    bit(16) avgFrameRate;
    bit(2) constantFrameRate;
    bit(3) numTemporalLayers;
    bit(1) temporalIdNested;
    unsigned int(2) lengthSizeMinusOne;
    unsigned int(8) numOfArrays;
    for (j=0; j < numOfArrays; j++) {
        bit(1) array_completeness;
        unsigned int(1) reserved = 0;
        unsigned int(6) NAL_unit_type;
        unsigned int(16) numNalus;
        for (i=0; i< numNalus; i++) {
            unsigned int(16) nalUnitLength;
            bit(8*nalUnitLength) nalUnit;
        }
    }
}
*/

type NalUnit struct {
	NalUnitLength uint16
	Nalu          []byte
}

type HVCCNALUnitArray struct {
	ArrayCompleteness uint8
	NalUnitType       uint8
	NumNalus          uint16
	NalUnits          []*NalUnit
}

type HEVCRecordConfiguration struct {
	ConfigurationVersion             uint8
	GeneralProfileSpace              uint8
	GeneralTierFlag                  uint8
	GeneralProfileIdc                uint8
	GeneralProfileCompatibilityFlags uint32
	GeneralConstraintIndicatorFlags  uint64
	GeneralLevelIdc                  uint8
	MinSpatialSegmentationIdc        uint16
	ParallelismType                  uint8
	ChromaFormat                     uint8
	BitDepthLumaMinus8               uint8
	BitDepthChromaMinus8             uint8
	AvgFrameRate                     uint16
	ConstantFrameRate                uint8
	NumTemporalLayers                uint8
	TemporalIdNested                 uint8
	LengthSizeMinusOne               uint8
	NumOfArrays                      uint8
	Arrays                           []*HVCCNALUnitArray
}

func NewHEVCRecordConfiguration() *HEVCRecordConfiguration {
	return &HEVCRecordConfiguration{
		ConfigurationVersion:             1,
		GeneralProfileCompatibilityFlags: 0xffffffff,
		GeneralConstraintIndicatorFlags:  0xffffffffffffffff,
		MinSpatialSegmentationIdc:        4097,
		LengthSizeMinusOne:               3,
	}
}

func (hvcc *HEVCRecordConfiguration) Encode() ([]byte, error) {
	if len(hvcc.Arrays) < 3 {
		return nil, errors.New("lack of sps or pps or vps")
	}
	bsw := NewBitStreamWriter(512)
	bsw.PutByte(hvcc.ConfigurationVersion)
	bsw.PutUint8(hvcc.GeneralProfileSpace, 2)
	bsw.PutUint8(hvcc.GeneralTierFlag, 1)
	bsw.PutUint8(hvcc.GeneralProfileIdc, 5)
	bsw.PutUint32(hvcc.GeneralProfileCompatibilityFlags, 32)
	bsw.PutUint64(hvcc.GeneralConstraintIndicatorFlags, 48)
	bsw.PutByte(hvcc.GeneralLevelIdc)
	bsw.PutUint8(0x0F, 4)
	bsw.PutUint16(hvcc.MinSpatialSegmentationIdc, 12)
	bsw.PutUint8(0x3F, 6)
	//ffmpeg hvcc_write(AVIOContext *pb, HEVCDecoderConfigurationRecord *hvcc)
	/*
	 * parallelismType indicates the type of parallelism that is used to meet
	 * the restrictions imposed by min_spatial_segmentation_idc when the value
	 * of min_spatial_segmentation_idc is greater than 0.
	 */
	if hvcc.MinSpatialSegmentationIdc == 0 {
		hvcc.ParallelismType = 0
	}
	bsw.PutUint8(hvcc.ParallelismType, 2)
	bsw.PutUint8(0x3F, 6)
	bsw.PutUint8(hvcc.ChromaFormat, 2)
	bsw.PutUint8(0x1F, 5)
	bsw.PutUint8(hvcc.BitDepthLumaMinus8, 3)
	bsw.PutUint8(0x1F, 5)
	bsw.PutUint8(hvcc.BitDepthChromaMinus8, 3)
	bsw.PutUint16(hvcc.AvgFrameRate, 16)
	bsw.PutUint8(hvcc.ConstantFrameRate, 2)
	bsw.PutUint8(hvcc.NumTemporalLayers, 3)
	bsw.PutUint8(hvcc.TemporalIdNested, 1)
	bsw.PutUint8(hvcc.LengthSizeMinusOne, 2)
	bsw.PutByte(uint8(len(hvcc.Arrays)))
	for _, arrays := range hvcc.Arrays {
		bsw.PutUint8(arrays.ArrayCompleteness, 1)
		bsw.PutUint8(0, 1)
		bsw.PutUint8(arrays.NalUnitType, 6)
		bsw.PutUint16(uint16(len(arrays.NalUnits)), 16)
		for _, nalu := range arrays.NalUnits {
			bsw.PutUint16(nalu.NalUnitLength, 16)
			bsw.PutBytes(nalu.Nalu)
		}
	}
	return bsw.Bits(), nil
}

func (hvcc *HEVCRecordConfiguration) Decode(hevc []byte) {
	bs := NewBitStream(hevc)
	hvcc.ConfigurationVersion = bs.Uint8(8)
	hvcc.GeneralProfileSpace = bs.Uint8(2)
	hvcc.GeneralTierFlag = bs.Uint8(1)
	hvcc.GeneralProfileIdc = bs.Uint8(5)
	hvcc.GeneralProfileCompatibilityFlags = bs.Uint32(32)
	hvcc.GeneralConstraintIndicatorFlags = bs.GetBits(48)
	hvcc.GeneralLevelIdc = bs.Uint8(8)
	bs.SkipBits(4)
	hvcc.MinSpatialSegmentationIdc = bs.Uint16(12)
	bs.SkipBits(6)
	hvcc.ParallelismType = bs.Uint8(2)
	bs.SkipBits(6)
	hvcc.ChromaFormat = bs.Uint8(2)
	bs.SkipBits(5)
	hvcc.BitDepthLumaMinus8 = bs.Uint8(3)
	bs.SkipBits(5)
	hvcc.BitDepthChromaMinus8 = bs.Uint8(3)
	hvcc.AvgFrameRate = bs.Uint16(16)
	hvcc.ConstantFrameRate = bs.Uint8(2)
	hvcc.NumTemporalLayers = bs.Uint8(3)
	hvcc.TemporalIdNested = bs.Uint8(1)
	hvcc.LengthSizeMinusOne = bs.Uint8(2)
	hvcc.NumOfArrays = bs.Uint8(8)
	hvcc.Arrays = make([]*HVCCNALUnitArray, hvcc.NumOfArrays)
	for i := 0; i < int(hvcc.NumOfArrays); i++ {
		hvcc.Arrays[i] = new(HVCCNALUnitArray)
		hvcc.Arrays[i].ArrayCompleteness = bs.GetBit()
		bs.SkipBits(1)
		hvcc.Arrays[i].NalUnitType = bs.Uint8(6)
		hvcc.Arrays[i].NumNalus = bs.Uint16(16)
		hvcc.Arrays[i].NalUnits = make([]*NalUnit, hvcc.Arrays[i].NumNalus)
		for j := 0; j < int(hvcc.Arrays[i].NumNalus); j++ {
			hvcc.Arrays[i].NalUnits[j] = new(NalUnit)
			hvcc.Arrays[i].NalUnits[j].NalUnitLength = bs.Uint16(16)
			hvcc.Arrays[i].NalUnits[j].Nalu = bs.GetBytes(int(hvcc.Arrays[i].NalUnits[j].NalUnitLength))
		}
	}
}

func (hvcc *HEVCRecordConfiguration) UpdateSPS(sps []byte) {
	start, sc := FindStartCode(sps, 0)
	sps = sps[start+int(sc):]
	var rawsps H265RawSPS
	rawsps.Decode(sps)
	spsid := rawsps.SpsSeqParameterSetId
	var needUpdate = false
	i := 0
	for ; i < len(hvcc.Arrays); i++ {
		arrays := hvcc.Arrays[i]
		if arrays.NalUnitType != uint8(H265NalSps) {
			continue
		}
		j := 0
		for ; j < len(arrays.NalUnits); j++ {
			if spsid != GetH265SPSId(arrays.NalUnits[j].Nalu) {
				continue
			}
			//find the same sps nalu
			if arrays.NalUnits[j].NalUnitLength == uint16(len(sps)) && bytes.Equal(arrays.NalUnits[j].Nalu, sps) {
				return
			}
			tmpsps := make([]byte, len(sps))
			copy(tmpsps, sps)
			arrays.NalUnits[j].Nalu = tmpsps
			arrays.NalUnits[j].NalUnitLength = uint16(len(tmpsps))
			needUpdate = true
			break
		}
		if j == len(arrays.NalUnits) {
			nalu := &NalUnit{
				Nalu:          make([]byte, len(sps)),
				NalUnitLength: uint16(len(sps)),
			}
			copy(nalu.Nalu, sps)
			arrays.NalUnits = append(arrays.NalUnits, nalu)
			needUpdate = true
		}
		break
	}
	if i == len(hvcc.Arrays) {
		nua := &HVCCNALUnitArray{
			ArrayCompleteness: 1,
			NalUnitType:       33,
			NumNalus:          1,
			NalUnits:          make([]*NalUnit, 1),
		}
		nu := &NalUnit{
			NalUnitLength: uint16(len(sps)),
			Nalu:          make([]byte, len(sps)),
		}
		copy(nu.Nalu, sps)
		nua.NalUnits[0] = nu
		hvcc.Arrays = append(hvcc.Arrays, nua)
		needUpdate = true
	}
	if needUpdate {
		hvcc.NumTemporalLayers = uint8(Max(int(hvcc.NumTemporalLayers), int(rawsps.SpsMaxSubLayersMinus1+1)))
		hvcc.TemporalIdNested = rawsps.SpsTemporalIdNestingFlag
		hvcc.ChromaFormat = uint8(rawsps.ChromaFormatIdc)
		hvcc.BitDepthChromaMinus8 = uint8(rawsps.BitDepthChromaMinus8)
		hvcc.BitDepthLumaMinus8 = uint8(rawsps.BitDepthLumaMinus8)
		hvcc.updatePtl(rawsps.Ptl)
		hvcc.updateVui(rawsps.Vui)
	}
}

func (hvcc *HEVCRecordConfiguration) UpdatePPS(pps []byte) {
	start, sc := FindStartCode(pps, 0)
	pps = pps[start+int(sc):]
	var rawpps H265RawPPS
	rawpps.Decode(pps)
	ppsid := rawpps.PpsPicParameterSetId
	var needUpdate = false
	i := 0
	for ; i < len(hvcc.Arrays); i++ {
		arrays := hvcc.Arrays[i]
		if arrays.NalUnitType != uint8(H265NalPps) {
			continue
		}
		j := 0
		for ; j < len(arrays.NalUnits); j++ {
			if ppsid != GetH265PPSId(arrays.NalUnits[j].Nalu) {
				continue
			}
			//find the same sps nalu
			if arrays.NalUnits[j].NalUnitLength == uint16(len(pps)) && bytes.Equal(arrays.NalUnits[j].Nalu, pps) {
				return
			}
			tmppps := make([]byte, len(pps))
			copy(tmppps, pps)
			arrays.NalUnits[j].Nalu = tmppps
			arrays.NalUnits[j].NalUnitLength = uint16(len(tmppps))
			needUpdate = true
			break
		}
		if j == len(arrays.NalUnits) {
			nalu := &NalUnit{
				Nalu:          make([]byte, len(pps)),
				NalUnitLength: uint16(len(pps)),
			}
			copy(nalu.Nalu, pps)
			arrays.NalUnits = append(arrays.NalUnits, nalu)
			needUpdate = true
		}
		break
	}
	if i == len(hvcc.Arrays) {
		nua := &HVCCNALUnitArray{
			ArrayCompleteness: 1,
			NalUnitType:       34,
			NumNalus:          1,
			NalUnits:          make([]*NalUnit, 1),
		}
		nu := &NalUnit{
			NalUnitLength: uint16(len(pps)),
			Nalu:          make([]byte, len(pps)),
		}
		copy(nu.Nalu, pps)
		nua.NalUnits[0] = nu
		hvcc.Arrays = append(hvcc.Arrays, nua)
		needUpdate = true
	}
	if needUpdate {
		if rawpps.EntropyCodingSyncEnabledFlag == 1 && rawpps.TilesEnabledFlag == 1 {
			hvcc.ParallelismType = 0
		} else if rawpps.EntropyCodingSyncEnabledFlag == 1 {
			hvcc.ParallelismType = 3
		} else if rawpps.TilesEnabledFlag == 1 {
			hvcc.ParallelismType = 2
		} else {
			hvcc.ParallelismType = 1
		}
	}
}

func (hvcc *HEVCRecordConfiguration) UpdateVPS(vps []byte) {
	start, sc := FindStartCode(vps, 0)
	vps = vps[start+int(sc):]
	var rawvps VPS
	rawvps.Decode(vps)
	vpsid := rawvps.VpsVideoParameterSetId
	var needUpdate = false
	i := 0
	for ; i < len(hvcc.Arrays); i++ {
		arrays := hvcc.Arrays[i]
		if arrays.NalUnitType != uint8(H265NalVps) {
			continue
		}
		j := 0
		for ; j < len(arrays.NalUnits); j++ {
			if vpsid != GetVPSId(arrays.NalUnits[j].Nalu) {
				continue
			}
			//find the same sps nalu
			if arrays.NalUnits[j].NalUnitLength == uint16(len(vps)) && bytes.Equal(arrays.NalUnits[j].Nalu, vps) {
				return
			}
			tmpvps := make([]byte, len(vps))
			copy(tmpvps, vps)
			arrays.NalUnits[j].Nalu = tmpvps
			arrays.NalUnits[j].NalUnitLength = uint16(len(tmpvps))
			needUpdate = true
			break
		}
		if j == len(arrays.NalUnits) {
			nalu := &NalUnit{
				Nalu:          make([]byte, len(vps)),
				NalUnitLength: uint16(len(vps)),
			}
			copy(nalu.Nalu, vps)
			arrays.NalUnits = append(arrays.NalUnits, nalu)
			needUpdate = true
		}
		break
	}
	if i == len(hvcc.Arrays) {
		nua := &HVCCNALUnitArray{
			ArrayCompleteness: 1,
			NalUnitType:       32,
			NumNalus:          1,
			NalUnits:          make([]*NalUnit, 1),
		}
		nu := &NalUnit{
			NalUnitLength: uint16(len(vps)),
			Nalu:          make([]byte, len(vps)),
		}
		copy(nu.Nalu, vps)
		nua.NalUnits[0] = nu
		hvcc.Arrays = append(hvcc.Arrays, nua)
		needUpdate = true
	}
	if needUpdate {
		hvcc.NumTemporalLayers = uint8(Max(int(hvcc.NumTemporalLayers), int(rawvps.VpsMaxLayersMinus1+1)))
		hvcc.updatePtl(rawvps.Ptl)
	}
}

func (hvcc *HEVCRecordConfiguration) ToNalus() (nalus []byte) {
	startcode := []byte{0x00, 0x00, 0x00, 0x01}
	for _, arrays := range hvcc.Arrays {
		for _, unit := range arrays.NalUnits {
			nalus = append(nalus, startcode...)
			nalus = append(nalus, unit.Nalu[:unit.NalUnitLength]...)
		}
	}
	return
}

func (hvcc *HEVCRecordConfiguration) updatePtl(ptl ProfileTierLevel) {
	hvcc.GeneralProfileSpace = ptl.GeneralProfileSpace
	if hvcc.GeneralTierFlag < ptl.GeneralTierFlag {
		hvcc.GeneralLevelIdc = ptl.GeneralLevelIdc
	} else {
		hvcc.GeneralLevelIdc = uint8(Max(int(hvcc.GeneralLevelIdc), int(ptl.GeneralLevelIdc)))
	}
	hvcc.GeneralTierFlag = uint8(Max(int(hvcc.GeneralTierFlag), int(ptl.GeneralTierFlag)))
	hvcc.GeneralProfileIdc = uint8(Max(int(hvcc.GeneralProfileIdc), int(ptl.GeneralProfileIdc)))
	hvcc.GeneralProfileCompatibilityFlags &= ptl.GeneralProfileCompatibilityFlag
	hvcc.GeneralConstraintIndicatorFlags &= ptl.GeneralConstraintIndicatorFlag
}

func (hvcc *HEVCRecordConfiguration) updateVui(vui VuiParameters) {
	hvcc.MinSpatialSegmentationIdc = uint16(Min(int(hvcc.MinSpatialSegmentationIdc), int(vui.MinSpatialSegmentationIdc)))
}
