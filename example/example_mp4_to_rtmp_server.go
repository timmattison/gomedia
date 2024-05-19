package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/timmattison/gomedia/go-codec"
	"github.com/timmattison/gomedia/go-mp4"
	"github.com/timmattison/gomedia/go-rtmp"
)

type TimestampAdjust struct {
	lastTimeStamp   int64
	adjustTimestamp int64
}

func newTimestampAdjust() *TimestampAdjust {
	return &TimestampAdjust{
		lastTimeStamp:   -1,
		adjustTimestamp: 0,
	}
}

// timestamp in millisecond
func (adjust *TimestampAdjust) adjust(timestamp int64) int64 {
	if adjust.lastTimeStamp == -1 {
		adjust.adjustTimestamp = timestamp
		adjust.lastTimeStamp = timestamp
		return adjust.adjustTimestamp
	}

	delta := timestamp - adjust.lastTimeStamp
	if delta < -1000 || delta > 1000 {
		adjust.adjustTimestamp = adjust.adjustTimestamp + 1
	} else {
		adjust.adjustTimestamp = adjust.adjustTimestamp + delta
	}
	adjust.lastTimeStamp = timestamp
	return adjust.adjustTimestamp
}

var videoPtsAdjust = newTimestampAdjust()
var videoDtsAdjust = newTimestampAdjust()
var audioTsAdjust = newTimestampAdjust()

// Will push the last file under mp4sPath to the specified rtmp server
func main() {
	var (
		mp4Path = "your_mp4_dir" //like ./mp4/
		rtmpUrl = "rtmpUrl"      //like rtmp://127.0.0.1:1935/live/test110
	)
	c, err := net.Dial("tcp4", "${rtmp_host}:${rtmp_port}") // like 127.0.0.1:1935
	if err != nil {
		fmt.Println(err)
	}
	cli := rtmp.NewRtmpClient(rtmp.WithComplexHandshake(),
		rtmp.WithComplexHandshakeSchema(rtmp.HandshakeComplexSchema0),
		rtmp.WithEnablePublish())
	cli.OnError(func(code, describe string) {
		fmt.Printf("rtmp code:%s ,describe:%s\n", code, describe)
	})
	isReady := make(chan struct{})
	cli.OnStatus(func(code, level, describe string) {
		fmt.Printf("rtmp onstatus code:%s ,level %s ,describe:%s\n", code, describe)
	})
	cli.OnStateChange(func(newState rtmp.RtmpState) {
		if newState == rtmp.StateRtmpPublishStart {
			fmt.Println("ready for publish")
			close(isReady)
		}
	})
	cli.SetOutput(func(bytes []byte) error {
		_, err := c.Write(bytes)
		return err
	})
	go func() {
		<-isReady
		fmt.Println("start to read file")
		for {
			filees, err := os.ReadDir(mp4Path)
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println(mp4Path + filees[len(filees)-1].Name())
			PushRtmp(mp4Path+filees[len(filees)-1].Name(), cli)
		}
	}()

	cli.Start(rtmpUrl)
	buf := make([]byte, 4096)
	n := 0
	for err == nil {
		n, err = c.Read(buf)
		if err != nil {
			continue
		}
		fmt.Println("read byte", n)
		cli.Input(buf[:n])
	}
	fmt.Println(err)
}

func PushRtmp(fileName string, cli *rtmp.RtmpClient) {
	mp4File, err := os.Open(fileName)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer mp4File.Close()
	demuxer := mp4.CreateMp4Demuxer(mp4File)
	if infos, err := demuxer.ReadHead(); err != nil && err != io.EOF {
		fmt.Println(err)
	} else {
		fmt.Printf("%+v\n", infos)
	}
	mp4info := demuxer.GetMp4Info()
	fmt.Printf("%+v\n", mp4info)

	for {
		pkg, err := demuxer.ReadPacket()
		if err != nil {
			fmt.Println(err)
			break
		}
		if pkg.Cid == mp4.Mp4CodecH264 {
			time.Sleep(20 * time.Millisecond)
			pts := videoPtsAdjust.adjust(int64(pkg.Pts))
			dts := videoDtsAdjust.adjust(int64(pkg.Dts))
			cli.WriteVideo(codec.CodecidVideoH264, pkg.Data, uint32(pts), uint32(dts))
		} else if pkg.Cid == mp4.Mp4CodecAac {
			pts := audioTsAdjust.adjust(int64(pkg.Pts))
			cli.WriteAudio(codec.CodecidAudioAac, pkg.Data, uint32(pts), uint32(pts))
		} else if pkg.Cid == mp4.Mp4CodecMp3 {
			pts := audioTsAdjust.adjust(int64(pkg.Pts))
			cli.WriteAudio(codec.CodecidAudioMp3, pkg.Data, uint32(pts), uint32(pts))
		}

	}
	return
}
