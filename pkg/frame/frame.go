package frame

import "encoding/json"

type CTRLFrame struct {
	Typ  byte
	Data []string
}

const (
	CTRLUNPAIR    = uint8(0)
	CTRLEXPOSETCP = uint8(1)
	CTRLHIDETCP   = uint8(2)
	CTRLEXPOSEUDP = uint8(3)
	CTRLHIDEUDP   = uint8(4)
	CTRLCONNECT   = uint8(5)

	CTRLPORT     uint16 = 47921
	UDPPROXYPORT uint16 = 47922
	TCPPROXYBASE uint16 = 47923
	// Set this to the number of tcp ports you want to relay
	NRTCPPORTS uint16 = 10
)

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
