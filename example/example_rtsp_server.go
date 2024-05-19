package main

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/timmattison/gomedia/go-codec"
	"github.com/timmattison/gomedia/go-rtsp"
	"github.com/timmattison/gomedia/go-rtsp/sdp"
)

type RtspSourceManager struct {
	mtx     sync.Mutex
	sources map[string]*StreamSource
}

var gManager *RtspSourceManager

func init() {
	gManager = &RtspSourceManager{}
	gManager.sources = make(map[string]*StreamSource)
	fmt.Println("int g_manager", gManager)
}

func (manager *RtspSourceManager) getSource(name string) (*StreamSource, bool) {
	manager.mtx.Lock()
	defer manager.mtx.Unlock()
	s, found := manager.sources[name]
	return s, found
}

func (manager *RtspSourceManager) addSource(name string, source *StreamSource) {
	manager.mtx.Lock()
	defer manager.mtx.Unlock()
	manager.sources[name] = source
}

func (manager *RtspSourceManager) removeSource(name string) {
	manager.mtx.Lock()
	defer manager.mtx.Unlock()
	delete(manager.sources, name)
}

type VideoConfig struct {
	cid codec.CodecID
	sps []byte
	pps []byte
	vps []byte
}

type AudioConfig struct {
	cid          codec.CodecID
	asc          []byte
	sampleRate   int
	channalCount int
}

type StreamSource struct {
	streamName string
	producer   *RtspServerSession
	mtx        sync.Mutex
	consumers  []*RtspServerSession
	audioCfg   *AudioConfig
	videoCfg   *VideoConfig
}

func (s *StreamSource) addConsumer(sess *RtspServerSession) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.consumers = append(s.consumers, sess)
}

func (s *StreamSource) dispatch() {
	for {
		select {
		case frame := <-s.producer.readChan:
			if frame.frameType == 0 {
				//	fmt.Println("video ts", frame.ts)
				if s.videoCfg == nil {
					fmt.Println("add video config")
					s.videoCfg = &VideoConfig{}
				}
				if frame.cid == rtsp.RtspCodecH264 && (len(s.videoCfg.sps) == 0 || len(s.videoCfg.pps) == 0) {
					s.videoCfg.cid = codec.CodecidVideoH264
					codec.SplitFrame(frame.frame, func(nalu []byte) bool {
						naluType := codec.H264NaluTypeWithoutStartCode(nalu)
						switch naluType {
						case codec.H264NalSps:
							s.videoCfg.sps = make([]byte, len(nalu))
							copy(s.videoCfg.sps, nalu)
						case codec.H264NalPps:
							s.videoCfg.pps = make([]byte, len(nalu))
							copy(s.videoCfg.pps, nalu)
						}
						return true
					})
				} else if frame.cid == rtsp.RtspCodecH265 && (len(s.videoCfg.sps) == 0 || len(s.videoCfg.pps) == 0 || len(s.videoCfg.vps) == 0) {
					s.videoCfg.cid = codec.CodecidVideoH265
					codec.SplitFrame(frame.frame, func(nalu []byte) bool {
						naluType := codec.H265NaluTypeWithoutStartCode(nalu)
						switch naluType {
						case codec.H265NalPps:
							fmt.Println("got pps", len(nalu))
							s.videoCfg.pps = make([]byte, len(nalu))
							copy(s.videoCfg.pps, nalu)
						case codec.H265NalSps:
							fmt.Println("got sps")
							s.videoCfg.sps = make([]byte, len(nalu))
							copy(s.videoCfg.sps, nalu)
						case codec.H265NalVps:
							fmt.Println("got vps")
							s.videoCfg.vps = make([]byte, len(nalu))
							copy(s.videoCfg.vps, nalu)
						}
						return true
					})
				}
			} else {
				if s.audioCfg == nil {
					s.audioCfg = &AudioConfig{}
					fmt.Println("add audio config")
				}
				if frame.cid == rtsp.RtspCodecAac && len(s.audioCfg.asc) == 0 {
					s.audioCfg.cid = codec.CodecidAudioAac
					asc, _ := codec.ConvertADTSToASC(frame.frame)
					s.audioCfg.sampleRate = codec.AACSampleIdxToSample(int(asc.SampleFreqIndex))
					s.audioCfg.channalCount = int(asc.ChannelConfiguration)
					s.audioCfg.asc = asc.Encode()
				}
			}

			for _, c := range s.consumers {
				c.SendFrame(frame)
			}

		}
	}
}

type RtspFrame struct {
	frameType int //0 - video , 1 - audio
	keyFrame  int
	cid       rtsp.RtspCodecId
	frame     []byte
	ts        uint32
}

type RtspServerSession struct {
	readChan   chan *RtspFrame
	c          net.Conn
	tracks     map[string]*rtsp.RtspTrack
	isProducer bool
	name       string
}

func NewRtspServerSession(c net.Conn) *RtspServerSession {
	return &RtspServerSession{
		c:        c,
		readChan: make(chan *RtspFrame, 100),
		tracks:   make(map[string]*rtsp.RtspTrack),
	}
}

func (sess *RtspServerSession) Start() {
	svr := rtsp.NewRtspServer(&ServerHandleImpl{sess: sess}, rtsp.WithUserInfo("test", "test123"))
	svr.SetOutput(func(b []byte) (err error) {
		_, err = sess.c.Write(b)
		return
	})
	buf := make([]byte, 65535)
	for {
		n, err := sess.c.Read(buf)
		if err != nil {
			fmt.Println(err)
			break
		}
		svr.Input(buf[:n])
	}
	if sess.isProducer {
		gManager.removeSource(sess.name)
	}
	sess.c.Close()
}

func (sess *RtspServerSession) SendFrame(frame *RtspFrame) {
	switch frame.frameType {
	case 0:
		sess.tracks["video"].WriteSample(rtsp.RtspSample{
			Sample:    frame.frame,
			Timestamp: frame.ts,
		})
	case 1:
		sess.tracks["audio"].WriteSample(rtsp.RtspSample{
			Sample:    frame.frame[7:],
			Timestamp: frame.ts,
		})
	}
}

type ServerHandleImpl struct {
	sess *RtspServerSession
}

func (impl *ServerHandleImpl) HandleOption(svr *rtsp.RtspServer, req rtsp.RtspRequest, res *rtsp.RtspResponse) {
	fmt.Println("handle option")
}

func (impl *ServerHandleImpl) HandleDescribe(svr *rtsp.RtspServer, req rtsp.RtspRequest, res *rtsp.RtspResponse) {
	fmt.Println("handle describe")
	streamName := req.Uri[strings.LastIndex(req.Uri, "/")+1:]
	source, found := gManager.getSource(streamName)
	if !found {
		res.StatusCode = rtsp.NotFound
		return
	}

	if source.audioCfg != nil {
		if source.audioCfg.cid == codec.CodecidAudioAac {
			fmt.Println("add audio track", source.audioCfg.sampleRate)
			audioCodec := rtsp.RtspCodec{
				Cid:          rtsp.RtspCodecAac,
				PayloadType:  97,
				SampleRate:   uint32(source.audioCfg.sampleRate),
				ChannelCount: uint8(source.audioCfg.channalCount),
			}
			audioTrack := rtsp.NewAudioTrack(audioCodec, rtsp.WithCodecParamHandler(sdp.NewAACFmtpParam(sdp.WithAudioSpecificConfig(source.audioCfg.asc))))
			svr.AddTrack(audioTrack)
			impl.sess.tracks["audio"] = audioTrack
		}
	}

	if source.videoCfg != nil {
		if source.videoCfg.cid == codec.CodecidVideoH264 {
			fmt.Println("add video track")
			fmtpHandle := sdp.NewH264FmtpParam(sdp.WithH264SPS(source.videoCfg.sps), sdp.WithH264PPS(source.videoCfg.pps))
			videoTrack := rtsp.NewVideoTrack(rtsp.RtspCodec{Cid: rtsp.RtspCodecH264, PayloadType: 96, SampleRate: 90000}, rtsp.WithCodecParamHandler(fmtpHandle))
			svr.AddTrack(videoTrack)
			impl.sess.tracks["video"] = videoTrack
		} else if source.videoCfg.cid == codec.CodecidVideoH265 {
			fmt.Println("add video track")
			fmtpHandle := sdp.NewH265FmtpParam(sdp.WithH265SPS(source.videoCfg.sps), sdp.WithH265PPS(source.videoCfg.pps), sdp.WithH265VPS(source.videoCfg.vps))
			videoTrack := rtsp.NewVideoTrack(rtsp.RtspCodec{Cid: rtsp.RtspCodecH265, PayloadType: 98, SampleRate: 90000}, rtsp.WithCodecParamHandler(fmtpHandle))
			svr.AddTrack(videoTrack)
			impl.sess.tracks["video"] = videoTrack
		}
	}
}

func (impl *ServerHandleImpl) HandleSetup(svr *rtsp.RtspServer, req rtsp.RtspRequest, res *rtsp.RtspResponse, transport *rtsp.RtspTransport, tracks *rtsp.RtspTrack) {
	fmt.Println("handle setup", *transport)
	if transport.Proto == rtsp.UDP {
		res.StatusCode = rtsp.UnsupportedTransport
		return
	}
}

func (impl *ServerHandleImpl) HandleAnnounce(svr *rtsp.RtspServer, req rtsp.RtspRequest, tracks map[string]*rtsp.RtspTrack) {
	fmt.Println("handle announce")
	streamName := req.Uri[strings.LastIndex(req.Uri, "/")+1:]
	fmt.Println("stream name ", streamName)
	source := &StreamSource{}
	fmt.Println(gManager)
	go source.dispatch()
	gManager.addSource(streamName, source)
	source.producer = impl.sess
	impl.sess.name = streamName
	impl.sess.isProducer = true
	if atrack, found := tracks["audio"]; found {
		afile, err := os.OpenFile("rtsp.aac", os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			fmt.Println(err)
			return
		}
		atrack.OnSample(func(sample rtsp.RtspSample) {
			frame := &RtspFrame{
				frameType: 1,
				keyFrame:  1,
				cid:       sample.Cid,
				frame:     sample.Sample,
				ts:        sample.Timestamp,
			}
			source.producer.readChan <- frame
			afile.Write(frame.frame)
		})
	}

	if vtrack, found := tracks["video"]; found {
		vfile, err := os.OpenFile("rtsp.h265", os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			fmt.Println(err)
			return
		}
		vtrack.OnSample(func(sample rtsp.RtspSample) {
			frame := &RtspFrame{
				frameType: 0,
				cid:       sample.Cid,
				frame:     make([]byte, len(sample.Sample)),
				ts:        sample.Timestamp,
			}
			//fmt.Println("onsample", frame.ts)
			copy(frame.frame, sample.Sample)
			if sample.Cid == rtsp.RtspCodecH264 {
				if codec.H264NaluType(frame.frame) == codec.H264NalISlice {
					frame.keyFrame = 1
				}
			} else if sample.Cid == rtsp.RtspCodecH265 {
				if codec.H265NaluType(frame.frame) >= codec.H265NalSliceBlaWLp || codec.H265NaluType(frame.frame) <= codec.H265NalSliceCra {
					frame.keyFrame = 1
				}
			}
			vfile.Write(frame.frame)
			source.producer.readChan <- frame

		})
	}
}

func (impl *ServerHandleImpl) HandlePlay(svr *rtsp.RtspServer, req rtsp.RtspRequest, res *rtsp.RtspResponse, timeRange *rtsp.RangeTime, info []*rtsp.RtpInfo) {
	fmt.Println("handle play")
	streamName := req.Uri[strings.LastIndex(req.Uri, "/")+1:]
	source, found := gManager.getSource(streamName)
	if !found {
		res.StatusCode = rtsp.NotFound
		return
	}
	source.addConsumer(impl.sess)
}

func (impl *ServerHandleImpl) HandlePause(svr *rtsp.RtspServer, req rtsp.RtspRequest, res *rtsp.RtspResponse) {

}

func (impl *ServerHandleImpl) HandleTeardown(svr *rtsp.RtspServer, req rtsp.RtspRequest, res *rtsp.RtspResponse) {

}

func (impl *ServerHandleImpl) HandleGetParameter(svr *rtsp.RtspServer, req rtsp.RtspRequest, res *rtsp.RtspResponse) {

}

func (impl *ServerHandleImpl) HandleSetParameter(svr *rtsp.RtspServer, req rtsp.RtspRequest, res *rtsp.RtspResponse) {

}

func (impl *ServerHandleImpl) HandleRecord(svr *rtsp.RtspServer, req rtsp.RtspRequest, res *rtsp.RtspResponse, timeRange *rtsp.RangeTime, info []*rtsp.RtpInfo) {

}

func (impl *ServerHandleImpl) HandleResponse(svr *rtsp.RtspServer, res rtsp.RtspResponse) {

}

func main() {
	addr := "0.0.0.0:554"
	listen, err := net.Listen("tcp4", addr)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer listen.Close()
	for {
		conn, _ := listen.Accept()
		sess := NewRtspServerSession(conn)
		go sess.Start()
	}
}
