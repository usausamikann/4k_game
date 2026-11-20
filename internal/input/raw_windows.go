//go:build windows

package input

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/usausamikann/4k_game/internal/chart"
	"github.com/usausamikann/4k_game/internal/judge"
	"golang.org/x/sys/windows"
)

const (
	wmDestroy = 0x0002
	wmClose   = 0x0010
	wmInput   = 0x00FF

	hwndMessage = ^uintptr(2) // (HWND)-3

	ridInput        = 0x10000003
	rimTypeKeyboard = 1

	ridevRemove    = 0x00000001
	ridevInputSink = 0x00000100

	riKeyBreak = 0x0001
	riKeyE0    = 0x0002
	riKeyE1    = 0x0004

	hidUsagePageGeneric     = 0x01
	hidUsageGenericKeyboard = 0x06
	overrunMakeCode         = 0xFF
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetModuleHandleW        = kernel32.NewProc("GetModuleHandleW")
	procRegisterClassExW        = user32.NewProc("RegisterClassExW")
	procUnregisterClassW        = user32.NewProc("UnregisterClassW")
	procCreateWindowExW         = user32.NewProc("CreateWindowExW")
	procDestroyWindow           = user32.NewProc("DestroyWindow")
	procDefWindowProcW          = user32.NewProc("DefWindowProcW")
	procGetMessageW             = user32.NewProc("GetMessageW")
	procDispatchMessageW        = user32.NewProc("DispatchMessageW")
	procPostMessageW            = user32.NewProc("PostMessageW")
	procPostQuitMessage         = user32.NewProc("PostQuitMessage")
	procRegisterRawInputDevices = user32.NewProc("RegisterRawInputDevices")
	procGetRawInputData         = user32.NewProc("GetRawInputData")
)

type wndClassExW struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type winMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	PtX     int32
	PtY     int32
	Private uint32 // MSG.lPrivate must exist on both 32- and 64-bit Windows
}

type rawInputDevice struct {
	UsagePage uint16
	Usage     uint16
	Flags     uint32
	Target    uintptr
}

type rawInputHeader struct {
	Type   uint32
	Size   uint32
	Device uintptr
	WParam uintptr
}

type rawKeyboard struct {
	MakeCode         uint16
	Flags            uint16
	Reserved         uint16
	VKey             uint16
	Message          uint32
	ExtraInformation uint32
}

type rawInputKeyboard struct {
	Header   rawInputHeader
	Keyboard rawKeyboard
}

var (
	rawWndProcOnce sync.Once
	rawWndProcPtr  uintptr
	rawClassSeq    atomic.Uint64
	rawReaders     sync.Map // map[uintptr]*RawReader
)

// RawReader listens for Windows WM_INPUT keyboard events on a dedicated OS
// thread and stamps each Make/Break transition with time.Now() on receipt.
type RawReader struct {
	scanCodes [chart.LaneCount]uint16
	queue     rawQueue
	hwnd      atomic.Uintptr
	closeOnce sync.Once
	readErrors atomic.Uint64
	done chan struct{}
}

func NewRawReader(scanCodes [chart.LaneCount]uint16) (*RawReader, error) {
	for lane, code := range scanCodes {
		if code == 0 {
			return nil, fmt.Errorf("missing scancode for lane %d", lane)
		}
	}
	rawWndProcOnce.Do(func() {
		rawWndProcPtr = syscall.NewCallback(rawWndProc)
	})
	r := &RawReader{
		scanCodes: scanCodes,
		done:      make(chan struct{}),
	}
	ready := make(chan error, 1)
	go r.run(ready)
	if err := <-ready; err != nil {
		<-r.done
		return nil, err
	}
	return r, nil
}

func (r *RawReader) DrainUpTo(limit time.Time, dst []RawEvent) ([]RawEvent, uint64, [chart.LaneCount]bool) {
	return r.queue.drainUpTo(limit, dst)
}

func (r *RawReader) FailedReads() uint64 { return r.readErrors.Load() }

func (r *RawReader) Close() error {
	r.closeOnce.Do(func() {
		if hwnd := r.hwnd.Load(); hwnd != 0 {
			procPostMessageW.Call(hwnd, wmClose, 0, 0)
		}
		<-r.done
	})
	return nil
}

func (r *RawReader) run(ready chan<- error) {
	defer close(r.done)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	instance, _, _ := procGetModuleHandleW.Call(0)
	classNameStr := fmt.Sprintf("4k_game_raw_input_%d", rawClassSeq.Add(1))
	className, err := windows.UTF16PtrFromString(classNameStr)
	if err != nil {
		ready <- err
		return
	}
	wc := wndClassExW{
		Size:      uint32(unsafe.Sizeof(wndClassExW{})),
		WndProc:   rawWndProcPtr,
		Instance:  instance,
		ClassName: className,
	}
	if atom, _, callErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		ready <- fmt.Errorf("RegisterClassExW: %w", callErr)
		return
	}
	defer procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), instance)

	hwnd, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		0,
		0, 0, 0, 0,
		hwndMessage,
		0,
		instance,
		0,
	)
	if hwnd == 0 {
		ready <- fmt.Errorf("CreateWindowExW: %w", callErr)
		return
	}
	r.hwnd.Store(hwnd)
	rawReaders.Store(hwnd, r)
	defer func() {
		rawReaders.Delete(hwnd)
		r.hwnd.Store(0)
	}()

	dev := rawInputDevice{
		UsagePage: hidUsagePageGeneric,
		Usage:     hidUsageGenericKeyboard,
		Flags:     ridevInputSink,
		Target:    hwnd,
	}
	if ok, _, regErr := procRegisterRawInputDevices.Call(
		uintptr(unsafe.Pointer(&dev)),
		1,
		unsafe.Sizeof(dev),
	); ok == 0 {
		procDestroyWindow.Call(hwnd)
		ready <- fmt.Errorf("RegisterRawInputDevices: %w", regErr)
		return
	}
	defer func() {
		unreg := rawInputDevice{
			UsagePage: hidUsagePageGeneric,
			Usage:     hidUsageGenericKeyboard,
			Flags:     ridevRemove,
			Target:    0,
		}
		procRegisterRawInputDevices.Call(uintptr(unsafe.Pointer(&unreg)), 1, unsafe.Sizeof(unreg))
	}()

	ready <- nil

	var msg winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		code := int32(ret)
		if code == 0 || code == -1 {
			break
		}
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func (r *RawReader) handleWMInput(lParam uintptr) {
	now := time.Now()
	var buf [8]uint64 // 64 bytes, 8-byte aligned; fits RAWINPUT keyboard without allocation.
	size := uint32(unsafe.Sizeof(buf))
	ret, _, _ := procGetRawInputData.Call(
		lParam,
		ridInput,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		unsafe.Sizeof(rawInputHeader{}),
	)
	// GetRawInputData returns UINT(-1) on error, even on 64-bit Windows.
	if uint32(ret) == ^uint32(0) || uint32(ret) < uint32(unsafe.Sizeof(rawInputKeyboard{})) {
		r.readErrors.Add(1)
		return
	}
	raw := (*rawInputKeyboard)(unsafe.Pointer(&buf[0]))
	if raw.Header.Type != rimTypeKeyboard {
		return
	}
	kb := raw.Keyboard
	if kb.MakeCode == overrunMakeCode || kb.Flags&(riKeyE0|riKeyE1) != 0 {
		return
	}
	for lane, code := range r.scanCodes {
		if kb.MakeCode == code {
			kind := judge.Press
			if kb.Flags&riKeyBreak != 0 {
				kind = judge.Release
			}
			r.queue.push(RawEvent{Lane: lane, Wall: now, Kind: kind})
			return
		}
	}
}

func rawWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmInput:
		if v, ok := rawReaders.Load(hwnd); ok {
			v.(*RawReader).handleWMInput(lParam)
		}
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return ret
}
