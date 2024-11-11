package serf

import (
	"bytes"
	"fmt"
	"math/rand"

	"github.com/hashicorp/go-msgpack/codec"
)

func encode(t msgType, in interface{}) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte(uint8(t))
	h := codec.MsgpackHandle{}
	enc := codec.NewEncoder(buf, &h)
	if err := enc.Encode(in); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeTags(tags map[string]string) ([]byte, error) {
	return encode(tagMagicByte, tags)
}

func decode(buf []byte, out interface{}) error {
	r := bytes.NewReader(buf)
	h := codec.MsgpackHandle{}
	dec := codec.NewDecoder(r, &h)
	return dec.Decode(out)
}

func decodeTags(msg []byte) (map[string]string, error) {
	if len(msg) == 0 {
		return map[string]string{}, nil
	}
	if msg[0] != byte(tagMagicByte) {
		return nil, fmt.Errorf("missing tag magic byte")
	}
	tags := make(map[string]string)
	err := decode(msg[1:], tags)
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func randIntN(n int) int {
	if n == 0 { // if n == 0, modulo will panic
		return 0
	}
	return int(rand.Uint32() % uint32(n))
}
