package libnet

import (
	"encoding/binary"
	"github.com/libp2p/go-libp2p/core/network"
	"google.golang.org/protobuf/proto"
	"io"
)

// Write a protobuf message to a libp2p stream
func WriteProtoMsgStream(s network.Stream, msg proto.Message) error {
	return WriteProtoMsg(s, msg) // s implements io.Writer
}

// Read a protobuf message from a libp2p stream
func ReadProtoMsgStream(s network.Stream, msg proto.Message) error {
	return ReadProtoMsg(s, msg) // s implements io.Reader
}

// Generic length-prefix functions
func WriteProtoMsg(w io.Writer, msg proto.Message) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	length := uint32(len(data))
	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func ReadProtoMsg(r io.Reader, msg proto.Message) error {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	return proto.Unmarshal(buf, msg)
}
