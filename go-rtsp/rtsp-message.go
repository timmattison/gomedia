package rtsp

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	Rtsp10 = 1
	Rtsp20 = 2
)

// method            direction        object     requirement
// DESCRIBE          C->S             P,S        recommended
// ANNOUNCE          C->S, S->C       P,S        optional
// GetParameter     C->S, S->C       P,S        optional
// OPTIONS           C->S, S->C       P,S        required
//
//	(S->C: optional)
//
// PAUSE             C->S             P,S        recommended
// PLAY              C->S             P,S        required
// RECORD            C->S             P,S        optional
// REDIRECT          S->C             P,S        optional
// SETUP             C->S             S          required
// SET_PARAMETER     C->S, S->C       P,S        optional
// TEARDOWN          C->S             P,S        required
const (
	OPTIONS       = "OPTIONS"
	DESCRIBE      = "DESCRIBE"
	SETUP         = "SETUP"
	PLAY          = "PLAY"
	GetParameter  = "GET_PARAMETER"
	SET_PARAMETER = "SET_PARAMETER"
	ANNOUNCE      = "ANNOUNCE"
	PAUSE         = "PAUSE"
	RECORD        = "RECORD"
	REDIRECT      = "REDIRECT"
	TEARDOWN      = "TEARDOWN"
)

func hasPlayAbility(capset []string) bool {
	score := 0
	for _, method := range capset {
		switch method {
		case SETUP:
			score++
		case DESCRIBE:
			score++
		case PLAY:
			score++
		case TEARDOWN:
			score++
		}
	}
	if score < 4 {
		return false
	} else {
		return true
	}
}

func hasRecordAbility(capset []string) bool {
	score := 0
	for _, method := range capset {
		switch method {
		case SETUP:
			score++
		case ANNOUNCE:
			score++
		case RECORD:
			score++
		case TEARDOWN:
			score++
		}
	}
	if score < 4 {
		return false
	} else {
		return true
	}
}

const (
	Accept            = "Accept"
	AcceptEncoding    = "AcceptEncoding"
	AcceptLanguage    = "AcceptLanguage"
	Allow             = "Allow"
	Authorization     = "Authorization"
	Bandwidth         = "Bandwidth"
	Blocksize         = "Blocksize"
	CacheControl      = "CacheControl"
	Conference        = "Conference"
	Connection        = "Connection"
	ContentBase       = "Content-Base"
	ContentEncoding   = "Content-Encoding"
	ContentLanguage   = "Content-Language"
	ContentLength     = "Content-Length"
	ContentLocation   = "Content-Location"
	ContentType       = "Content-Type"
	CSeq              = "CSeq"
	Date              = "Date"
	Expires           = "Expires"
	From              = "From"
	IfModifiedSince   = "IfModifiedSince"
	LastModified      = "LastModified"
	ProxyAuthenticate = "ProxyAuthenticate"
	ProxyRequire      = "ProxyRequire"
	Public            = "Public"
	Range             = "Range"
	Referer           = "Referer"
	Require           = "Require"
	RetryAfter        = "RetryAfter"
	RTPInfo           = "RTPInfo"
	Scale             = "Scale"
	Session           = "Session"
	Server            = "Server"
	Speed             = "Speed"
	Transport         = "Transport"
	Unsupported       = "Unsupported"
	UserAgent         = "UserAgent"
	Via               = "Via"
	WWWAuthenticate   = "WWW-Authenticate"
	Location          = "Location"
)

const (
	OK                   = 200
	MovedPermanently     = 300
	MovedTemporarily     = 301
	BadRequest           = 400
	Unauthorized         = 401
	NotFound             = 404
	SessionNotFound      = 454
	UnsupportedTransport = 461
	InternalServerError  = 500
	NotImplemented       = 501
	VersionNotSupported  = 505
)

var errNeedMore = errors.New("need more")

type HeadFiled map[string]string

func (filed HeadFiled) Add(key string, value interface{}) {
	switch v := value.(type) {
	case int:
		filed[key] = strconv.Itoa(v)
	case int32:
		filed[key] = strconv.FormatInt(int64(v), 10)
	case string:
		filed[key] = v
	}
}

func (filed HeadFiled) Has(key string) bool {
	_, found := filed[key]
	return found
}

type RtspRequest struct {
	Method  string
	Uri     string
	Version int
	Fileds  HeadFiled
	Body    string
}

func (req *RtspRequest) parse(data string) (int, error) {

	loc := strings.Index(data, "\r\n\r\n")
	if loc == -1 {
		return 0, errNeedMore
	}
	body := data[loc+4:]
	data = data[:loc]
	strs := strings.Split(data, "\r\n")
	if len(strs) <= 1 {
		return 0, errors.New("illegal rtsp request")
	}

	req.parseFirstLine(strs[0])
	for i := 1; i < len(strs); i++ {
		kv := strings.SplitN(strs[i], ":", 2)
		k := strings.Title(strings.TrimSpace(kv[0]))
		v := strings.TrimSpace(kv[1])
		req.Fileds[k] = v
	}

	if contentLength, found := req.Fileds["Content-Length"]; found {
		length, _ := strconv.Atoi(contentLength)
		if length > len(body) {
			return 0, errNeedMore
		}
		req.Body = body[:length]
	}
	return loc + len(req.Body) + 4, nil
}

func (req *RtspRequest) parseFirstLine(firstLine string) error {
	sets := strings.Fields(firstLine)
	if len(sets) < 3 {
		return errors.New("parse rtsp request first line failed")
	}
	req.Method = sets[0]
	req.Uri = sets[1]
	if sets[2] == "RTSP/1.0" {
		req.Version = Rtsp10
	} else if sets[2] == "RTSP/2.0" {
		req.Version = Rtsp20
	} else {
		return errors.New("rtsp parse request failed,unsupport rtsp version")
	}
	return nil
}

func (req *RtspRequest) Encode() string {
	request := req.Method
	request += " " + req.Uri
	if req.Version == Rtsp10 {
		request += " " + "RTSP/1.0\r\n"
	} else if req.Version == Rtsp20 {
		request += " " + "RTSP/2.0\r\n"
	}
	if len(req.Body) > 0 {
		req.Fileds[ContentLength] = strconv.Itoa(len(req.Body))
	}
	for k, v := range req.Fileds {
		request += k + ": " + v + "\r\n"
	}
	request += "\r\n"
	request += req.Body
	return request
}

func makeOptions(uri string, cseq int32) RtspRequest {
	return makeCommonReq(OPTIONS, uri, cseq)
}

func makeSetParameter(uri string, cseq int32) RtspRequest {
	return makeCommonReq(SET_PARAMETER, uri, cseq)
}

func makeGetParameter(uri string, cseq int32) RtspRequest {
	return makeCommonReq(GetParameter, uri, cseq)
}

func makeDescribe(uri string, cseq int32) RtspRequest {
	return makeCommonReq(DESCRIBE, uri, cseq)
}

func makeSetup(uri string, cseq int32) RtspRequest {
	return makeCommonReq(SETUP, uri, cseq)
}

func makePlay(uri string, cseq int32) RtspRequest {
	return makeCommonReq(PLAY, uri, cseq)
}

func makeTeardown(uri string, cseq int32) RtspRequest {
	return makeCommonReq(TEARDOWN, uri, cseq)
}

func makePause(uri string, cseq int32) RtspRequest {
	return makeCommonReq(PAUSE, uri, cseq)
}

func makeAnnounce(uri string, cseq int32) RtspRequest {
	return makeCommonReq(ANNOUNCE, uri, cseq)
}

func makeRecord(uri string, cseq int32) RtspRequest {
	return makeCommonReq(RECORD, uri, cseq)
}

func makeRedirect(uri string, cseq int32) RtspRequest {
	return makeCommonReq(REDIRECT, uri, cseq)
}

func makeCommonReq(method string, uri string, cseq int32) RtspRequest {
	req := RtspRequest{Method: method, Uri: uri, Fileds: make(HeadFiled)}
	req.Fileds.Add(CSeq, cseq)
	req.Version = Rtsp10
	req.Fileds.Add(ContentLength, 0)
	req.Fileds[Date] = time.Now().UTC().Format("02 Jan 06 15:04:05 GMT")
	return req
}

func getReasonByStatusCode(code int) string {
	switch code {
	case OK:
		return "OK"
	case MovedPermanently:
		return "Moved Permanently"
	case MovedTemporarily:
		return "Moved Temporarily"
	case BadRequest:
		return "Bad Request"
	case Unauthorized:
		return "Unauthorized"
	case NotFound:
		return "Not Found"
	case SessionNotFound:
		return "Session Not Found"
	case UnsupportedTransport:
		return "Unsupported transport"
	case InternalServerError:
		return "Internal Server Error"
	case NotImplemented:
		return "Not Implemented"
	case VersionNotSupported:
		return "RTSP Version not supported"
	}
	return "Unsupport StatusCode"
}

type RtspResponse struct {
	Version    int
	StatusCode int
	Reason     string
	Fileds     HeadFiled
	Body       string
}

func (res *RtspResponse) parse(data string) (int, error) {
	loc := strings.Index(data, "\r\n\r\n")
	if loc == -1 {
		return 0, errNeedMore
	}

	body := data[loc+4:]
	data = data[:loc]
	strs := strings.Split(data, "\r\n")

	if len(strs) <= 1 {
		return 0, errors.New("illegal rtsp response")
	}

	err := res.parseFirstLine(strs[0])
	if err != nil {
		return 0, err
	}

	for i := 1; i < len(strs); i++ {
		kv := strings.SplitN(strs[i], ":", 2)
		k := strings.Title(strings.TrimSpace(kv[0]))
		v := strings.TrimSpace(kv[1])
		res.Fileds[k] = v
	}

	if contentLength, found := res.Fileds[ContentLength]; found {
		length, _ := strconv.Atoi(contentLength)
		if length > len(body) {
			return 0, errNeedMore
		}
		res.Body = body[:length]
	}
	return loc + 4 + len(res.Body), nil
}

func (res *RtspResponse) parseFirstLine(firstLine string) error {

	sets := strings.SplitN(firstLine, " ", 3)
	if len(sets) < 3 {
		return errors.New("parse rtsp request first line failed")
	}

	if sets[0] == "RTSP/1.0" {
		res.Version = Rtsp10
	} else if sets[0] == "RTSP/2.0" {
		res.Version = Rtsp20
	} else {
		return errors.New("rtsp parse response failed,unsupport rtsp version")
	}
	res.StatusCode, _ = strconv.Atoi(sets[1])
	res.Reason = sets[2]
	return nil
}

func (res *RtspResponse) Encode() string {
	var response = ""
	if res.Version == Rtsp10 {
		response += "RTSP/1.0 "
	} else if res.Version == Rtsp20 {
		response += "RTSP/2.0 "
	} else {
		response += "RTSP/1.0 "
	}

	if res.Reason == "" {
		res.Reason = getReasonByStatusCode(res.StatusCode)
	}
	response += strconv.Itoa(res.StatusCode) + " " + res.Reason + "\r\n"

	if len(res.Body) > 0 {
		res.Fileds["Content-Length"] = strconv.Itoa(len(res.Body))
	}
	for k, v := range res.Fileds {
		response += k + ": " + v + "\r\n"
	}
	response += "\r\n"
	response += res.Body
	return response
}
