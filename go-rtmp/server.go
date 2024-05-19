package rtmp

import (
	"encoding/binary"
	"errors"

	"github.com/timmattison/gomedia/go-codec"
	"github.com/timmattison/gomedia/go-flv"
)

//example
//1. rtmp 推流服务端
//
//listen, _ := net.Listen("tcp4", "0.0.0.0:1935")
//conn, _ := listen.Accept()
//
// handle := NewRtmpServerHandle()
// handle.OnPublish(func(app, streamName string) StatusCode {
//     return NETSTREAM_PUBLISH_START
// })
//
// handle.SetOutput(func(b []byte) error {
//     _, err := conn.Write(b)
//     return err
// })

// handle.OnFrame(func(cid codec.CodecID, pts, dts uint32, frame []byte) {
//     if cid == codec.CODECID_VIDEO_H264 {
//        //do something
//     }
//     ........
// })

//
// 把从网络中接收到的数据，input到rtmp句柄当中
// buf := make([]byte, 60000)
// for {
//     n, err := conn.Read(buf)
//     if err != nil {
//         fmt.Println(err)
//         break
//     }
//     err = handle.Input(buf[0:n])
//     if err != nil {
//         fmt.Println(err)
//         break
//     }
// }

// rtmp播放服务端
// listen, _ := net.Listen("tcp4", "0.0.0.0:1935")
// conn, _ := listen.Accept()

// ready := make(chan struct{})
// handle := NewRtmpServerHandle()
// handle.onPlay = func(app, streamName string, start, duration float64, reset bool) StatusCode {
//        return NETSTREAM_PLAY_START
//  }
//
// handle.OnStateChange(func(newstate RtmpState) {
//    if newstate == STATE_RTMP_PLAY_START {
//        close(ready) //关闭这个通道，通知推流协程可以向客户端推流了
//    }
//  })
//
//  handle.SetOutput(func(b []byte) error {
//       _, err := conn.Write(b)
//      return err
//  })
//
//  go func() {
//
//      等待推流
//      <-ready
//
//      开始推流
//      handle.WriteVideo(cid, frame, pts, dts)
//      handle.WriteAudio(cid, frame, pts, dts)
//
//  }()
//
//  把从网络中接收到的数据，input到rtmp句柄当中
//  buf := make([]byte, 60000)
//  for {
//      n, err := conn.Read(buf)
//      if err != nil {
//          fmt.Println(err)
//          break
//      }
//      err = handle.Input(buf[0:n])
//      if err != nil {
//          fmt.Println(err)
//          break
//      }
//  }
//  conn.Close()

type RtmpServerHandle struct {
	app            string
	streamName     string
	tcUrl          string
	state          RtmpParserState
	streamState    RtmpState
	cmdChan        *chunkStreamWriter
	userCtrlChan   *chunkStreamWriter
	audioChan      *chunkStreamWriter
	videoChan      *chunkStreamWriter
	reader         *chunkStreamReader
	writeChunkSize uint32
	hs             *serverHandShake
	wndAckSize     uint32
	peerWndAckSize uint32
	videoDemuxer   flv.VideoTagDemuxer
	audioDemuxer   flv.AudioTagDemuxer
	videoMuxer     flv.AVTagMuxer
	audioMuxer     flv.AVTagMuxer
	onframe        OnFrame
	output         OutputCB
	onRelease      OnReleaseStream
	onChangeState  OnStateChange
	onPlay         OnPlay
	onPublish      OnPublish
	timestamp      uint32
	streamId       uint32
}

func NewRtmpServerHandle(options ...func(*RtmpServerHandle)) *RtmpServerHandle {
	server := &RtmpServerHandle{
		hs:             newServerHandShake(),
		cmdChan:        newChunkStreamWriter(ChunkChannelCmd),
		userCtrlChan:   newChunkStreamWriter(ChunkChannelUseCtrl),
		reader:         newChunkStreamReader(FixChunkSize),
		wndAckSize:     DefaultAckSize,
		writeChunkSize: DefaultChunkSize,
		streamId:       1,
	}

	for _, o := range options {
		o(server)
	}

	return server
}

func (server *RtmpServerHandle) SetOutput(output OutputCB) {
	server.output = output
	server.hs.output = output
}

func (server *RtmpServerHandle) OnFrame(onframe OnFrame) {
	server.onframe = onframe
}

func (server *RtmpServerHandle) OnPlay(onPlay OnPlay) {
	server.onPlay = onPlay
}

func (server *RtmpServerHandle) OnPublish(onPub OnPublish) {
	server.onPublish = onPub
}

func (server *RtmpServerHandle) OnRelease(onRelease OnReleaseStream) {
	server.onRelease = onRelease
}

// 状态变更，回调函数，
// 服务端在STATE_RTMP_PLAY_START状态下，开始发流
// 客户端在STATE_RTMP_PUBLISH_START状态，开始推流
func (server *RtmpServerHandle) OnStateChange(stateChange OnStateChange) {
	server.onChangeState = stateChange
}

func (server *RtmpServerHandle) GetStreamName() string {
	return server.streamName
}

func (server *RtmpServerHandle) GetApp() string {
	return server.app
}

func (server *RtmpServerHandle) GetState() RtmpState {
	return server.streamState
}

func (server *RtmpServerHandle) Input(data []byte) error {
	for len(data) > 0 {
		switch server.state {
		case HandShake:
			server.changeState(StateHandshakeing)
			r := server.hs.input(data)
			if server.hs.getState() == HandshakeDone {
				server.changeState(StateHandshakeDone)
				server.state = ReadChunk
			}
			data = data[r:]
		case ReadChunk:

			err := server.reader.readRtmpMessage(data, func(msg *rtmpMessage) error {
				server.timestamp = msg.timestamp
				return server.handleMessage(msg)
			})
			return err
		}
	}
	return nil
}

func (server *RtmpServerHandle) WriteFrame(cid codec.CodecID, frame []byte, pts, dts uint32) error {
	if cid == codec.CodecidAudioAac || cid == codec.CodecidAudioG711a || cid == codec.CodecidAudioG711u {
		return server.WriteAudio(cid, frame, pts, dts)
	} else if cid == codec.CodecidVideoH264 || cid == codec.CodecidVideoH265 {
		return server.WriteVideo(cid, frame, pts, dts)
	} else {
		return errors.New("unsupport codec id")
	}
}

func (server *RtmpServerHandle) WriteAudio(cid codec.CodecID, frame []byte, pts, dts uint32) error {

	if server.audioMuxer == nil {
		server.audioMuxer = flv.CreateAudioMuxer(flv.CovertCodecId2SoundFromat(cid))
	}
	if server.audioChan == nil {
		server.audioChan = newChunkStreamWriter(ChunkChannelAudio)
		server.audioChan.chunkSize = server.writeChunkSize
	}
	tags := server.audioMuxer.Write(frame, pts, dts)
	for _, tag := range tags {
		pkt := server.audioChan.writeData(tag, AUDIO, server.streamId, dts)
		if len(pkt) > 0 {
			if err := server.output(pkt); err != nil {
				return err
			}
		}
	}
	return nil
}

func (server *RtmpServerHandle) WriteVideo(cid codec.CodecID, frame []byte, pts, dts uint32) error {
	if server.videoMuxer == nil {
		server.videoMuxer = flv.CreateVideoMuxer(flv.CovertCodecId2FlvVideoCodecId(cid))
	}
	if server.videoChan == nil {
		server.videoChan = newChunkStreamWriter(ChunkChannelVideo)
		server.videoChan.chunkSize = server.writeChunkSize
	}
	tags := server.videoMuxer.Write(frame, pts, dts)
	for _, tag := range tags {
		pkt := server.videoChan.writeData(tag, VIDEO, server.streamId, dts)
		if len(pkt) > 0 {
			if err := server.output(pkt); err != nil {
				return err
			}
		}
	}
	return nil
}

func (server *RtmpServerHandle) changeState(newState RtmpState) {
	if server.streamState != newState {
		server.streamState = newState
		if server.onChangeState != nil {
			server.onChangeState(newState)
		}
	}
}

func (server *RtmpServerHandle) handleMessage(msg *rtmpMessage) error {
	switch msg.msgtype {
	case SetChunkSize:
		if len(msg.msg) < 4 {
			return errors.New("bytes of \"set chunk size\"  < 4")
		}
		size := binary.BigEndian.Uint32(msg.msg)
		server.reader.chunkSize = size
	case AbortMessage:
		//TODO
	case ACKNOWLEDGEMENT:
		if len(msg.msg) < 4 {
			return errors.New("bytes of \"window acknowledgement size\"  < 4")
		}
		server.peerWndAckSize = binary.BigEndian.Uint32(msg.msg)
	case UserControl:
		//TODO
	case WndAckSize:
		//TODO
	case SetPeerBw:
		//TODO
	case AUDIO:
		return server.handleAudioMessage(msg)
	case VIDEO:
		return server.handleVideoMessage(msg)
	case CommandAmf0:
		return server.handleCommand(msg.msg)
	case CommandAmf3:
	case MetadataAmf0:
	case MetadataAmf3:
	case SharedobjectAmf0:
	case SharedobjectAmf3:
	case Aggregate:
	default:
		return errors.New("unkown message type")
	}
	return nil
}

func (server *RtmpServerHandle) handleCommand(data []byte) error {
	item := amf0Item{}
	l := item.decode(data)
	data = data[l:]
	cmd := string(item.value.([]byte))
	switch cmd {
	case "connect":
		server.changeState(StateRtmpConnecting)
		return server.handleConnect(data)
	case "releaseStream":
		server.handleReleaseStream(data)
	case "FCPublish":
	case "createStream":
		return server.handleCreateStream(data)
	case "play":
		return server.handlePlay(data)
	case "publish":
		return server.handlePublish(data)
	default:
	}
	return nil
}

func (server *RtmpServerHandle) handleConnect(data []byte) error {
	_, objs := decodeAmf0(data)
	if len(objs) > 0 {
		for _, item := range objs[0].items {
			if item.name == "app" {
				server.app = string(item.value.value.([]byte))
			} else if item.name == "tcUrl" {
				server.tcUrl = string(item.value.value.([]byte))
			}
		}
	}

	buf := makeSetChunkSize(server.writeChunkSize)
	bufs := server.userCtrlChan.writeData(buf, SetChunkSize, 0, 0)
	server.userCtrlChan.chunkSize = server.writeChunkSize
	server.cmdChan.chunkSize = server.writeChunkSize
	buf = makeAcknowledgementSize(server.wndAckSize)
	bufs = append(bufs, server.userCtrlChan.writeData(buf, WndAckSize, 0, 0)...)
	buf = makeSetPeerBandwidth(server.wndAckSize, LimittypeDynamic)
	bufs = append(bufs, server.userCtrlChan.writeData(buf, SetPeerBw, 0, 0)...)
	bufs = append(bufs, server.cmdChan.writeData(makeConnectRes(), CommandAmf0, 0, 0)...)
	return server.output(bufs)
}

func (server *RtmpServerHandle) handleReleaseStream(data []byte) {
	items, _ := decodeAmf0(data)
	if len(items) == 0 {
		return
	}
	streamName := string(items[len(items)-1].value.([]byte))
	if server.onRelease != nil {
		server.onRelease(server.app, streamName)
	}
}

func (server *RtmpServerHandle) handleCreateStream(data []byte) error {
	items, _ := decodeAmf0(data)
	if len(items) == 0 {
		return nil
	}
	tid := uint32(items[0].value.(float64))
	bufs := server.cmdChan.writeData(makeCreateStreamRes(tid, server.streamId), CommandAmf0, 0, 0)
	return server.output(bufs)
}

func (server *RtmpServerHandle) handlePlay(data []byte) error {
	items, _ := decodeAmf0(data)
	tid := int(items[0].value.(float64))
	streamName := string(items[2].value.([]byte))
	server.streamName = streamName
	start := float64(-2)
	duration := float64(-1)
	reset := false

	if len(items) > 3 {
		start = items[3].value.(float64)
	}
	if len(items) > 4 {
		duration = items[4].value.(float64)
	}

	if len(items) > 5 {
		reset = items[5].value.(bool)
	}

	code := NetstreamPlayStart
	if server.onPlay != nil {
		code = server.onPlay(server.app, streamName, start, duration, reset)
	}
	if code == NetstreamPlayStart {
		res := makeUserControlMessage(StreamBegin, int(server.streamId))
		bufs := server.userCtrlChan.writeData(res, UserControl, 0, 0)
		res = makeStatusRes(tid, NetstreamPlayReset, NetstreamPlayReset.Level(), string(NetstreamPlayReset.Description()))
		bufs = append(bufs, server.cmdChan.writeData(res, CommandAmf0, server.streamId, 0)...)
		res = makeStatusRes(tid, NetstreamPlayStart, NetstreamPlayStart.Level(), string(NetstreamPlayStart.Description()))
		bufs = append(bufs, server.cmdChan.writeData(res, CommandAmf0, server.streamId, 0)...)
		if err := server.output(bufs); err != nil {
			return err
		}
		server.changeState(StateRtmpPlayStart)
	} else {
		res := makeStatusRes(tid, code, code.Level(), string(code.Description()))
		if err := server.output(server.cmdChan.writeData(res, CommandAmf0, server.streamId, 0)); err != nil {
			return err
		}
		server.changeState(StateRtmpPlayFailed)
	}
	return nil
}

func (server *RtmpServerHandle) handlePublish(data []byte) error {
	items, _ := decodeAmf0(data)
	tid := int(items[0].value.(float64))
	streamName := string(items[2].value.([]byte))
	server.streamName = streamName
	code := NetstreamPublishStart
	if server.onPublish != nil {
		code = server.onPublish(server.app, streamName)
	}
	res := makeStatusRes(tid, code, code.Level(), string(code.Description()))
	if err := server.output(server.cmdChan.writeData(res, CommandAmf0, server.streamId, 0)); err != nil {
		return err
	}
	if code == NetstreamPublishStart {
		server.changeState(StateRtmpPublishStart)
	} else {
		server.changeState(StateRtmpPublishFailed)
	}
	return nil
}

func (server *RtmpServerHandle) handleVideoMessage(msg *rtmpMessage) error {
	if server.videoDemuxer == nil {
		server.videoDemuxer = flv.CreateFlvVideoTagHandle(flv.GetFLVVideoCodecId(msg.msg))
		server.videoDemuxer.OnFrame(func(codecid codec.CodecID, frame []byte, cts int) {
			dts := server.timestamp
			pts := dts + uint32(cts)
			server.onframe(codecid, pts, dts, frame)
		})
	}
	return server.videoDemuxer.Decode(msg.msg)
}

func (server *RtmpServerHandle) handleAudioMessage(msg *rtmpMessage) error {
	if server.audioDemuxer == nil {
		server.audioDemuxer = flv.CreateAudioTagDemuxer(flv.FlvSoundFormat((msg.msg[0] >> 4) & 0x0F))
		server.audioDemuxer.OnFrame(func(codecid codec.CodecID, frame []byte) {
			dts := server.timestamp
			pts := dts
			server.onframe(codecid, pts, dts, frame)
		})
	}
	return server.audioDemuxer.Decode(msg.msg)
}
