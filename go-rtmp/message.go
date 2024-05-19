package rtmp

// Protocol control messages MUST have message stream ID 0 (called as control stream) and chunk stream ID 2, and are sent with highest
// priority.

type MessageType int

const (
	//Protocol control messages
	SetChunkSize    MessageType = 1
	AbortMessage    MessageType = 2
	ACKNOWLEDGEMENT MessageType = 3
	UserControl     MessageType = 4
	WndAckSize      MessageType = 5
	SetPeerBw       MessageType = 6

	AUDIO            MessageType = 8
	VIDEO            MessageType = 9
	CommandAmf0      MessageType = 20
	CommandAmf3      MessageType = 17
	MetadataAmf0     MessageType = 18
	MetadataAmf3     MessageType = 15
	SharedobjectAmf0 MessageType = 19
	SharedobjectAmf3 MessageType = 16
	Aggregate        MessageType = 22
)

type rtmpMessage struct {
	timestamp uint32
	msg       []byte
	msgtype   MessageType
	streamid  uint32
}
