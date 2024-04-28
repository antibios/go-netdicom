package netdicom

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/antibios/dicom"
	dicomtag "github.com/antibios/dicom/pkg/tag"
	dicomuid "github.com/antibios/dicom/pkg/uid"
	"github.com/antibios/go-dicom/dicomlog"
	"github.com/antibios/go-netdicom/dimse"
)

// Helper function used by C-{STORE,GET,MOVE} to send a dataset using C-STORE
// over an already-established association.
func runCStoreOnAssociation(upcallCh chan upcallEvent, downcallCh chan stateEvent,
	cm *contextManager,
	messageID dimse.MessageID,
	ds *dicom.Dataset) error {
	var getElement = func(tag dicomtag.Tag) (string, error) {
		elem, err := ds.FindElementByTag(tag)
		if err != nil {
			return "", fmt.Errorf("dicom.cstore: data lacks %s: %v", tag.String(), err)
		}
		s := elem.Value.GetValue().([]string)[0]

		return s, nil
	}
	sopInstanceUID, err := getElement(dicomtag.MediaStorageSOPInstanceUID)
	if err != nil {
		return fmt.Errorf("dicom.cstore: data lacks SOPInstanceUID: %v", err)
	}
	sopClassUID, err := getElement(dicomtag.MediaStorageSOPClassUID)
	if err != nil {
		return fmt.Errorf("dicom.cstore: data lacks MediaStorageSOPClassUID: %v", err)
	}
	dicomlog.Vprintf(1, "dicom.cstore(%s): DICOM abstractsyntax: %s, sopinstance: %s", cm.label, dicomuid.UIDString(sopClassUID), sopInstanceUID)
	context, err := cm.lookupByAbstractSyntaxUID(sopClassUID)
	if err != nil {
		dicomlog.Vprintf(0, "dicom.cstore(%s): sop class %v not found in context %v", cm.label, sopClassUID, err)
		return err
	}
	dicomlog.Vprintf(1, "dicom.cstore(%s): using transfersyntax %s to send sop class %s, instance %s",
		cm.label,
		dicomuid.UIDString(context.transferSyntaxUID),
		dicomuid.UIDString(sopClassUID),
		sopInstanceUID)
	// MK Write our own data to the DICOM file.
	bodyEncoder := bytes.Buffer{}
	e := dicom.NewWriter(&bodyEncoder, dicom.SkipVRVerification())
	e.SetTransferSyntax(binary.LittleEndian, true)
	for _, elem := range ds.Elements {
		e.WriteElement(elem)
	}
	downcallCh <- stateEvent{
		event: evt09,
		dimsePayload: &stateEventDIMSEPayload{
			abstractSyntaxName: sopClassUID,
			command: &dimse.CStoreRq{
				AffectedSOPClassUID:    sopClassUID,
				MessageID:              messageID,
				CommandDataSetType:     int(dimse.CommandDataSetTypeNonNull),
				AffectedSOPInstanceUID: sopInstanceUID,
			},
			data: bodyEncoder.Bytes(),
		},
	}
	for {
		dicomlog.Vprintf(0, "dicom.cstore(%s): Start reading resp w/ messageID:%v", cm.label, messageID)
		event, ok := <-upcallCh
		if !ok {
			return fmt.Errorf("dicom.cstore(%s): Connection closed while waiting for C-STORE response", cm.label)
		}
		dicomlog.Vprintf(1, "dicom.cstore(%s): resp event: %v", cm.label, event.command)
		doassert(event.eventType == upcallEventData)
		doassert(event.command != nil)
		resp, ok := event.command.(*dimse.CStoreRsp)
		doassert(ok) // TODO(saito)
		if resp.Status.Status != 0 {
			return fmt.Errorf("dicom.cstore(%s): failed: %v", cm.label, resp.String())
		}
		return nil
	}
}
