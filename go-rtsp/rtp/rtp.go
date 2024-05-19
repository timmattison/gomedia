package rtp

import (
	"encoding/binary"
	"errors"
)

type RtpHookFunc func(pkg *RtpPacket)
type OnRtpPktFunc func(pkt []byte) error
type OnFrameFunc func(frame []byte, timestamp uint32, lost bool)

type Packer interface {
	Pack(data []byte, timestamp uint32) error
	HookRtp(cb RtpHookFunc)
	SetMtu(mtu int)
	OnPacket(onPkt OnRtpPktFunc)
}

type CommPacker struct {
	onPacket OnRtpPktFunc
	onRtp    RtpHookFunc
	mtu      int
}

func (pack *CommPacker) OnPacket(onPkt OnRtpPktFunc) {
	pack.onPacket = onPkt
}

func (pack *CommPacker) SetMtu(mtu int) {
	pack.mtu = mtu
}

func (pack *CommPacker) HookRtp(cb RtpHookFunc) {
	pack.onRtp = cb
}

type UnPacker interface {
	OnFrame(onframe OnFrameFunc)
	UnPack(pkt []byte) error
	HookRtp(cb RtpHookFunc)
}

type CommUnPacker struct {
	onFrame OnFrameFunc
	onRtp   RtpHookFunc
}

func (unpack *CommUnPacker) OnFrame(onframe OnFrameFunc) {
	unpack.onFrame = onframe
}

func (unpack *CommUnPacker) HookRtp(cb RtpHookFunc) {
	unpack.onRtp = cb
}

type RtpPacket struct {
	Header     RtpHdr
	Extensions []byte
	Payload    []byte
	Padding    []byte
}

func (pkg *RtpPacket) Decode(data []byte) error {
	offset, err := pkg.Header.Decode(data)
	if err != nil {
		return err
	}

	data = data[offset:]
	if pkg.Header.ExtensionFlag > 0 {
		if len(data) < 4 {
			return errors.New("rtp extension need 4 bytes at least")
		}
		length := binary.BigEndian.Uint16(data[2:])
		if len(data)-4 < int(length)*4 {
			return errors.New("rtp extension need more bytes")
		}
		pkg.Extensions = data[:4+4*length]
		data = data[4+4*length:]
	}
	if pkg.Header.PaddingFlag > 0 {
		if len(data) == 0 || int(data[len(data)-1]) > len(data) {
			return errors.New("rtp padding need more bytes")
		}
		pkg.Padding = data[len(data)-int(data[len(data)-1]):]
		data = data[:len(data)-int(data[len(data)-1])]
	}
	pkg.Payload = data
	return nil
}

func (pkg *RtpPacket) Encode() []byte {
	if len(pkg.Extensions) > 0 {
		pkg.Header.ExtensionFlag = 1
	}
	if len(pkg.Padding) > 0 {
		pkg.Header.PaddingFlag = 1
	}

	hdr := pkg.Header.Encode()

	data := make([]byte, 0, len(hdr)+len(pkg.Extensions)+len(pkg.Payload)+len(pkg.Padding))
	data = append(data, hdr...)
	data = append(data, pkg.Extensions...)
	data = append(data, pkg.Payload...)
	data = append(data, pkg.Padding...)
	return data
}
