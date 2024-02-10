package internal

import (
	"encoding/json"
	"net"
)

const (
	CTRLUNPAIR    = uint8(200)
	CTRLEXPOSETCP = uint8(201)
	CTRLHIDETCP   = uint8(202)
	CTRLEXPOSEUDP = uint8(203)
	CTRLHIDEUDP   = uint8(204)
	CTRLCONNECT   = uint8(205)
	STOP          = uint8(0)
)

type CTRLFrame struct {
	Typ  byte
	Data []string
}

func NewCTRLFrame(typ byte, data []string) *CTRLFrame {
	return &CTRLFrame{
		Typ:  typ,
		Data: data,
	}
}

func ToByteArray(ctrlFrame *CTRLFrame) ([]byte, error) {
	jsonBytes, err := json.Marshal(ctrlFrame)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

func FromByteArray(jsonBytes []byte) (*CTRLFrame, error) {
	ctrlFrame := &CTRLFrame{}
	err := json.Unmarshal(jsonBytes, ctrlFrame)
	if err != nil {
		return nil, err
	}
	return ctrlFrame, nil
}

func ReadFrame(conn net.Conn) (*CTRLFrame, error) {
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	fr, err := FromByteArray(buf[:n])
	if err != nil {
		return nil, err
	}
	return fr, nil
}

func WriteFrame(conn net.Conn, fr *CTRLFrame) error {
	jsonBytes, err := ToByteArray(fr)
	if err != nil {
		return err
	}
	_, err = conn.Write(jsonBytes)
	if err != nil {
		return err
	}
	return nil
}
