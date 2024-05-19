package rtmp

import (
	"encoding/binary"
	"fmt"
	"math"
)

type Amf0DataType int

const (
	Amf0Number Amf0DataType = iota
	Amf0Boolean
	Amf0String
	Amf0Object
	Amf0Movieclip
	Amf0Null
	Amf0Undefined
	Amf0Reference
	Amf0EcmaArray
	Amf0ObjectEnd
	Amf0StrictArray
	Amf0Date
	Amf0LongString
	Amf0Unsupported
	Amf0Recordset
	Amf0XmlDocument
	Amf0TypedObject
	Amf0AvmplusObject
)

var NullItem = []byte{byte(Amf0Null)}
var EndObj = []byte{0, 0, byte(Amf0ObjectEnd)}

type amf0Item struct {
	amfType Amf0DataType
	length  int
	value   interface{}
}

func (amf *amf0Item) encode() []byte {
	buf := make([]byte, amf.length+4+8)
	switch amf.amfType {
	case Amf0Number:
		buf[0] = byte(Amf0Number)
		binary.BigEndian.PutUint64(buf[1:], math.Float64bits(amf.value.(float64)))
		return buf[:9]
	case Amf0Boolean:
		buf[0] = byte(Amf0Boolean)
		v := amf.value.(bool)
		if v {
			buf[1] = 1
		} else {
			buf[1] = 0
		}
		return buf[0:2]
	case Amf0String:
		buf[0] = byte(Amf0String)
		buf[1] = byte(uint16(amf.length) >> 8)
		buf[2] = byte(uint16(amf.length))
		copy(buf[3:], []byte(amf.value.(string)))
		return buf[0 : 3+amf.length]
	case Amf0Movieclip:
	case Amf0Null:
		buf[0] = byte(Amf0Null)
		return buf[0:1]
	case Amf0Undefined:
	case Amf0Reference:
	case Amf0EcmaArray:
	case Amf0StrictArray:
	case Amf0Date:
	case Amf0LongString:
	case Amf0Unsupported:
	case Amf0Recordset:
	case Amf0XmlDocument:
	case Amf0TypedObject:
	case Amf0AvmplusObject:
	default:
		panic("unsupport")
	}
	return nil
}

func (amf *amf0Item) decode(data []byte) int {
	_ = data[0]
	amf.amfType = Amf0DataType(data[0])
	switch amf.amfType {
	case Amf0Number:
		amf.length = 8
		v := math.Float64frombits(binary.BigEndian.Uint64(data[1:]))
		amf.value = v
		return 9
	case Amf0Boolean:
		amf.length = 1
		if data[1] == 1 {
			amf.value = true
		} else {
			amf.value = false
		}
		return 2
	case Amf0String:
		amf.length = int(binary.BigEndian.Uint16(data[1:]))
		str := make([]byte, amf.length)
		copy(str, data[3:3+amf.length])
		amf.value = str
		return 3 + amf.length
	case Amf0Null:
	case Amf0LongString:
		amf.length = int(binary.BigEndian.Uint32(data[1:]))
		str := make([]byte, amf.length)
		copy(str, data[5:5+amf.length])
		return 5 + amf.length
	case Amf0Undefined:
	case Amf0EcmaArray:
		return 5
	default:
		panic(fmt.Sprintf("unsupport amf type %d", amf.amfType))
	}
	return 1
}

func makeStringItem(str string) amf0Item {
	item := amf0Item{
		amfType: Amf0String,
		length:  len(str),
		value:   str,
	}
	return item
}

func makeNumberItem(num float64) amf0Item {
	item := amf0Item{
		amfType: Amf0Number,
		value:   num,
	}
	return item
}

func makeBoolItem(v bool) amf0Item {
	item := amf0Item{
		amfType: Amf0Boolean,
		value:   v,
	}
	return item
}

type amfObjectItem struct {
	name  string
	value amf0Item
}

type amfObject struct {
	items []*amfObjectItem
}

func (object *amfObject) encode() []byte {
	obj := make([]byte, 1)
	obj[0] = byte(Amf0Object)
	for _, item := range object.items {
		lenbytes := make([]byte, 2)
		binary.BigEndian.PutUint16(lenbytes, uint16(len(item.name)))
		obj = append(obj, lenbytes...)
		obj = append(obj, []byte(item.name)...)
		obj = append(obj, item.value.encode()...)
	}
	obj = append(obj, EndObj...)
	return obj
}

func (object *amfObject) decode(data []byte) int {
	total := 1
	data = data[1:]
	isArray := false
	for len(data) > 0 {
		if data[0] == 0x00 && data[1] == 0x00 && data[2] == byte(Amf0ObjectEnd) {
			total += 3
			if isArray {
				isArray = false
				continue
			} else {
				break
			}
		}
		length := binary.BigEndian.Uint16(data)
		name := string(data[2 : 2+length])
		item := amf0Item{}
		l := item.decode(data[2+length:])
		if item.amfType == Amf0EcmaArray {
			isArray = true
		} else {
			obj := &amfObjectItem{
				name:  name,
				value: item,
			}
			object.items = append(object.items, obj)
		}
		data = data[2+int(length)+l:]
		total += 2 + int(length) + l
	}
	return total
}

func decodeAmf0(data []byte) (items []amf0Item, objs []amfObject) {
	for len(data) > 0 {
		switch Amf0DataType(data[0]) {
		case Amf0EcmaArray:
			data = data[5:]
			fallthrough
		case Amf0Object:
			obj := amfObject{}
			l := obj.decode(data)
			data = data[l:]
			objs = append(objs, obj)
		default:
			item := amf0Item{}
			l := item.decode(data)
			data = data[l:]
			items = append(items, item)
		}
	}
	return
}
