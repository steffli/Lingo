package main

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// runWatch implements `translator watch`: it watches a screen region and
// prints a translation for every new line of text that appears in it.
func runWatch(args []string) {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	region := fs.String("region", "", "screen region to watch as x,y,width,height (required)")
	to := fs.String("to", "en-GB", "target language code")
	ocrLang := fs.String("ocr", "eng", "Tesseract language(s), e.g. eng or eng+chi_sim")
	tesseract := fs.String("tesseract", "tesseract", "path to the tesseract binary")
	interval := fs.Duration("interval", 500*time.Millisecond, "how often to check the region")
	fs.Parse(args)

	r, err := parseRegion(*region)
	if err != nil {
		log.Fatal(err)
	}
	if !supported(*to) {
		log.Fatalf("unsupported target language %q", *to)
	}

	w := watcher{
		region:    r,
		to:        *to,
		ocrLang:   *ocrLang,
		tesseract: *tesseract,
		seen:      map[string]bool{},
	}
	log.Printf("watching %v, translating to %s", r, *to)
	for {
		if err := w.tick(); err != nil {
			log.Fatal(err)
		}
		time.Sleep(*interval)
	}
}

type watcher struct {
	region    [4]int
	to        string
	ocrLang   string
	tesseract string
	seen      map[string]bool // every line already translated
	last      [32]byte        // hash of the last captured frame
}

// tick captures the region and does OCR + translation only if the pixels changed.
func (w *watcher) tick() error {
	img, err := captureRegion(w.region[0], w.region[1], w.region[2], w.region[3])
	if err != nil {
		return err
	}
	sum := sha256.Sum256(img.Pix)
	if sum == w.last {
		return nil
	}
	w.last = sum

	lines, err := w.ocr(img)
	if err != nil {
		log.Println("ocr:", err)
		return nil
	}
	for _, line := range lines {
		if w.seen[line] {
			continue
		}
		w.seen[line] = true
		out, from, err := translate("auto", w.to, line)
		if err != nil {
			log.Println("translate:", err)
			continue
		}
		fmt.Printf("[%s] %s\n    -> %s\n", from, line, out)
	}
	return nil
}

// ocr returns the non-empty lines of text Tesseract finds in the frame.
func (w *watcher) ocr(img *image.RGBA) ([]string, error) {
	// psm 6: a uniform block of text, which fits a chat window.
	text, err := runOCR(w.tesseract, w.ocrLang, "6", img)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

// runOCR writes the frame to a temp PNG and returns Tesseract's raw text output.
func runOCR(bin, lang, psm string, img *image.RGBA) (string, error) {
	f, err := os.CreateTemp("", "translator-*.png")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return "", err
	}
	f.Close()

	cmd := exec.Command(bin, f.Name(), "stdout", "-l", lang, "--psm", psm)
	hideWindow(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// parseRegion parses "x,y,width,height".
func parseRegion(s string) ([4]int, error) {
	var r [4]int
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return r, fmt.Errorf("-region must be x,y,width,height")
	}
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return r, fmt.Errorf("-region: %q is not a number", p)
		}
		r[i] = n
	}
	if r[2] <= 0 || r[3] <= 0 {
		return r, fmt.Errorf("-region width and height must be positive")
	}
	return r, nil
}
