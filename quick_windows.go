//go:build windows

package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"unicode/utf8"
	"unsafe"
)

var (
	registerHotKey   = user32.NewProc("RegisterHotKey")
	getMessage       = user32.NewProc("GetMessageW")
	messageBox       = user32.NewProc("MessageBoxW")
	getSystemMetrics = user32.NewProc("GetSystemMetrics")
)

const (
	modAlt      = 0x0001
	modControl  = 0x0002
	modNoRepeat = 0x4000
	vkT         = 'T'
	hotkeyID    = 1
	wmHotkey    = 0x0312

	smCxScreen = 0
	smCyScreen = 1

	mbIconInfo      = 0x00000040
	mbTopmost       = 0x00040000
	mbSetForeground = 0x00010000
)

// Every language Tesseract can read that the translator supports. Mixing many
// languages costs some accuracy, but it is the only way to handle "anything".
const defaultQuickLangs = "eng+chi_sim+hin+spa+fra+ara+ben+por+rus+urd+ind+deu+jpn"

// Matches the MyMemory limit; longer text is rejected upstream.
const quickMaxBytes = 500

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	ptX     int32
	ptY     int32
}

// runQuick registers a global hotkey. Each press OCRs the whole screen,
// translates it, and shows the result in a popup.
func runQuick(args []string) {
	fs := flag.NewFlagSet("quick", flag.ExitOnError)
	to := fs.String("to", "en-GB", "target language code")
	ocrLang := fs.String("ocr", defaultQuickLangs, "Tesseract language(s) to try")
	tesseract := fs.String("tesseract", "tesseract", "path to the tesseract binary")
	fs.Parse(args)

	if !supported(*to) {
		log.Fatalf("unsupported target language %q", *to)
	}

	// RegisterHotKey and GetMessage must run on the same thread.
	runtime.LockOSThread()
	ok, _, err := registerHotKey.Call(0, hotkeyID, modControl|modAlt|modNoRepeat, vkT)
	if ok == 0 {
		log.Fatalf("RegisterHotKey failed (is Ctrl+Alt+T already in use?): %v", err)
	}
	log.Println("press Ctrl+Alt+T to translate what is on screen")

	var busy atomic.Bool
	var m winMsg
	for {
		r, _, err := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			log.Fatalf("GetMessage: %v", err)
		}
		if m.message == wmHotkey && busy.CompareAndSwap(false, true) {
			go func() {
				defer busy.Store(false)
				quickTranslate(*tesseract, *ocrLang, *to)
			}()
		}
	}
}

func quickTranslate(tesseract, lang, to string) {
	w, _, _ := getSystemMetrics.Call(smCxScreen)
	h, _, _ := getSystemMetrics.Call(smCyScreen)
	img, err := captureRegion(0, 0, int(w), int(h))
	if err != nil {
		showPopup("Translator", "Screen capture failed: "+err.Error())
		return
	}

	text, err := runOCR(tesseract, lang, "3", img)
	if err != nil {
		showPopup("Translator", "OCR failed: "+err.Error())
		return
	}
	text = truncateBytes(strings.TrimSpace(text), quickMaxBytes)
	if text == "" {
		showPopup("Translator", "No text found on screen.")
		return
	}

	out, from, err := translate("auto", to, text)
	if err != nil {
		showPopup("Translator", "Translation failed: "+err.Error())
		return
	}
	showPopup(fmt.Sprintf("Translation (%s to %s)", from, to), out+"\n\n---\n"+text)
}

// showPopup displays a modal message box that stays on top.
func showPopup(title, body string) {
	t, _ := syscall.UTF16PtrFromString(title)
	b, _ := syscall.UTF16PtrFromString(body)
	messageBox.Call(0, uintptr(unsafe.Pointer(b)), uintptr(unsafe.Pointer(t)),
		mbIconInfo|mbTopmost|mbSetForeground)
}

// truncateBytes cuts s to at most n bytes without splitting a UTF-8 character.
func truncateBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
