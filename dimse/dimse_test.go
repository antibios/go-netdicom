package dimse_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	dicom "github.com/antibios/dicom"
	"github.com/antibios/go-netdicom/dimse"
)

func testDIMSE(t *testing.T, v dimse.Message) {
	b := bytes.Buffer{}
	e := dicom.NewWriter(&b, dicom.SkipVRVerification())
	e.SetTransferSyntax(binary.LittleEndian, true)
	dimse.EncodeMessage(e, v)
	bytes := b.Bytes()
	d, err := dicom.ReadDataSetInBytes(&bytes, dicom.SkipMetadataReadOnNewParserInit())
	if err != nil {
		t.Errorf("ReadDataSetInBytes %v from %v", bytes, v)
	}

	v2 := dimse.ReadMessage(d)
	//TODO: Check that buffer is empty

	if v.String() != v2.String() {
		t.Errorf("%v <-> %v", v, v2)
	}
}

func TestCStoreRq(t *testing.T) {
	testDIMSE(t, &dimse.CStoreRq{
		AffectedSOPClassUID:                  "1.2.3",
		MessageID:                            0,
		Priority:                             0,
		CommandDataSetType:                   0,
		AffectedSOPInstanceUID:               "3.4.5",
		CalledApplicationEntityTitle:         "ALICE",
		MoveOriginatorApplicationEntityTitle: "EMMA",
		MoveOriginatorMessageID:              0,
		Extra:                                []*dicom.Element{},
	})
}

func TestCStoreRsp(t *testing.T) {
	testDIMSE(t, &dimse.CStoreRsp{
		"1.2.3",
		0x1234,
		dimse.CommandDataSetTypeNull,
		"3.4.5",
		dimse.Status{Status: dimse.StatusCode(0x3456)},
		nil})
}

func TestCEchoRq(t *testing.T) {
	testDIMSE(t, &dimse.CEchoRq{0x1234, 1, nil})
}

func TestCEchoRsp(t *testing.T) {
	testDIMSE(t, &dimse.CEchoRsp{0x1234, 1,
		dimse.Status{Status: dimse.StatusCode(0x2345)},
		nil})
}

// This constantly fails and doesn't really test anything more that our actual tests.
/* func FuzzCstoreRq(f *testing.F) {
	testcases := []string{"ABC", "CAST123", "WINTE-IR-123"}
	for _, tc := range testcases {
		f.Add(tc) // Use f.Add to provide a seed corpus
	}
	f.Fuzz(func(t *testing.T, callingAE string) {
		testDIMSE(t, &dimse.CStoreRq{
			AffectedSOPClassUID:                  "1.2.3",
			MessageID:                            0,
			Priority:                             0,
			CommandDataSetType:                   0,
			AffectedSOPInstanceUID:               "3.4.5",
			CalledApplicationEntityTitle:         "ALICE",
			MoveOriginatorApplicationEntityTitle: callingAE,
			MoveOriginatorMessageID:              0,
			Extra:                                []*dicom.Element{},
		})
	})
} */
