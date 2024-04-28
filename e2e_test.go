package netdicom

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/antibios/dicom"
	"github.com/antibios/dicom/pkg/tag"
	"github.com/antibios/dicom/pkg/uid"
	"github.com/antibios/go-netdicom/dimse"
	"github.com/antibios/go-netdicom/sopclass"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var provider *ServiceProvider

var cstoreData []byte            // data received by the cstore handler
var cstoreStatus = dimse.Success // status returned by the cstore handler
var nEchoRequests int
var once sync.Once

func TestMain(m *testing.M) {
	flag.Parse()
	var err error
	provider, err = NewServiceProvider(ServiceProviderParams{
		CEcho:  onCEchoRequest,
		CStore: onCStoreRequest,
		CFind:  onCFindRequest,
		CGet:   onCGetRequest,
	}, ":0")
	if err != nil {
		panic(err)
	}
	go provider.Run()
	os.Exit(m.Run())
}

func onCEchoRequest(connState ConnectionState) dimse.Status {
	nEchoRequests++
	return dimse.Success
}

func onCStoreRequest(
	connState ConnectionState,
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string,
	callingAETitle string,
	calledAETitle string,
	data []byte) dimse.Status {
	log.Printf("Start C-STORE handler, transfersyntax=%s, sopclass=%s, sopinstance=%s",
		uid.UIDString(transferSyntaxUID),
		uid.UIDString(sopClassUID),
		uid.UIDString(sopInstanceUID))

	/* e := dicomio.NewBytesEncoder(nil, dicomio.UnknownVR)
	dicom.WriteFileHeader(e, */
	b := bytes.Buffer{}
	e := dicom.NewWriter(&b, dicom.SkipVRVerification())
	e.SetTransferSyntax(binary.LittleEndian, true)

	for _, elem := range []*dicom.Element{
		dicom.MustNewElement(tag.TransferSyntaxUID, transferSyntaxUID),
		dicom.MustNewElement(tag.MediaStorageSOPClassUID, sopClassUID),
		dicom.MustNewElement(tag.MediaStorageSOPInstanceUID, sopInstanceUID),
	} {
		e.WriteElement(elem)
	}
	e.WriteBytes(data)
	cstoreData = data
	log.Printf("Received C-STORE request, %d bytes", len(cstoreData))
	return cstoreStatus
}

func onCFindRequest(
	connState ConnectionState,
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element,
	ch chan CFindResult) {
	log.Printf("Received cfind request")
	found := 0
	for _, elem := range filters {
		log.Printf("Filter %v", elem)
		if elem.Tag == tag.QueryRetrieveLevel {
			if elem.String() != "PATIENT" {
				log.Panicf("Wrong QR level: %v", elem)
			}
			found++
		}
		if elem.Tag == tag.PatientName {
			if elem.String() != "foohah" {
				log.Panicf("Wrong patient name: %v", elem)
			}
			found++
		}
	}
	if found != 2 {
		log.Panicf("Didn't find expected filters: %v", filters)
	}
	ch <- CFindResult{
		Elements: []*dicom.Element{dicom.MustNewElement(tag.PatientName, "johndoe")},
	}
	ch <- CFindResult{
		Elements: []*dicom.Element{dicom.MustNewElement(tag.PatientName, "johndoe2")},
	}
	close(ch)
}

func onCGetRequest(
	connState ConnectionState,
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element,
	ch chan CMoveResult) {
	log.Printf("Received cget request")
	path := "testdata/reportsi.dcm"
	dataset := mustReadDICOMFile(path)
	ch <- CMoveResult{
		Remaining: -1,
		Path:      path,
		DataSet:   dataset,
	}
	close(ch)
}

// Test compare elements a and b with i being an count of the element in the DICOM header.
func compareElements(t *testing.T, a, b dicom.Element, i int) bool {
	if a.Tag != b.Tag {
		t.Errorf("%dth Tag element mismatch: %v <-> %v", i, a.Tag, b.Tag)
	}
	if a.RawValueRepresentation != b.RawValueRepresentation {
		t.Errorf("%dth Tag Representation mismatch: %v <-> %v", i, b.RawValueRepresentation, b.RawValueRepresentation)
	}

	if a.RawValueRepresentation == "SQ" {
		as := a.Value.GetValue().([]*dicom.SequenceItemValue)
		bs := b.Value.GetValue().([]*dicom.SequenceItemValue)
		if len(as) != len(bs) {
			t.Errorf("Sequence length incorrect %v <-> %v", as, bs)
		}
		for j, _ := range as {
			for n, _ := range as[j].GetValue().([]*dicom.Element) {
				compareElements(t, *as[j].GetValue().([]*dicom.Element)[n], *bs[j].GetValue().([]*dicom.Element)[n], n)

			}
		}
		//Now check Value
	} else if !reflect.DeepEqual(a.Value.GetValue(), b.Value.GetValue()) {
		t.Errorf("%dth Value element mismatch: %v <-> %v", i, a.Value.GetValue(), b.Value.GetValue())
	}
	return true
}

// Check that two datasets, "in" and "out" are the same, except for metadata
// elements.
func checkFileBodiesEqual(t *testing.T, in, out *dicom.Dataset) {
	// DCMTK arbitrarily changes the sequences and items to use
	// undefined-length encoding, so ignore such diffs.
	/* 	var normalize = func(s string) string {
		s = strings.Replace(s, "NA u", "NA ", -1)
		s = strings.Replace(s, "SQ u", "SQ ", -1)
		return s
	} */

	var removeMetaElems = func(f *dicom.Dataset) []*dicom.Element {
		var elems []*dicom.Element
		for _, elem := range f.Elements {
			if elem.Tag.Group != tag.MetadataGroup {
				elems = append(elems, elem)
			}
		}
		return elems
	}

	inElems := removeMetaElems(in)
	outElems := removeMetaElems(out)
	assert.Equal(t, len(inElems), len(outElems))
	for i := 0; i < len(inElems); i++ {
		/* 		ins := normalize(inElems[i].String())
		   		outs := normalize(outElems[i].String())
		   		if ins != outs {
		   			t.Errorf("%dth element mismatch: %v <-> %v", i, ins, outs)
		   		}
		*/
		//Tag
		compareElements(t, *inElems[i], *outElems[i], i)
	}
}

// Get the dataset received by the cstore handler.
func getCStoreData() (*dicom.Dataset, error) {
	if len(cstoreData) == 0 {
		return nil, errors.New("Did not receive C-STORE data")
	}
	//f, err := dicom.ReadDataSetInBytes(cstoreData, dicom.ReadOptions{})
	//reader := bytes.NewReader(cstoreData)
	//f, err := dicom.ParseUntilEOF(reader, nil, dicom.SkipMetadataReadOnNewParserInit())
	f, err := dicom.ReadDataSetInBytes(&cstoreData, dicom.SkipMetadataReadOnNewParserInit())
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func mustReadDICOMFile(path string) *dicom.Dataset {
	dataset, err := dicom.ParseFile(path, nil)
	if err != nil {
		log.Panic(err)
	}
	return &dataset
}

func mustNewServiceUser(t *testing.T, sopClasses []string) *ServiceUser {
	su, err := NewServiceUser(ServiceUserParams{SOPClasses: sopClasses})
	require.NoError(t, err)
	log.Printf("Connecting to %v", provider.ListenAddr().String())
	su.Connect(provider.ListenAddr().String())
	return su
}

func TestStore(t *testing.T) {
	dataset := mustReadDICOMFile("testdata/IM-0001-0003.dcm")
	su := mustNewServiceUser(t, sopclass.StorageClasses)
	defer su.Release()
	err := su.CStore(dataset)
	if err != nil {
		log.Panic(err)
	}
	log.Printf("Store done!!")

	out, err := getCStoreData()
	if err != nil {
		log.Panic(err)
	}
	checkFileBodiesEqual(t, dataset, out)
}

// Arrange so that the cstore server returns an error. The client should detect
// that.
func TestStoreFailure0(t *testing.T) {
	dataset := mustReadDICOMFile("testdata/IM-0001-0003.dcm")
	cstoreStatus = dimse.Status{Status: dimse.StatusNotAuthorized, ErrorComment: "Foohah"}
	defer func() { cstoreStatus = dimse.Success }()
	su := mustNewServiceUser(t, sopclass.StorageClasses)
	defer su.Release()
	err := su.CStore(dataset)
	if err == nil || strings.Index(err.Error(), "Foohah") < 0 {
		log.Panic(err)
	}
}

func getProviderPort() string {
	match := regexp.MustCompile("(\\d+)$").FindStringSubmatch(provider.ListenAddr().String())
	return match[1]
}

func TestDCMTKEcho(t *testing.T) {
	echoscuPath, err := exec.LookPath("echoscu")
	if err != nil {
		t.Skip("echoscu not found.")
		return
	}
	cstoreData = nil
	cmd := exec.Command(echoscuPath, "localhost", getProviderPort())
	require.NoError(t, cmd.Run())

}

func waitForDicomSuccess() bool {
	// Set a timeout of 3 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		for {
			// Test your condition here
			// ...
			if cstoreStatus == dimse.Success {
				cancel() // Cancel the context if the condition becomes true
				return
			}
			select {
			case <-ctx.Done():
				fmt.Println("cstoreStatus not met within timeout: ", cstoreStatus)
				return
			case <-time.After(100 * time.Millisecond):
				// Do nothing, loop continues checking the condition
			}
		}
	}()

	// Wait for the goroutine to finish
	<-ctx.Done()

	if cstoreStatus == dimse.Success {
		fmt.Println("Condition became true within timeout")
		return true
	}
	return false
}

// Test using "storescu" command from dcmtk.
func TestDCMTKCStore(t *testing.T) {
	storescuPath, err := exec.LookPath("storescu")
	if err != nil {
		t.Skip("storescu not found.")
		return
	}
	cstoreData = nil
	cmd := exec.Command(storescuPath, "localhost", getProviderPort(), "testdata/reportsi.dcm")
	require.NoError(t, cmd.Run())
	require.True(t, waitForDicomSuccess() == true, "No sucessful send")
	require.True(t, len(cstoreData) > 0, "No data received")
	ds, err := dicom.ReadDataSetInBytes(&cstoreData)
	require.NoError(t, err)
	expected := mustReadDICOMFile("testdata/reportsi.dcm")
	checkFileBodiesEqual(t, expected, &ds)
}

// Test using "getscu" command from dcmtk.
func TestDCMTKCGet(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	getscuPath, err := exec.LookPath("getscu")
	if err != nil {
		t.Skip("getscu not found.")
		return
	}
	log.Printf("PORT is %v %v", getProviderPort(), tempDir)
	cmd := exec.Command(getscuPath, "localhost", getProviderPort(), "-od", tempDir, "-k", "0010,0020=foo" /*not used*/)
	require.NoError(t, cmd.Run())
	require.NoError(t, err)

	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Equal(t, len(files), 1)
	t.Logf("Found C-GET file %v/%v", tempDir, files[0].Name())
	expected := mustReadDICOMFile("testdata/reportsi.dcm")
	ds, err := dicom.ParseFile(filepath.Join(tempDir, files[0].Name()), nil, nil)
	//ds, err := dicom.ReadDataSetFromFile(filepath.Join(tempDir, files[0].Name()), dicom.ReadOptions{})
	require.NoError(t, err)
	checkFileBodiesEqual(t, expected, &ds)
}

type testFaultInjector struct {
	connected bool
}

func (fi *testFaultInjector) onStateTransition(oldState stateType, event *stateEvent, action *stateAction, newState stateType) {
	if newState == sta06 {
		// sta06 is the "association ready" state.
		fi.connected = true
	}
}

func (fi *testFaultInjector) onSend(data []byte) faultInjectorAction {
	if fi.connected {
		log.Printf("Disconnecting!")
		return faultInjectorDisconnect
	}
	return faultInjectorContinue
}

func (fi *testFaultInjector) String() string {
	return "testFaultInjector"
}

// Similar to the previous test, but inject a network failure during send.
func TestStoreFailure1(t *testing.T) {
	dataset := mustReadDICOMFile("testdata/IM-0001-0003.dcm")
	SetUserFaultInjector(&testFaultInjector{})
	defer SetUserFaultInjector(nil)

	su := mustNewServiceUser(t, sopclass.StorageClasses)
	defer su.Release()
	err := su.CStore(dataset)
	if err == nil || strings.Index(err.Error(), "Connection failed") < 0 {
		log.Panic(err)
	}
}

func TestEcho(t *testing.T) {
	su := mustNewServiceUser(t, sopclass.VerificationClasses)
	defer su.Release()
	oldCount := nEchoRequests
	if err := su.CEcho(); err != nil {
		log.Panic(err)
	}
	if nEchoRequests != oldCount+1 {
		log.Panic("C-ECHO handler did not run")
	}
}

func TestFind(t *testing.T) {
	su := mustNewServiceUser(t, sopclass.QRFindClasses)
	defer su.Release()
	filter := []*dicom.Element{
		dicom.MustNewElement(tag.PatientName, "foohah"),
	}
	var namesFound []string

	for result := range su.CFind(QRLevelPatient, filter) {
		log.Printf("Got result: %v", result)
		if result.Err != nil {
			t.Error(result.Err)
			continue
		}
		for _, elem := range result.Elements {
			if elem.Tag != tag.PatientName {
				t.Error(elem)
			}
			namesFound = append(namesFound, elem.Value.GetValue().(string))
		}
	}
	if len(namesFound) != 2 || namesFound[0] != "johndoe" || namesFound[1] != "johndoe2" {
		t.Error(namesFound)
	}
}

func TestCGet(t *testing.T) {
	su := mustNewServiceUser(t, sopclass.QRGetClasses)
	defer su.Release()
	filter := []*dicom.Element{
		dicom.MustNewElement(tag.PatientName, "foohah"),
	}

	var cgetData []byte

	err := su.CGet(QRLevelPatient, filter,
		func(transferSyntaxUID, sopClassUID, sopInstanceUID string, data []byte) dimse.Status {
			log.Printf("Got data: %v %v %v %d bytes", transferSyntaxUID, sopClassUID, sopInstanceUID, len(data))
			require.True(t, len(cgetData) == 0, "Received multiple C-GET responses")
			/* e := dicomio.NewBytesEncoder(nil, dicomio.UnknownVR)
			dicom.WriteFileHeader(e, */
			b := bytes.Buffer{}
			e := dicom.NewWriter(&b, dicom.SkipVRVerification())
			for _, elem := range []*dicom.Element{
				dicom.MustNewElement(tag.TransferSyntaxUID, transferSyntaxUID),
				dicom.MustNewElement(tag.MediaStorageSOPClassUID, sopClassUID),
				dicom.MustNewElement(tag.MediaStorageSOPInstanceUID, sopInstanceUID),
			} {
				e.WriteElement(elem)
			}
			//e.WriteBytes(data)
			cgetData = b.Bytes()
			return dimse.Success
		})
	require.NoError(t, err)
	require.True(t, len(cgetData) > 0, "No data received")
	ds, err := dicom.ReadDataSetInBytes(&cgetData, nil)
	require.NoError(t, err)
	expected := mustReadDICOMFile("testdata/reportsi.dcm")
	checkFileBodiesEqual(t, expected, &ds)
}

func TestReleaseWithoutConnect(t *testing.T) {
	su, err := NewServiceUser(ServiceUserParams{
		SOPClasses: sopclass.StorageClasses})
	require.NoError(t, err)
	su.Release()
}

func TestNonexistentServer(t *testing.T) {
	su, err := NewServiceUser(ServiceUserParams{
		SOPClasses: sopclass.StorageClasses})
	require.NoError(t, err)
	defer su.Release()
	su.Connect(":99999")
	err = su.CStore(mustReadDICOMFile("testdata/IM-0001-0003.dcm"))
	if err == nil || !strings.Contains(err.Error(), "Connection failed") {
		log.Panicf("Expect C-STORE to fail: %v", err)
	}
}

// TODO(saito) Test that the state machine shuts down properly.
