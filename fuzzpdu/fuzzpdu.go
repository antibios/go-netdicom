package fuzzpdu

import (
	"bytes"
	"flag"

	"github.com/antibios/dicom"
	"github.com/antibios/go-netdicom/dimse"
	"github.com/antibios/go-netdicom/pdu"
)

func init() {
	flag.Parse()
}

func Fuzz(data []byte) int {
	in := bytes.NewBuffer(data)
	if len(data) == 0 || data[0] <= 0xc0 {
		pdu.ReadPDU(in, 4<<20) // nolint: errcheck
	} else {
		//d := dicomio.NewDecoder(in, binary.LittleEndian, dicomio.ExplicitVR)
		d, err := dicom.ReadDataSetInBytes(&data, nil, nil)
		if err != nil {
			panic(err)
		}

		dimse.ReadMessage(d)
	}
	return 0
}
