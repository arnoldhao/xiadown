//go:build windows

package discovery

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	dnsQueryRequestVersion1 = 1
	dnsRequestPending       = 9506
)

var (
	dnsapi                = windows.NewLazySystemDLL("dnsapi.dll")
	procConstructService  = dnsapi.NewProc("DnsServiceConstructInstance")
	procFreeService       = dnsapi.NewProc("DnsServiceFreeInstance")
	procRegisterService   = dnsapi.NewProc("DnsServiceRegister")
	procCancelRegister    = dnsapi.NewProc("DnsServiceRegisterCancel")
	procDeregisterService = dnsapi.NewProc("DnsServiceDeRegister")
)

// DNS_SERVICE_CANCEL is an opaque pointer-sized handle populated by
// DnsServiceRegister. It must remain alive until the asynchronous operation
// completes or DnsServiceRegisterCancel finishes.
type dnsServiceCancel struct{ reserved uintptr }

type dnsServiceRegisterRequest struct {
	Version            uint32
	InterfaceIndex     uint32
	ServiceInstance    uintptr
	CompletionCallback uintptr
	QueryContext       uintptr
	Credentials        windows.Handle
	UnicastEnabled     int32
}

type windowsRegistration struct {
	request  *dnsServiceRegisterRequest
	instance uintptr
	keys     []*uint16
	values   []*uint16
	events   chan uint32
	once     sync.Once
}

type windowsAdvertiser struct{}

func newPlatformAdvertiser() Advertiser { return windowsAdvertiser{} }

func freeWindowsDNSServiceInstance(instance uintptr) {
	procFreeService.Call(instance)
}

func (windowsAdvertiser) Register(ctx context.Context, value Advertisement) (Registration, error) {
	value, err := ValidateAdvertisement(value)
	if err != nil {
		return nil, err
	}
	indices, err := advertisementInterfaceIndices(value)
	if err != nil {
		return nil, err
	}
	group := &registrationGroup{}
	for _, index := range indices {
		registration, err := registerWindowsInterface(value, index)
		if err != nil {
			_ = group.Close()
			return nil, err
		}
		group.closeFuncs = append(group.closeFuncs, registration.Close)
	}
	go func() {
		<-ctx.Done()
		_ = group.Close()
	}()
	return group, nil
}

func registerWindowsInterface(value Advertisement, interfaceIndex int) (*windowsRegistration, error) {
	serviceName, err := windows.UTF16PtrFromString(value.Name + "." + ServiceType + ".local")
	if err != nil {
		return nil, err
	}
	systemHostName, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("resolve Windows DNS-SD host name: %w", err)
	}
	dnsHostName, err := windowsDNSServiceHostName(systemHostName)
	if err != nil {
		return nil, err
	}
	nativeHostName, err := windows.UTF16PtrFromString(dnsHostName)
	if err != nil {
		return nil, err
	}
	records := TXTRecords()
	keys := make([]*uint16, len(records))
	values := make([]*uint16, len(records))
	for index, record := range records {
		separator := -1
		for offset, character := range record {
			if character == '=' {
				separator = offset
				break
			}
		}
		if separator < 1 {
			return nil, fmt.Errorf("invalid DNS-SD TXT record")
		}
		keys[index], err = windows.UTF16PtrFromString(record[:separator])
		if err != nil {
			return nil, err
		}
		values[index], err = windows.UTF16PtrFromString(record[separator+1:])
		if err != nil {
			return nil, err
		}
	}
	var keysPointer, valuesPointer uintptr
	if len(keys) > 0 {
		keysPointer = uintptr(unsafe.Pointer(&keys[0]))
		valuesPointer = uintptr(unsafe.Pointer(&values[0]))
	}
	instance, _, _ := procConstructService.Call(
		uintptr(unsafe.Pointer(serviceName)), uintptr(unsafe.Pointer(nativeHostName)), 0, 0,
		uintptr(uint16(value.Port)), 0, 0,
		uintptr(len(keys)), keysPointer, valuesPointer,
	)
	runtime.KeepAlive(serviceName)
	runtime.KeepAlive(nativeHostName)
	runtime.KeepAlive(keys)
	runtime.KeepAlive(values)
	if instance == 0 {
		// Unlike DnsServiceRegister, DnsServiceConstructInstance does not
		// document GetLastError as part of its return contract. Its pointer is
		// authoritative; wrapping LazyProc.Call's errno can report stale
		// ERROR_SUCCESS as though it caused the failure.
		return nil, fmt.Errorf(
			"construct Windows DNS-SD instance for host %q: native constructor returned null",
			dnsHostName,
		)
	}
	events := make(chan uint32, 2)
	callback := syscall.NewCallback(func(status, _, callbackInstance uintptr) uintptr {
		// DNS_SERVICE_REGISTER_COMPLETE transfers ownership of every non-null
		// callback instance to us. Release that system copy before publishing
		// the status so even a full channel or a late callback cannot leak it.
		releaseWindowsDNSSDCallbackInstance(callbackInstance, freeWindowsDNSServiceInstance)
		select {
		case events <- uint32(status):
		default:
		}
		return 0
	})
	request := &dnsServiceRegisterRequest{
		Version: dnsQueryRequestVersion1, InterfaceIndex: uint32(interfaceIndex),
		ServiceInstance: instance, CompletionCallback: callback,
	}
	cancelRequest := new(dnsServiceCancel)
	defer func() { runtime.KeepAlive(cancelRequest) }()
	status, _, callErr := procRegisterService.Call(
		uintptr(unsafe.Pointer(request)), uintptr(unsafe.Pointer(cancelRequest)),
	)
	if status != dnsRequestPending {
		procFreeService.Call(instance)
		return nil, windowsDNSSDNativeCallError(
			fmt.Sprintf("register Windows DNS-SD on interface %d: status %d", interfaceIndex, status),
			callErr,
		)
	}
	select {
	case completionStatus := <-events:
		if completionStatus != 0 {
			procFreeService.Call(instance)
			return nil, fmt.Errorf("complete Windows DNS-SD registration on interface %d: status %d", interfaceIndex, completionStatus)
		}
	case <-time.After(5 * time.Second):
		// A timed-out asynchronous registration must be cancelled. Otherwise it
		// can complete after LAN setup has rolled back and leave a stale mDNS
		// advertisement pointing at a closed listener until process exit.
		cancelStatus, _, cancelErr := procCancelRegister.Call(uintptr(unsafe.Pointer(cancelRequest)))
		select {
		case completionStatus := <-events:
			if completionStatus == 0 {
				// Registration won the race with cancellation. Remove the now-live
				// service before returning the timeout to the caller.
				deregisterStatus, _, deregisterErr := procDeregisterService.Call(uintptr(unsafe.Pointer(request)), 0)
				if deregisterStatus == dnsRequestPending {
					select {
					case <-events:
					case <-time.After(5 * time.Second):
						return nil, fmt.Errorf("clean timed-out Windows DNS-SD registration on interface %d: timeout", interfaceIndex)
					}
				} else {
					return nil, windowsDNSSDNativeCallError(
						fmt.Sprintf("clean timed-out Windows DNS-SD registration on interface %d: status %d", interfaceIndex, deregisterStatus),
						deregisterErr,
					)
				}
			}
			procFreeService.Call(instance)
		case <-time.After(5 * time.Second):
			// If cancellation succeeded, Windows owns no discoverable service.
			// Keep the instance allocated because a late callback may still refer
			// to it; process teardown will reclaim the small allocation safely.
			if cancelStatus != 0 {
				return nil, windowsDNSSDNativeCallError(
					fmt.Sprintf("cancel timed-out Windows DNS-SD registration on interface %d: status %d", interfaceIndex, cancelStatus),
					cancelErr,
				)
			}
		}
		return nil, fmt.Errorf("complete Windows DNS-SD registration on interface %d: timeout", interfaceIndex)
	}
	registration := &windowsRegistration{request: request, instance: instance, keys: keys, values: values, events: events}
	runtime.KeepAlive(serviceName)
	runtime.KeepAlive(nativeHostName)
	runtime.KeepAlive(keys)
	runtime.KeepAlive(values)
	return registration, nil
}

func (registration *windowsRegistration) Close() (result error) {
	registration.once.Do(func() {
		for {
			select {
			case <-registration.events:
				continue
			default:
				goto drained
			}
		}
	drained:
		status, _, callErr := procDeregisterService.Call(uintptr(unsafe.Pointer(registration.request)), 0)
		if status != dnsRequestPending {
			result = windowsDNSSDNativeCallError(
				fmt.Sprintf("deregister Windows DNS-SD: status %d", status),
				callErr,
			)
			return
		}
		select {
		case completionStatus := <-registration.events:
			if completionStatus != 0 {
				result = fmt.Errorf("complete Windows DNS-SD deregistration: status %d", completionStatus)
			}
		case <-time.After(5 * time.Second):
			result = fmt.Errorf("complete Windows DNS-SD deregistration: timeout")
			return
		}
		procFreeService.Call(registration.instance)
		registration.instance = 0
		runtime.KeepAlive(registration.keys)
		runtime.KeepAlive(registration.values)
	})
	return result
}
