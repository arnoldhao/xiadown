package wails

import (
	"reflect"
	"strings"
	"testing"
)

func TestLibraryImportSelectionRequestCannotSubmitLocalPaths(t *testing.T) {
	typeOfRequest := reflect.TypeOf(SelectLibraryImportRequest{})
	for index := 0; index < typeOfRequest.NumField(); index++ {
		field := typeOfRequest.Field(index)
		name := strings.ToLower(field.Name + " " + field.Tag.Get("json"))
		if strings.Contains(name, "path") || strings.Contains(name, "root") || strings.Contains(name, "source") {
			t.Fatalf("path-bearing field %q must not cross the Wails request boundary", field.Name)
		}
	}
}

func TestLibraryImportHandlerUsesDedicatedServiceName(t *testing.T) {
	handler := NewLibraryImportHandler(nil, nil)
	if got := handler.ServiceName(); got != "LibraryImportHandler" {
		t.Fatalf("unexpected service name %q", got)
	}
}
