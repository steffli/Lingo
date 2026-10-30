//go:build windows

package main

import (
	"fmt"
	"image"
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	user32                 = syscall.NewLazyDLL("user32.dll")
	gdi32                  = syscall.NewLazyDLL("gdi32.dll")
	getDC                  = user32.NewProc("GetDC")
	releaseDC              = user32.NewProc("ReleaseDC")
	setProcessDPIAware     = user32.NewProc("SetProcessDPIAware")
	createCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	createCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	selectObject           = gdi32.NewProc("SelectObject")
	bitBlt                 = gdi32.NewProc("BitBlt")
	getDIBits              = gdi32.NewProc("GetDIBits")
	deleteObject           = gdi32.NewProc("DeleteObject")
	deleteDC               = gdi32.NewProc("DeleteDC")
)

const srcCopy = 0x00CC0020

type bitmapInfoHeader struct {
	Size          uint32
	Width, Height int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

func init() {
	// Without this, scaled displays give coordinates and pixels from a virtualized space.
	setProcessDPIAware.Call()
}

// captureRegion copies a rectangle of the screen into memory.
func captureRegion(x, y, w, h int) (*image.RGBA, error) {
	screen, _, _ := getDC.Call(0)
	if screen == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}
	defer releaseDC.Call(0, screen)

	mem, _, _ := createCompatibleDC.Call(screen)
	if mem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer deleteDC.Call(mem)

	bmp, _, _ := createCompatibleBitmap.Call(screen, uintptr(w), uintptr(h))
	if bmp == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap failed")
	}
	defer deleteObject.Call(bmp)

	selectObject.Call(mem, bmp)
	if ok, _, _ := bitBlt.Call(mem, 0, 0, uintptr(w), uintptr(h), screen, uintptr(x), uintptr(y), srcCopy); ok == 0 {
		return nil, fmt.Errorf("BitBlt failed")
	}

	hdr := bitmapInfoHeader{
		Size:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:    int32(w),
		Height:   -int32(h), // negative: top-down rows
		Planes:   1,
		BitCount: 32,
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	if ok, _, _ := getDIBits.Call(mem, bmp, 0, uintptr(h),
		uintptr(unsafe.Pointer(&img.Pix[0])), uintptr(unsafe.Pointer(&hdr)), 0); ok == 0 {
		return nil, fmt.Errorf("GetDIBits failed")
	}

	// Windows gives BGRA; image.RGBA wants RGBA with opaque alpha.
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+2] = img.Pix[i+2], img.Pix[i]
		img.Pix[i+3] = 255
	}
	return img, nil
}

// hideWindow stops a console window from flashing up for each OCR call.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
