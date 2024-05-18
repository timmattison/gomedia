package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/timmattison/gomedia/go-rtsp"
	"github.com/timmattison/gomedia/go-rtsp/sdp"
)

type UdpPairSession struct {
	rtpSess  *net.UDPConn
	rtcpSess *net.UDPConn
}

func makeUdpPairSession(localRtpPort, localRtcpPort uint16, remoteAddr string, remoteRtpPort, remoteRtcpPort uint16) *UdpPairSession {
	srcAddr := net.UDPAddr{IP: net.IPv4zero, Port: int(localRtpPort)}
	srcAddr2 := net.UDPAddr{IP: net.IPv4zero, Port: int(localRtcpPort)}
	dstAddr := net.UDPAddr{IP: net.ParseIP(remoteAddr), Port: int(remoteRtpPort)}
	dstAddr2 := net.UDPAddr{IP: net.ParseIP(remoteAddr), Port: int(remoteRtcpPort)}
	rtpUdpsess, _ := net.DialUDP("udp4", &srcAddr, &dstAddr)
	rtcpUdpsess, _ := net.DialUDP("udp4", &srcAddr2, &dstAddr2)
	return &UdpPairSession{
		rtpSess:  rtpUdpsess,
		rtcpSess: rtcpUdpsess,
	}
}

type RtspUdpPlaySession struct {
	udpport    uint16
	videoFile  *os.File
	audioFile  *os.File
	tsFile     *os.File
	timeout    int
	once       sync.Once
	die        chan struct{}
	c          net.Conn
	lastError  error
	sesss      map[string]*UdpPairSession
	remoteAddr string
}

func NewRtspUdpPlaySession(c net.Conn) *RtspUdpPlaySession {
	return &RtspUdpPlaySession{udpport: 30000, die: make(chan struct{}), c: c, sesss: make(map[string]*UdpPairSession)}
}

func (cli *RtspUdpPlaySession) Destory() {
	cli.once.Do(func() {
		if cli.videoFile != nil {
			cli.videoFile.Close()
		}
		if cli.audioFile != nil {
			cli.audioFile.Close()
		}
		if cli.tsFile != nil {
			cli.tsFile.Close()
		}
		cli.c.Close()
		close(cli.die)
	})
}

func (cli *RtspUdpPlaySession) HandleOption(client *rtsp.RtspClient, res rtsp.RtspResponse, public []string) error {
	fmt.Println("rtsp server public ", public)
	return nil
}

func (cli *RtspUdpPlaySession) HandleDescribe(client *rtsp.RtspClient, res rtsp.RtspResponse, sdp *sdp.Sdp, tracks map[string]*rtsp.RtspTrack) error {
	fmt.Println("handle describe ", res.StatusCode, res.Reason)
	for k, t := range tracks {
		if t == nil {
			continue
		}
		fmt.Println("Got ", k, " track")
		transport := rtsp.NewRtspTransport(rtsp.WithEnableUdp(), rtsp.WithClientUdpPort(cli.udpport, cli.udpport+1), rtsp.WithMode(rtsp.MODE_PLAY))
		t.SetTransport(transport)
		t.OpenTrack()
		cli.udpport += 2
		if t.Codec.Cid == rtsp.RTSP_CODEC_H264 {
			if cli.videoFile == nil {
				cli.videoFile, _ = os.OpenFile("video.h264", os.O_CREATE|os.O_RDWR, 0666)
			}
			t.OnSample(func(sample rtsp.RtspSample) {
				fmt.Println("Got H264 Frame size:", len(sample.Sample), " timestamp:", sample.Timestamp)
				cli.videoFile.Write(sample.Sample)
			})
		} else if t.Codec.Cid == rtsp.RTSP_CODEC_AAC {
			if cli.audioFile == nil {
				cli.audioFile, _ = os.OpenFile("audio.aac", os.O_CREATE|os.O_RDWR, 0666)
			}
			t.OnSample(func(sample rtsp.RtspSample) {
				fmt.Println("Got AAC Frame size:", len(sample.Sample), " timestamp:", sample.Timestamp)
				cli.audioFile.Write(sample.Sample)
			})
		} else if t.Codec.Cid == rtsp.RTSP_CODEC_TS {
			if cli.tsFile == nil {
				cli.tsFile, _ = os.OpenFile("mp2t.ts", os.O_CREATE|os.O_RDWR, 0666)
			}
			t.OnSample(func(sample rtsp.RtspSample) {
				cli.tsFile.Write(sample.Sample)
			})
		}
	}
	return nil
}

func (cli *RtspUdpPlaySession) HandleSetup(client *rtsp.RtspClient, res rtsp.RtspResponse, track *rtsp.RtspTrack, tracks map[string]*rtsp.RtspTrack, sessionId string, timeout int) error {
	fmt.Println("HandleSetup sessionid:", sessionId, " timeout:", timeout)
	if res.StatusCode == rtsp.Unsupported_Transport {
		return errors.New("unsupport udp transport")
	}
	ip, _, _ := net.SplitHostPort(cli.c.RemoteAddr().String())
	cli.sesss[track.TrackName] = makeUdpPairSession(track.GetTransport().Client_ports[0], track.GetTransport().Client_ports[1], ip, track.GetTransport().Server_ports[0], track.GetTransport().Server_ports[1])
	track.OnPacket(func(b []byte, isRtcp bool) (err error) {
		if isRtcp {
			_, err = cli.sesss[track.TrackName].rtcpSess.Write(b)
		}
		return
	})
	go func() {
		buf := make([]byte, 1500)
		for {
			r, err := cli.sesss[track.TrackName].rtpSess.Read(buf)
			if err != nil {
				fmt.Println(err)
				break
			}
			//fmt.Println("read rtp")
			err = track.Input(buf[:r], false)
			if err != nil {
				fmt.Println(err)
				break
			}
		}
		cli.Destory()
	}()

	go func() {
		buf := make([]byte, 1500)
		for {
			r, err := cli.sesss[track.TrackName].rtcpSess.Read(buf)
			if err != nil {
				fmt.Println(err)
				break
			}
			fmt.Println("read rtcp")
			err = track.Input(buf[:r], true)
			if err != nil {
				fmt.Println(err)
				break
			}
		}
		cli.Destory()
	}()

	cli.timeout = timeout
	return nil
}

func (cli *RtspUdpPlaySession) HandleAnnounce(client *rtsp.RtspClient, res rtsp.RtspResponse) error {
	return nil
}

func (cli *RtspUdpPlaySession) HandlePlay(client *rtsp.RtspClient, res rtsp.RtspResponse, timeRange *rtsp.RangeTime, info *rtsp.RtpInfo) error {
	if res.StatusCode != 200 {
		fmt.Println("play failed ", res.StatusCode, res.Reason)
		return nil
	}
	go func() {
		//rtsp keepalive
		to := time.NewTicker(time.Duration(cli.timeout/2) * time.Second)
		defer to.Stop()
		for {
			select {
			case <-to.C:
				client.KeepAlive(rtsp.OPTIONS)
			case <-cli.die:
				return
			}
		}
	}()
	return nil
}

func (cli *RtspUdpPlaySession) HandlePause(client *rtsp.RtspClient, res rtsp.RtspResponse) error {
	return nil
}

func (cli *RtspUdpPlaySession) HandleTeardown(client *rtsp.RtspClient, res rtsp.RtspResponse) error {
	return nil
}

func (cli *RtspUdpPlaySession) HandleGetParameter(client *rtsp.RtspClient, res rtsp.RtspResponse) error {
	return nil
}

func (cli *RtspUdpPlaySession) HandleSetParameter(client *rtsp.RtspClient, res rtsp.RtspResponse) error {
	return nil
}

func (cli *RtspUdpPlaySession) HandleRedirect(client *rtsp.RtspClient, req rtsp.RtspRequest, location string, timeRange *rtsp.RangeTime) error {
	return nil
}

func (cli *RtspUdpPlaySession) HandleRecord(client *rtsp.RtspClient, res rtsp.RtspResponse, timeRange *rtsp.RangeTime, info *rtsp.RtpInfo) error {
	return nil
}

func (cli *RtspUdpPlaySession) HandleRequest(client *rtsp.RtspClient, req rtsp.RtspRequest) error {
	return nil
}

func (cli *RtspUdpPlaySession) sendInLoop(sendChan chan []byte) {
	for {
		select {
		case b := <-sendChan:
			_, err := cli.c.Write(b)
			if err != nil {
				cli.Destory()
				cli.lastError = err
				fmt.Println("quit send in loop")
				return
			}

		case <-cli.die:
			fmt.Println("quit send in loop")
			return
		}
	}
}

func main() {
	u, err := url.Parse(os.Args[1])
	if err != nil {
		panic(err)
	}
	host := u.Host
	if u.Port() == "" {
		host += ":554"
	}
	c, err := net.Dial("tcp4", host)
	if err != nil {
		fmt.Println(err)
		return
	}

	sc := make(chan []byte, 100)
	sess := NewRtspUdpPlaySession(c)
	go sess.sendInLoop(sc)
	client, _ := rtsp.NewRtspClient(os.Args[1], sess)
	client.SetOutput(func(b []byte) error {
		if sess.lastError != nil {
			return sess.lastError
		}
		sc <- b
		return nil
	})
	client.Start()
	buf := make([]byte, 4096)
	for {
		n, err := c.Read(buf)
		if err != nil {
			fmt.Println(err)
			break
		}
		if err = client.Input(buf[:n]); err != nil {
			fmt.Println(err)
			break
		}
	}
	sess.Destory()
}
