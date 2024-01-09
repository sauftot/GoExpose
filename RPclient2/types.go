package main

import "encoding/json"

type CTRLFrame struct {
	typ  byte
	data []string
}

const (
	CTRLUNPAIR    = uint8(0)
	CTRLEXPOSETCP = uint8(1)
	CTRLHIDETCP   = uint8(2)
	CTRLEXPOSEUDP = uint8(3)
	CTRLHIDEUDP   = uint8(4)
	CTRLCONNECT   = uint8(5)
)

func toByteArray(ctrlFrame *CTRLFrame) ([]byte, error) {
	jsonBytes, err := json.Marshal(ctrlFrame)
	if err != nil {
		return nil, err
	}
	return jsonBytes, nil
}

func fromByteArray(jsonBytes []byte) (*CTRLFrame, error) {
	ctrlFrame := &CTRLFrame{}
	err := json.Unmarshal(jsonBytes, ctrlFrame)
	if err != nil {
		return nil, err
	}
	return ctrlFrame, nil
}
