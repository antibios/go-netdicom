module github.com/antibios/go-netdicom

go 1.20

replace github.com/antibios/dicom => ../dicom

replace github.com/antibios/go-dicom => ../go-dicom

replace github.com/antibios/dicom/dicomlog => ../go-dicom/dicomlog

replace github.com/suyashkumar/dicom => ../dicom

require (
	github.com/antibios/dicom v0.0.0-00010101000000-000000000000
	github.com/antibios/go-dicom v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.8.4
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/text v0.3.8 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
