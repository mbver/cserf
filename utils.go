// Copyright (c) HashiCorp, Inc.
// Copyright (c) 2024 Phuoc Phi
// SPDX-License-Identifier: MPL-2.0
package serf

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"

	"github.com/hashicorp/go-msgpack/v2/codec"
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

func EncodeTags(tags map[string]string) ([]byte, error) {
	return encode(tagMagicByte, tags)
}

func decode(buf []byte, out interface{}) error {
	r := bytes.NewReader(buf)
	h := codec.MsgpackHandle{}
	dec := codec.NewDecoder(r, &h)
	return dec.Decode(out)
}

func DecodeTags(msg []byte) (map[string]string, error) {
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

func TagMapToString(tags map[string]string) string {
	buf := &strings.Builder{}
	for k, v := range tags {
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(v)
		buf.WriteString(",")
	}
	s := buf.String()
	return strings.TrimSuffix(s, ",")
}

func TagsEncodedToString(tag []byte) (string, error) {
	m, err := DecodeTags(tag)
	if err != nil {
		return "", err
	}
	return TagMapToString(m), nil
}

func ToTagMap(s string) map[string]string {
	m := map[string]string{}
	kvs := strings.Split(s, ",")
	for _, kv := range kvs {
		pair := strings.Split(kv, "=")
		if len(pair) < 2 {
			continue
		}
		m[pair[0]] = pair[1]
	}
	return m
}
