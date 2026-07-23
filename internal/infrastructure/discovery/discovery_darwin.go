//go:build darwin

package discovery

/*
#include <dns_sd.h>
#include <arpa/inet.h>
#include <stdlib.h>

static DNSServiceErrorType xiadown_register(
    DNSServiceRef *ref,
    uint32_t interfaceIndex,
    const char *name,
    uint16_t port,
    uint16_t txtLen,
    const void *txtRecord) {
  return DNSServiceRegister(ref, 0, interfaceIndex, name, "_xiadown._tcp",
    "local.", NULL, htons(port), txtLen, txtRecord, NULL, NULL);
}
*/
import "C"

import (
	"context"
	"fmt"
	"sync"
	"unsafe"
)

type darwinAdvertiser struct{}

func newPlatformAdvertiser() Advertiser { return darwinAdvertiser{} }

func (darwinAdvertiser) Register(ctx context.Context, value Advertisement) (Registration, error) {
	value, err := ValidateAdvertisement(value)
	if err != nil {
		return nil, err
	}
	indices, err := advertisementInterfaceIndices(value)
	if err != nil {
		return nil, err
	}
	name := C.CString(value.Name)
	defer C.free(unsafe.Pointer(name))
	txt := encodeTXT(TXTRecords())
	var txtPointer unsafe.Pointer
	if len(txt) > 0 {
		txtPointer = C.CBytes(txt)
		defer C.free(txtPointer)
	}
	group := &registrationGroup{}
	for _, index := range indices {
		var ref C.DNSServiceRef
		status := C.xiadown_register(&ref, C.uint32_t(index), name, C.uint16_t(value.Port), C.uint16_t(len(txt)), txtPointer)
		if status != C.kDNSServiceErr_NoError {
			_ = group.Close()
			return nil, fmt.Errorf("register DNS-SD on interface %d: status %d", index, int(status))
		}
		var once sync.Once
		group.closeFuncs = append(group.closeFuncs, func() error {
			once.Do(func() { C.DNSServiceRefDeallocate(ref) })
			return nil
		})
	}
	go func() {
		<-ctx.Done()
		_ = group.Close()
	}()
	return group, nil
}

func encodeTXT(records []string) []byte {
	result := make([]byte, 0, 64)
	for _, record := range records {
		if len(record) == 0 || len(record) > 255 {
			continue
		}
		result = append(result, byte(len(record)))
		result = append(result, record...)
	}
	return result
}
