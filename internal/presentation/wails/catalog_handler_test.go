package wails

import (
	"reflect"
	"testing"
)

func TestSelectCatalogStorageRootRequestCannotCarryAPath(t *testing.T) {
	typeOfRequest := reflect.TypeOf(SelectCatalogStorageRootRequest{})
	if typeOfRequest.NumField() != 2 {
		t.Fatalf("storage root picker request fields = %d, want name and mode only", typeOfRequest.NumField())
	}
	for index := 0; index < typeOfRequest.NumField(); index++ {
		field := typeOfRequest.Field(index)
		if field.Name == "Path" || field.Tag.Get("json") == "path" {
			t.Fatalf("webview request unexpectedly accepts a local path: %#v", field)
		}
	}
}
