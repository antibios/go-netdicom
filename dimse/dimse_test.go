package dimse_test

import (
	"encoding/binary"
	"testing"

	dicom "github.com/antibios/go-dicom"
	"github.com/antibios/go-dicom/dicomio"
	"github.com/antibios/go-netdicom/dimse"
)

func testDIMSE(t *testing.T, v dimse.Message) {
	e := dicomio.NewBytesEncoder(binary.LittleEndian, dicomio.ImplicitVR)
	dimse.EncodeMessage(e, v)
	bytes := e.Bytes()
	d := dicomio.NewBytesDecoder(bytes, binary.LittleEndian, dicomio.ImplicitVR)
	v2 := dimse.ReadMessage(d)
	err := d.Finish()
	if err != nil {
		t.Fatal(err)
	}
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
		MoveOriginatorApplicationEntityTitle: "BOB",
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
