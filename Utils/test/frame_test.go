package test

import (
	"Utils"
	"strings"
	"testing"
)

func TestFrameToJsonAndBack(t *testing.T) {
	fr := Utils.NewCTRLFrame(Utils.CTRLUNPAIR, []string{
		"test",
		strings.Repeat("a", 1024),
	})

	t.Log("Frame created", "frame", fr)

	jsonBytes, err := Utils.ToByteArray(fr)
	if err != nil {
		t.Error("Error converting frame to json", err)
	}

	t.Log("Frame converted", "json", string(jsonBytes))

	fr2, err := Utils.FromByteArray(jsonBytes)
	if err != nil {
		t.Error("Error converting json to frame", err)
	}

	t.Log("Frame converted back", "frame", fr2)

	if fr.Typ != fr2.Typ {
		t.Error("Frame type mismatch")
	}

	if fr.Data[0] != fr2.Data[0] {
		t.Error("Frame data mismatch")
	}
}

func FuzzFrameJson(f *testing.F) {
	for _, seed := range [][]byte{{}, {0}, {9}, {0xa}, {0xf}, {1, 2, 3, 4}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		fr := Utils.NewCTRLFrame(Utils.CTRLCONNECT, []string{string(in)})
		jsonBytes, err := Utils.ToByteArray(fr)
		if err != nil {
			t.Fatal("Error converting frame to json", err)
		}
		fr2, err := Utils.FromByteArray(jsonBytes)
		if err != nil {
			t.Fatal("Error converting json to frame", err)
		}
		if fr.Typ != fr2.Typ {
			t.Fatal("Frame type mismatch")
		}
		for i := range fr.Data {
			if fr.Data[i] != fr2.Data[i] {
				t.Fatal("Frame data mismatch", "Expected", []byte(fr.Data[i]), "Got", []byte(fr2.Data[i]))
			}
		}
	})
}
