package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/timmattison/gomedia/go-codec"
	"github.com/timmattison/gomedia/go-flv"
)

const (
	HTTPOK = 200
)

// .example_http_flv_client -i url -o out.flv
func main() {
	flvurl, flvfilename := parseFlag()
	flvfile, err := os.Create(flvfilename)
	if err != nil {
		fmt.Println("create flv file failed, err:", err)
		return
	}

	defer flvfile.Close()

	fr := flv.CreateFlvReader()
	fr.OnFrame = func(ci codec.CodecID, b []byte, pts, dts uint32) {
		fmt.Println("codec:", codec.CodecString(ci), " pts:", pts, " dts:", dts)
	}

	resp, err := http.Get(flvurl)
	if err != nil {
		fmt.Println("http get failed, err:", err)
		return
	}

	if resp.StatusCode != HTTPOK {
		resp.Body.Close()
		return
	}

	buf := make([]byte, 4*1024)
	for {
		n, err := resp.Body.Read(buf)
		if err != nil {
			fmt.Println("read failed, err:", err)
			break
		}

		if n == 0 {
			fmt.Println("read n=0")
			break
		}

		if n > 0 {
			flvfile.Write(buf[0:n])
			fr.Input(buf[0:n])
		}
	}

	resp.Body.Close()

	fmt.Println("http flv client end")
}

func parseFlag() (url, flvfile string) {
	i := flag.String("i", "", "specify http-flv url")
	o := flag.String("o", "", "specify output flv file")
	flag.Parse()
	if *i == "" || *o == "" {
		flag.Usage()
		os.Exit(1)
	}
	return *i, *o
}
