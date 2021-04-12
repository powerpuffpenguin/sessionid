package sessionid

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"encoding/xml"

	"github.com/google/uuid"
)

type Encoder interface {
	Encode(interface{}) ([]byte, error)
}
type Decoder interface {
	Decode([]byte, interface{}) error
}
type Coder interface {
	Encoder
	Decoder
}

func newSessionID() (sessionid string, e error) {
	u, e := uuid.NewUUID()
	if e != nil {
		return
	}
	sessionid = encode(u[:])
	return
}

func encode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

type GOBCoder struct{}
type JSONCoder struct{}
type XMLCoder struct{}

func (GOBCoder) Encode(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	e := gob.NewEncoder(&buf).Encode(v)
	if e != nil {
		return nil, e
	}
	return buf.Bytes(), nil
}
func (GOBCoder) Decode(data []byte, v interface{}) error {
	return gob.NewDecoder(bytes.NewBuffer(data)).Decode(v)
}
func (JSONCoder) Encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
func (JSONCoder) Decode(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
func (XMLCoder) Encode(v interface{}) ([]byte, error) {
	return xml.Marshal(v)
}
func (XMLCoder) Decode(data []byte, v interface{}) error {
	return xml.Unmarshal(data, v)
}
