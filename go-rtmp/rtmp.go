package rtmp

import (
	"github.com/timmattison/gomedia/go-codec"
)

const (
	ChunkChannelUseCtrl   = 2
	ChunkChannelCmd       = 3
	ChunkChannelVideo     = 5
	ChunkChannelAudio     = 6
	ChunkChannelMeta      = 7
	ChunkChannelNetStream = 8
)

const (
	FixChunkSize     = 128
	DefaultChunkSize = 60000
	DefaultAckSize   = 5000000
)

const (
	HandshakeSize          = 1536
	HandshakeFixSize       = 8
	HandshakeOffsetSize    = 4
	HandshakeDigestSize    = 32
	HandshakeSchemaSize    = 764
	HandshakeSchema0Offset = 776 // 8 + 764 + 4
	HandshakeSchema1Offset = 12  // 8 + 4
)

const (
	HandshakeComplexSchema0 = 0
	HandshakeComplexSchema1 = 1
)

const (
	PublishingLive   = "live"
	PublishingRecord = "record"
	PublishingAppend = "append"
)

const (
	LimittypeHard    = 0
	LimittypeSoft    = 1
	LimittypeDynamic = 2
)

type RtmpParserState int

const (
	HandShake RtmpParserState = iota
	ReadChunk
)

type RtmpState int

const (
	StateHandshakeing RtmpState = iota
	StateHandshakeDone
	StateRtmpConnecting
	StateRtmpPlayStart
	StateRtmpPlayFailed
	StateRtmpPublishStart
	StateRtmpPublishFailed
)

//https://blog.csdn.net/wq892373445/article/details/118387494

type StatusLevel string

const (
	LevelStatus StatusLevel = "status"
	LevelError  StatusLevel = "error"
	LevelWarn   StatusLevel = "warning"
)

type StatusCode string

const (
	NetstreamPublishStart     StatusCode = "NetStream.Publish.Start"
	NetstreamPlayStart        StatusCode = "NetStream.Play.Start"
	NetstreamPlayStop         StatusCode = "NetStream.Play.Stop"
	NetstreamPlayFailed       StatusCode = "NetStream.Play.Failed"
	NetstreamPlayNotfound     StatusCode = "NetStream.Play.StreamNotFound"
	NetstreamPlayReset        StatusCode = "NetStream.Play.Reset"
	NetstreamPauseNotify      StatusCode = "NetStream.Pause.Notify"
	NetstreamUnpauseNotify    StatusCode = "NetStream.Unpause.Notify"
	NetstreamRecordStart      StatusCode = "NetStream.Record.Start"
	NetstreamRecordStop       StatusCode = "NetStream.Record.Stop"
	NetstreamRecordFailed     StatusCode = "NetStream.Record.Failed"
	NetstreamSeekFailed       StatusCode = "NetStream.Seek.Failed"
	NetstreamSeekNotify       StatusCode = "NetStream.Seek.Notify"
	NetconnectConnectClosed   StatusCode = "NetConnection.Connect.Closed"
	NetconnectConnectFailed   StatusCode = "NetConnection.Connect.Failed"
	NetconnectConnectSuccess  StatusCode = "NetConnection.Connect.Success"
	NetconnectConnectRejected StatusCode = "NetConnection.Connect.Rejected"
	NetstreamConnectClosed    StatusCode = "NetStream.Connect.Closed"
	NetstreamConnectFailed    StatusCode = "NetStream.Connect.Failed"
	NetstreamConnectSuccesss  StatusCode = "NetStream.Connect.Success"
	NetstreamConnectRejected  StatusCode = "NetStream.Connect.Rejected"
)

func (c StatusCode) Level() StatusLevel {
	switch c {
	case NetstreamPublishStart:
		return "status"
	case NetstreamPlayStart:
		return "status"
	case NetstreamPlayStop:
		return "status"
	case NetstreamPlayFailed:
		return "error"
	case NetstreamPlayNotfound:
		return "error"
	case NetstreamPlayReset:
		return "status"
	case NetstreamPauseNotify:
		return "status"
	case NetstreamUnpauseNotify:
		return "status"
	case NetstreamRecordStart:
		return "status"
	case NetstreamRecordStop:
		return "status"
	case NetstreamRecordFailed:
		return "error"
	case NetstreamSeekFailed:
		return "error"
	case NetstreamSeekNotify:
		return "status"
	case NetconnectConnectClosed:
		return "status"
	case NetconnectConnectFailed:
		return "error"
	case NetconnectConnectSuccess:
		return "status"
	case NetconnectConnectRejected:
		return "error"
	case NetstreamConnectClosed:
		return "status"
	case NetstreamConnectFailed:
		return "error"
	case NetstreamConnectSuccesss:
		return "status"
	case NetstreamConnectRejected:
		return "error"
	}
	return ""
}

func (c StatusCode) Description() StatusLevel {
	switch c {
	case NetstreamPublishStart:
		return "Start publishing stream"
	case NetstreamPlayStart:
		return "Start play stream "
	case NetstreamPlayStop:
		return "Stop play stream"
	case NetstreamPlayFailed:
		return "Play stream failed"
	case NetstreamPlayNotfound:
		return "Stream not found"
	case NetstreamPlayReset:
		return "Reset stream"
	case NetstreamPauseNotify:
		return "Pause stream"
	case NetstreamUnpauseNotify:
		return "Unpause stream"
	case NetstreamRecordStart:
		return "Start record stream"
	case NetstreamRecordStop:
		return "Stop record stream"
	case NetstreamRecordFailed:
		return "Record stream failed"
	case NetstreamSeekFailed:
		return "Seek stream failed"
	case NetstreamSeekNotify:
		return "Seek stream"
	case NetconnectConnectClosed:
		return "Close connection"
	case NetconnectConnectFailed:
		return "Connect failed"
	case NetconnectConnectSuccess:
		return "Connection succeeded"
	case NetconnectConnectRejected:
		return "Connection rejected"
	case NetstreamConnectClosed:
		return "Connection closed"
	case NetstreamConnectFailed:
		return "Connection failed"
	case NetstreamConnectSuccesss:
		return "Connect Stream suceessed"
	case NetstreamConnectRejected:
		return "Reject connect stram"
	}
	return ""
}

type OutputCB func([]byte) error
type OnFrame func(cid codec.CodecID, pts, dts uint32, frame []byte)
type OnStatus func(code, level, describe string)
type OnError func(code, describe string)
type OnReleaseStream func(app, streamName string)
type OnPlay func(app, streamName string, start, duration float64, reset bool) StatusCode
type OnPublish func(app, streamName string) StatusCode
type OnStateChange func(newState RtmpState)
