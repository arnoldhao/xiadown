//go:build windows

package equalizeraudio

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"xiadown/internal/application/equalizer"

	"golang.org/x/sys/windows"
)

const (
	processLoopbackMinimumBuild  = 20348
	processLoopbackSampleRate    = 44100
	processLoopbackChannels      = 2
	processLoopbackBitsPerSample = 16

	virtualAudioDeviceProcessLoopback = "VAD\\Process_Loopback"

	audioClientActivationTypeProcessLoopback    = 1
	processLoopbackModeIncludeTargetProcessTree = 0

	vtBlob = 65

	audioClientShareModeShared     = 0
	audioClientStreamFlagsLoopback = 0x00020000
	audioClientStreamFlagsEvent    = 0x00040000
	audioClientStreamFlagsAutoPCM  = 0x80000000
	audioClientBufferFlagsSilent   = 0x00000002
	waveFormatPCM                  = 1
	waitObject0                    = 0
	waitTimeout                    = 0x00000102
	activationTimeoutMilliseconds  = 5000
	capturePollMilliseconds        = 100
)

var (
	processLoopbackIIDIUnknown = windows.GUID{
		Data1: 0x00000000,
		Data2: 0x0000,
		Data3: 0x0000,
		Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
	}
	processLoopbackIIDIActivateAudioInterfaceCompletionHandler = windows.GUID{
		Data1: 0x41D949AB,
		Data2: 0x9862,
		Data3: 0x444A,
		Data4: [8]byte{0x80, 0xF6, 0xC2, 0x61, 0x33, 0x4D, 0xA5, 0xEB},
	}
	processLoopbackIIDIAgileObject = windows.GUID{
		Data1: 0x94EA2B94,
		Data2: 0xE9CC,
		Data3: 0x49E0,
		Data4: [8]byte{0xC0, 0xFF, 0xEE, 0x64, 0xCA, 0x8F, 0x5B, 0x90},
	}
	processLoopbackIIDIAudioClient = windows.GUID{
		Data1: 0x1CB9AD4C,
		Data2: 0xDBFA,
		Data3: 0x4C32,
		Data4: [8]byte{0xB1, 0x78, 0xC2, 0xF5, 0x68, 0xA7, 0x03, 0xB2},
	}
	processLoopbackIIDIAudioCaptureClient = windows.GUID{
		Data1: 0xC8ADBD64,
		Data2: 0xE71E,
		Data3: 0x48A0,
		Data4: [8]byte{0xA4, 0xDE, 0x18, 0x5C, 0x39, 0x5C, 0xD3, 0x17},
	}

	processLoopbackMMDevAPI                        = windows.NewLazySystemDLL("Mmdevapi.dll")
	processLoopbackProcActivateAudioInterfaceAsync = processLoopbackMMDevAPI.NewProc("ActivateAudioInterfaceAsync")
	processLoopbackNTDLL                           = windows.NewLazySystemDLL("ntdll.dll")
	processLoopbackProcRtlGetVersion               = processLoopbackNTDLL.NewProc("RtlGetVersion")
)

type Engine struct {
	mu sync.Mutex

	targetProcessProvider TargetProcessProvider
	capture               *processLoopbackCapture
	supportedOnce         sync.Once
	supported             bool
}

func NewEngine(options ...Option) *Engine {
	resolved := resolveOptions(options)
	return &Engine{targetProcessProvider: resolved.TargetProcessProvider}
}

func (engine *Engine) Features() equalizer.EngineFeatures {
	return equalizer.EngineFeatures{
		Equalizer:  false,
		Visualizer: true,
	}
}

func (engine *Engine) Supported() bool {
	if engine == nil {
		return false
	}
	engine.supportedOnce.Do(func() {
		if processLoopbackWindowsBuild() < processLoopbackMinimumBuild {
			return
		}
		engine.supported = processLoopbackProcActivateAudioInterfaceAsync.Find() == nil
	})
	return engine.supported
}

func (engine *Engine) IsRunning() bool {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	return engine.capture != nil && engine.capture.isRunning()
}

func (engine *Engine) HasObservedAudio() bool {
	engine.mu.Lock()
	capture := engine.capture
	engine.mu.Unlock()
	return capture != nil && capture.hasObservedAudio()
}

func (engine *Engine) VisualizerFrame() equalizer.VisualizerFrame {
	engine.mu.Lock()
	capture := engine.capture
	engine.mu.Unlock()
	if capture == nil {
		return equalizer.VisualizerFrame{
			Running:  false,
			Bands:    make([]float64, equalizer.VisualizerBandCount),
			Waveform: make([]float64, equalizer.VisualizerWaveformCount),
		}
	}
	return capture.frame()
}

func (engine *Engine) HasCapturePermission() bool {
	return true
}

func (engine *Engine) RequestCapturePermission() bool {
	return true
}

func (engine *Engine) Start(equalizer.Settings) *equalizer.StartFailure {
	if !engine.Supported() {
		return &equalizer.StartFailure{Code: equalizer.StartFailureUnsupported}
	}
	processID := engine.targetProcessID()
	if processID == 0 {
		return &equalizer.StartFailure{Code: equalizer.StartFailureNoAudioSource}
	}

	engine.mu.Lock()
	if engine.capture != nil && engine.capture.targetProcessID == processID && engine.capture.isRunning() {
		engine.mu.Unlock()
		return nil
	}
	previous := engine.capture
	engine.capture = nil
	engine.mu.Unlock()
	if previous != nil {
		previous.stop()
	}

	capture := newProcessLoopbackCapture(processID)
	if err := capture.start(); err != nil {
		return &equalizer.StartFailure{
			Code:   equalizer.StartFailureEngineStart,
			Detail: err.Error(),
		}
	}

	engine.mu.Lock()
	engine.capture = capture
	engine.mu.Unlock()
	return nil
}

func (engine *Engine) Apply(equalizer.Settings) {
	if engine == nil {
		return
	}
	processID := engine.targetProcessID()
	if processID == 0 {
		return
	}
	engine.mu.Lock()
	capture := engine.capture
	engine.mu.Unlock()
	if capture != nil && capture.targetProcessID != processID {
		engine.Stop()
	}
}

func (engine *Engine) Stop() {
	if engine == nil {
		return
	}
	engine.mu.Lock()
	capture := engine.capture
	engine.capture = nil
	engine.mu.Unlock()
	if capture != nil {
		capture.stop()
	}
}

func (engine *Engine) targetProcessID() uint32 {
	if engine == nil || engine.targetProcessProvider == nil {
		return 0
	}
	return engine.targetProcessProvider()
}

type processLoopbackCapture struct {
	targetProcessID uint32
	analyzer        *visualizerAnalyzer

	sampleReadyEvent windows.Handle
	audioClient      *windowsAudioClient
	captureClient    *windowsAudioCaptureClient

	running atomic.Bool
	stopped atomic.Bool
	done    chan struct{}
}

func newProcessLoopbackCapture(targetProcessID uint32) *processLoopbackCapture {
	return &processLoopbackCapture{
		targetProcessID: targetProcessID,
		analyzer:        newVisualizerAnalyzer(processLoopbackSampleRate),
	}
}

func (capture *processLoopbackCapture) start() error {
	started := make(chan error, 1)
	capture.done = make(chan struct{})
	go capture.run(started)
	return <-started
}

func (capture *processLoopbackCapture) run(started chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(capture.done)
	defer capture.cleanup()

	coinitializeErr := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED)
	coinitialized := coinitializeErr == nil || isHResult(coinitializeErr, 1)
	if !coinitialized {
		started <- fmt.Errorf("CoInitializeEx: %w", coinitializeErr)
		return
	}
	defer windows.CoUninitialize()

	if err := capture.activate(); err != nil {
		started <- err
		return
	}
	if err := capture.audioClient.start(); err != nil {
		started <- fmt.Errorf("IAudioClient.Start: %w", err)
		return
	}

	capture.running.Store(true)
	capture.analyzer.setRunning(true)
	started <- nil

	capture.readLoop()
	_ = capture.audioClient.stop()
	capture.running.Store(false)
	capture.analyzer.setRunning(false)
}

func (capture *processLoopbackCapture) activate() error {
	event, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return fmt.Errorf("CreateEvent: %w", err)
	}
	capture.sampleReadyEvent = event

	completedEvent, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return fmt.Errorf("CreateEvent activation: %w", err)
	}
	defer windows.CloseHandle(completedEvent)

	handler := newActivateAudioInterfaceCompletionHandler(capture, completedEvent)
	defer handler.release()
	defer runtime.KeepAlive(handler)
	devicePath, err := windows.UTF16PtrFromString(virtualAudioDeviceProcessLoopback)
	if err != nil {
		return err
	}
	activationParams := audioClientActivationParams{
		ActivationType: audioClientActivationTypeProcessLoopback,
		ProcessLoopbackParams: audioClientProcessLoopbackParams{
			TargetProcessID:     capture.targetProcessID,
			ProcessLoopbackMode: processLoopbackModeIncludeTargetProcessTree,
		},
	}
	propVariant := propVariantBlob{
		VT:       vtBlob,
		BlobSize: uint32(unsafe.Sizeof(activationParams)),
		BlobData: uintptr(unsafe.Pointer(&activationParams)),
	}

	var asyncOperation *activateAudioInterfaceAsyncOperation
	hr, _, _ := processLoopbackProcActivateAudioInterfaceAsync.Call(
		uintptr(unsafe.Pointer(devicePath)),
		uintptr(unsafe.Pointer(&processLoopbackIIDIAudioClient)),
		uintptr(unsafe.Pointer(&propVariant)),
		uintptr(unsafe.Pointer(handler)),
		uintptr(unsafe.Pointer(&asyncOperation)),
	)
	if hresultFailed(hr) {
		return hresultError("ActivateAudioInterfaceAsync", hr)
	}
	if asyncOperation != nil {
		defer asyncOperation.release()
	}

	waitResult, err := windows.WaitForSingleObject(completedEvent, activationTimeoutMilliseconds)
	if err != nil {
		return fmt.Errorf("wait activation: %w", err)
	}
	if waitResult == waitTimeout {
		return fmt.Errorf("ActivateAudioInterfaceAsync timed out")
	}
	if waitResult != waitObject0 {
		return fmt.Errorf("unexpected activation wait result 0x%X", waitResult)
	}
	if handler.activateResult != nil {
		return handler.activateResult
	}
	return nil
}

func (capture *processLoopbackCapture) readLoop() {
	for !capture.stopped.Load() {
		waitResult, err := windows.WaitForSingleObject(capture.sampleReadyEvent, capturePollMilliseconds)
		if err != nil || waitResult == waitTimeout {
			continue
		}
		if waitResult != waitObject0 {
			continue
		}
		if err := capture.drainPackets(); err != nil {
			return
		}
	}
}

func (capture *processLoopbackCapture) drainPackets() error {
	for !capture.stopped.Load() {
		packetFrames, err := capture.captureClient.nextPacketSize()
		if err != nil {
			return err
		}
		if packetFrames == 0 {
			return nil
		}

		data, frames, flags, err := capture.captureClient.getBuffer()
		if err != nil {
			return err
		}
		if frames > 0 {
			if flags&audioClientBufferFlagsSilent != 0 || data == nil {
				capture.analyzer.analyzeSilence(int(frames))
			} else {
				sampleCount := int(frames) * processLoopbackChannels
				samples := unsafe.Slice((*int16)(unsafe.Pointer(data)), sampleCount)
				capture.analyzer.analyzePCM16(samples, processLoopbackChannels, int(frames))
			}
		}
		if err := capture.captureClient.releaseBuffer(frames); err != nil {
			return err
		}
	}
	return nil
}

func (capture *processLoopbackCapture) stop() {
	if capture == nil {
		return
	}
	capture.stopped.Store(true)
	if capture.sampleReadyEvent != 0 {
		_ = windows.SetEvent(capture.sampleReadyEvent)
	}
	if capture.done == nil {
		return
	}
	select {
	case <-capture.done:
	case <-time.After(2 * time.Second):
	}
}

func (capture *processLoopbackCapture) cleanup() {
	if capture.captureClient != nil {
		capture.captureClient.release()
		capture.captureClient = nil
	}
	if capture.audioClient != nil {
		capture.audioClient.release()
		capture.audioClient = nil
	}
	if capture.sampleReadyEvent != 0 {
		_ = windows.CloseHandle(capture.sampleReadyEvent)
		capture.sampleReadyEvent = 0
	}
}

func (capture *processLoopbackCapture) isRunning() bool {
	return capture != nil && capture.running.Load()
}

func (capture *processLoopbackCapture) hasObservedAudio() bool {
	return capture != nil && capture.analyzer.hasObservedAudio()
}

func (capture *processLoopbackCapture) frame() equalizer.VisualizerFrame {
	if capture == nil {
		return equalizer.VisualizerFrame{
			Running:  false,
			Bands:    make([]float64, equalizer.VisualizerBandCount),
			Waveform: make([]float64, equalizer.VisualizerWaveformCount),
		}
	}
	return capture.analyzer.frame()
}

type audioClientActivationParams struct {
	ActivationType        uint32
	ProcessLoopbackParams audioClientProcessLoopbackParams
}

type audioClientProcessLoopbackParams struct {
	TargetProcessID     uint32
	ProcessLoopbackMode uint32
}

type propVariantBlob struct {
	VT        uint16
	Reserved1 uint16
	Reserved2 uint16
	Reserved3 uint16
	BlobSize  uint32
	BlobData  uintptr
}

type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	Size           uint16
}

func processLoopbackCaptureFormat() waveFormatEx {
	blockAlign := uint16(processLoopbackChannels * processLoopbackBitsPerSample / 8)
	return waveFormatEx{
		FormatTag:      waveFormatPCM,
		Channels:       processLoopbackChannels,
		SamplesPerSec:  processLoopbackSampleRate,
		AvgBytesPerSec: processLoopbackSampleRate * uint32(blockAlign),
		BlockAlign:     blockAlign,
		BitsPerSample:  processLoopbackBitsPerSample,
	}
}

type windowsCOMProc uintptr

func newWindowsCOMProc(fn any) windowsCOMProc {
	return windowsCOMProc(windows.NewCallback(fn))
}

func (proc windowsCOMProc) call(args ...uintptr) (uintptr, uintptr, error) {
	return syscall.SyscallN(uintptr(proc), args...)
}

type windowsIUnknownVtbl struct {
	QueryInterface windowsCOMProc
	AddRef         windowsCOMProc
	Release        windowsCOMProc
}

type activateAudioInterfaceCompletionHandlerVtbl struct {
	windowsIUnknownVtbl
	ActivateCompleted windowsCOMProc
}

type activateAudioInterfaceCompletionHandler struct {
	Vtbl           *activateAudioInterfaceCompletionHandlerVtbl
	references     int32
	capture        *processLoopbackCapture
	completedEvent windows.Handle
	activateResult error
}

var processLoopbackCompletionHandlerVtbl = activateAudioInterfaceCompletionHandlerVtbl{
	windowsIUnknownVtbl: windowsIUnknownVtbl{
		QueryInterface: newWindowsCOMProc(activateAudioInterfaceCompletionHandlerQueryInterface),
		AddRef:         newWindowsCOMProc(activateAudioInterfaceCompletionHandlerAddRef),
		Release:        newWindowsCOMProc(activateAudioInterfaceCompletionHandlerRelease),
	},
	ActivateCompleted: newWindowsCOMProc(activateAudioInterfaceCompletionHandlerActivateCompleted),
}

func newActivateAudioInterfaceCompletionHandler(
	capture *processLoopbackCapture,
	completedEvent windows.Handle,
) *activateAudioInterfaceCompletionHandler {
	return &activateAudioInterfaceCompletionHandler{
		Vtbl:           &processLoopbackCompletionHandlerVtbl,
		references:     1,
		capture:        capture,
		completedEvent: completedEvent,
	}
}

func activateAudioInterfaceCompletionHandlerQueryInterface(this uintptr, refiid uintptr, object uintptr) uintptr {
	if object == 0 {
		return 0x80004003
	}
	*(*uintptr)(unsafe.Pointer(object)) = 0
	if refiid == 0 {
		return 0x80004002
	}
	guid := (*windows.GUID)(unsafe.Pointer(refiid))
	if *guid == processLoopbackIIDIUnknown ||
		*guid == processLoopbackIIDIActivateAudioInterfaceCompletionHandler ||
		*guid == processLoopbackIIDIAgileObject {
		*(*uintptr)(unsafe.Pointer(object)) = this
		activateAudioInterfaceCompletionHandlerAddRef(this)
		return 0
	}
	return 0x80004002
}

func activateAudioInterfaceCompletionHandlerAddRef(this uintptr) uintptr {
	handler := (*activateAudioInterfaceCompletionHandler)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&handler.references, 1))
}

func activateAudioInterfaceCompletionHandlerRelease(this uintptr) uintptr {
	handler := (*activateAudioInterfaceCompletionHandler)(unsafe.Pointer(this))
	next := atomic.AddInt32(&handler.references, -1)
	if next < 0 {
		return 0
	}
	return uintptr(next)
}

func (handler *activateAudioInterfaceCompletionHandler) release() {
	if handler != nil {
		activateAudioInterfaceCompletionHandlerRelease(uintptr(unsafe.Pointer(handler)))
	}
}

func activateAudioInterfaceCompletionHandlerActivateCompleted(this uintptr, operation uintptr) uintptr {
	handler := (*activateAudioInterfaceCompletionHandler)(unsafe.Pointer(this))
	handler.activateResult = handler.finishActivation((*activateAudioInterfaceAsyncOperation)(unsafe.Pointer(operation)))
	_ = windows.SetEvent(handler.completedEvent)
	return 0
}

func (handler *activateAudioInterfaceCompletionHandler) finishActivation(
	operation *activateAudioInterfaceAsyncOperation,
) error {
	if handler == nil || handler.capture == nil {
		return fmt.Errorf("activation handler unavailable")
	}
	if operation == nil {
		return fmt.Errorf("activation operation unavailable")
	}
	activateResult, activatedInterface, err := operation.getActivateResult()
	if err != nil {
		return err
	}
	if hresultFailed(activateResult) {
		return hresultError("IActivateAudioInterfaceAsyncOperation.GetActivateResult", activateResult)
	}
	if activatedInterface == 0 {
		return fmt.Errorf("activation returned no IAudioClient")
	}

	audioClient := (*windowsAudioClient)(unsafe.Pointer(activatedInterface))
	captureFormat := processLoopbackCaptureFormat()
	flags := uint32(audioClientStreamFlagsLoopback | audioClientStreamFlagsEvent | audioClientStreamFlagsAutoPCM)
	if err := audioClient.initialize(audioClientShareModeShared, flags, 0, 0, &captureFormat); err != nil {
		audioClient.release()
		return fmt.Errorf("IAudioClient.Initialize: %w", err)
	}
	var captureClient *windowsAudioCaptureClient
	if err := audioClient.getService(&processLoopbackIIDIAudioCaptureClient, unsafe.Pointer(&captureClient)); err != nil {
		audioClient.release()
		return fmt.Errorf("IAudioClient.GetService: %w", err)
	}
	if err := audioClient.setEventHandle(handler.capture.sampleReadyEvent); err != nil {
		captureClient.release()
		audioClient.release()
		return fmt.Errorf("IAudioClient.SetEventHandle: %w", err)
	}

	handler.capture.audioClient = audioClient
	handler.capture.captureClient = captureClient
	return nil
}

type activateAudioInterfaceAsyncOperationVtbl struct {
	windowsIUnknownVtbl
	GetActivateResult windowsCOMProc
}

type activateAudioInterfaceAsyncOperation struct {
	Vtbl *activateAudioInterfaceAsyncOperationVtbl
}

func (operation *activateAudioInterfaceAsyncOperation) getActivateResult() (uintptr, uintptr, error) {
	var activateResult uintptr
	var activatedInterface uintptr
	hr, _, _ := operation.Vtbl.GetActivateResult.call(
		uintptr(unsafe.Pointer(operation)),
		uintptr(unsafe.Pointer(&activateResult)),
		uintptr(unsafe.Pointer(&activatedInterface)),
	)
	if hresultFailed(hr) {
		return 0, 0, hresultError("IActivateAudioInterfaceAsyncOperation.GetActivateResult", hr)
	}
	return activateResult, activatedInterface, nil
}

func (operation *activateAudioInterfaceAsyncOperation) release() {
	if operation != nil {
		operation.Vtbl.Release.call(uintptr(unsafe.Pointer(operation)))
	}
}

type windowsAudioClientVtbl struct {
	windowsIUnknownVtbl
	Initialize        windowsCOMProc
	GetBufferSize     windowsCOMProc
	GetStreamLatency  windowsCOMProc
	GetCurrentPadding windowsCOMProc
	IsFormatSupported windowsCOMProc
	GetMixFormat      windowsCOMProc
	GetDevicePeriod   windowsCOMProc
	Start             windowsCOMProc
	Stop              windowsCOMProc
	Reset             windowsCOMProc
	SetEventHandle    windowsCOMProc
	GetService        windowsCOMProc
}

type windowsAudioClient struct {
	Vtbl *windowsAudioClientVtbl
}

func (client *windowsAudioClient) initialize(
	shareMode uint32,
	streamFlags uint32,
	bufferDuration uint64,
	periodicity uint64,
	format *waveFormatEx,
) error {
	hr, _, _ := client.Vtbl.Initialize.call(
		uintptr(unsafe.Pointer(client)),
		uintptr(shareMode),
		uintptr(streamFlags),
		uintptr(bufferDuration),
		uintptr(periodicity),
		uintptr(unsafe.Pointer(format)),
		0,
	)
	if hresultFailed(hr) {
		return hresultError("IAudioClient.Initialize", hr)
	}
	return nil
}

func (client *windowsAudioClient) getService(iid *windows.GUID, object unsafe.Pointer) error {
	hr, _, _ := client.Vtbl.GetService.call(
		uintptr(unsafe.Pointer(client)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(object),
	)
	if hresultFailed(hr) {
		return hresultError("IAudioClient.GetService", hr)
	}
	return nil
}

func (client *windowsAudioClient) setEventHandle(event windows.Handle) error {
	hr, _, _ := client.Vtbl.SetEventHandle.call(uintptr(unsafe.Pointer(client)), uintptr(event))
	if hresultFailed(hr) {
		return hresultError("IAudioClient.SetEventHandle", hr)
	}
	return nil
}

func (client *windowsAudioClient) start() error {
	hr, _, _ := client.Vtbl.Start.call(uintptr(unsafe.Pointer(client)))
	if hresultFailed(hr) {
		return hresultError("IAudioClient.Start", hr)
	}
	return nil
}

func (client *windowsAudioClient) stop() error {
	hr, _, _ := client.Vtbl.Stop.call(uintptr(unsafe.Pointer(client)))
	if hresultFailed(hr) {
		return hresultError("IAudioClient.Stop", hr)
	}
	return nil
}

func (client *windowsAudioClient) release() {
	if client != nil {
		client.Vtbl.Release.call(uintptr(unsafe.Pointer(client)))
	}
}

type windowsAudioCaptureClientVtbl struct {
	windowsIUnknownVtbl
	GetBuffer         windowsCOMProc
	ReleaseBuffer     windowsCOMProc
	GetNextPacketSize windowsCOMProc
}

type windowsAudioCaptureClient struct {
	Vtbl *windowsAudioCaptureClientVtbl
}

func (client *windowsAudioCaptureClient) getBuffer() (*byte, uint32, uint32, error) {
	var data *byte
	var frames uint32
	var flags uint32
	var devicePosition uint64
	var qpcPosition uint64
	hr, _, _ := client.Vtbl.GetBuffer.call(
		uintptr(unsafe.Pointer(client)),
		uintptr(unsafe.Pointer(&data)),
		uintptr(unsafe.Pointer(&frames)),
		uintptr(unsafe.Pointer(&flags)),
		uintptr(unsafe.Pointer(&devicePosition)),
		uintptr(unsafe.Pointer(&qpcPosition)),
	)
	if hresultFailed(hr) {
		return nil, 0, 0, hresultError("IAudioCaptureClient.GetBuffer", hr)
	}
	return data, frames, flags, nil
}

func (client *windowsAudioCaptureClient) releaseBuffer(frames uint32) error {
	hr, _, _ := client.Vtbl.ReleaseBuffer.call(uintptr(unsafe.Pointer(client)), uintptr(frames))
	if hresultFailed(hr) {
		return hresultError("IAudioCaptureClient.ReleaseBuffer", hr)
	}
	return nil
}

func (client *windowsAudioCaptureClient) nextPacketSize() (uint32, error) {
	var frames uint32
	hr, _, _ := client.Vtbl.GetNextPacketSize.call(
		uintptr(unsafe.Pointer(client)),
		uintptr(unsafe.Pointer(&frames)),
	)
	if hresultFailed(hr) {
		return 0, hresultError("IAudioCaptureClient.GetNextPacketSize", hr)
	}
	return frames, nil
}

func (client *windowsAudioCaptureClient) release() {
	if client != nil {
		client.Vtbl.Release.call(uintptr(unsafe.Pointer(client)))
	}
}

type rtlOSVersionInfoEx struct {
	OSVersionInfoSize uint32
	MajorVersion      uint32
	MinorVersion      uint32
	BuildNumber       uint32
	PlatformID        uint32
	CSDVersion        [128]uint16
	ServicePackMajor  uint16
	ServicePackMinor  uint16
	SuiteMask         uint16
	ProductType       byte
	Reserved          byte
}

func processLoopbackWindowsBuild() uint32 {
	var info rtlOSVersionInfoEx
	info.OSVersionInfoSize = uint32(unsafe.Sizeof(info))
	hr, _, _ := processLoopbackProcRtlGetVersion.Call(uintptr(unsafe.Pointer(&info)))
	if hresultFailed(hr) {
		return 0
	}
	return info.BuildNumber
}

func hresultFailed(hr uintptr) bool {
	return uint32(hr)&0x80000000 != 0
}

func hresultError(operation string, hr uintptr) error {
	return fmt.Errorf("%s HRESULT 0x%08X", operation, uint32(hr))
}

func isHResult(err error, hr uintptr) bool {
	if err == nil {
		return hr == 0
	}
	errno, ok := err.(syscall.Errno)
	return ok && uintptr(errno) == hr
}
