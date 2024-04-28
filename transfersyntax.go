package netdicom

import (
	"encoding/binary"
	"fmt"

	dicomuid "github.com/antibios/dicom/pkg/uid"
)

// IsImplicitVR defines whether a 2-character VR tag is emit with each data
// element.
type IsImplicitVR int

const (
	// TODO(saito) Where are implicit/explicit defined? Add a ref!

	// ImplicitVR encodes a data element without a VR tag. The reader
	// consults the static tag->VR mapping (see tags.go) defined by DICOM
	// standard.
	ImplicitVR IsImplicitVR = iota

	// ExplicitVR stores the 2-byte VR value inline w/ a data element.
	ExplicitVR

	// UnknownVR is to be used when you never encode or decode DataElement.
	UnknownVR
)

// StandardTransferSyntaxes is the list of standard transfer syntaxes.
var StandardTransferSyntaxes = []string{
	dicomuid.ImplicitVRLittleEndian,
	dicomuid.ExplicitVRLittleEndian,
	dicomuid.ExplicitVRBigEndian,
	dicomuid.DeflatedExplicitVRLittleEndian,
}

// CanonicalTransferSyntaxUID return the canonical transfer syntax UID (e.g.,
// dicomuid.ExplicitVRLittleEndian or dicomuid.ImplicitVRLittleEndian), given an
// UID that represents any transfer syntax.  Returns an error if the uid is not
// defined in DICOM standard, or if the uid does not represent a transfer
// syntax.
//
// TODO(saito) Check the standard to see if we need to accept unknown UIDS as
// explicit little endian.
func CanonicalTransferSyntaxUID(uid string) (string, error) {
	// defaults are explicit VR, little endian
	switch uid {
	case dicomuid.ImplicitVRLittleEndian,
		dicomuid.ExplicitVRLittleEndian,
		dicomuid.ExplicitVRBigEndian,
		dicomuid.DeflatedExplicitVRLittleEndian:
		return uid, nil
	default:
		e, err := dicomuid.Lookup(uid)
		if err != nil {
			return "", err
		}
		if e.Type != dicomuid.TypeTransferSyntax {
			return "", fmt.Errorf("dicom.CanonicalTransferSyntaxUID: '%s' is not a transfer syntax (is %s)", uid, e.Type)
		}
		// The default is ExplicitVRLittleEndian
		return dicomuid.ExplicitVRLittleEndian, nil
	}
}

// ParseTransferSyntaxUID parses a transfer syntax uid and returns its byteorder
// and implicitVR/explicitVR type.  TrasnferSyntaxUID can be any UID that refers to
// a transfer syntax. It can be, e.g., 1.2.840.10008.1.2 (it will return
// LittleEndian, ImplicitVR) or 1.2.840.10008.1.2.4.54 (it will return
// (LittleEndian, ExplicitVR).
func ParseTransferSyntaxUID(uid string) (bo binary.ByteOrder, implicit IsImplicitVR, err error) {
	canonical, err := CanonicalTransferSyntaxUID(uid)
	if err != nil {
		return nil, UnknownVR, err
	}
	switch canonical {
	case dicomuid.ImplicitVRLittleEndian:
		return binary.LittleEndian, ImplicitVR, nil
	case dicomuid.DeflatedExplicitVRLittleEndian:
		fallthrough
	case dicomuid.ExplicitVRLittleEndian:
		return binary.LittleEndian, ExplicitVR, nil
	case dicomuid.ExplicitVRBigEndian:
		return binary.BigEndian, ExplicitVR, nil
	default:
		panic(fmt.Sprintf("Invalid transfer syntax: %v,  %v", canonical, uid))
	}
}
