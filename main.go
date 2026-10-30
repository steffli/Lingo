package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Top languages by total speakers. Codes are the ones MyMemory accepts.
var languages = []struct{ Code, Name string }{
	{"en-GB", "English"},
	{"zh-CN", "Chinese (Mandarin)"},
	{"hi-IN", "Hindi"},
	{"es-ES", "Spanish"},
	{"fr-FR", "French"},
	{"ar-SA", "Arabic"},
	{"bn-BD", "Bengali"},
	{"pt-PT", "Portuguese"},
	{"ru-RU", "Russian"},
	{"ur-PK", "Urdu"},
	{"id-ID", "Indonesian"},
	{"de-DE", "German"},
	{"ja-JP", "Japanese"},
}

// ponytail: MyMemory anonymous quota (~5k chars/day); add an API key param or swap provider when it bites.
var upstream = "https://api.mymemory.translated.net/get"

// MyMemory rejects queries over 500 bytes.
const maxBytes = 500

//go:embed index.html
var pageSrc string

var page = template.Must(template.New("index").Parse(pageSrc))

func supported(code string) bool {
	for _, l := range languages {
		if l.Code == code {
			return true
		}
	}
	return false
}

// translate returns the translation and, when from is "auto", the detected source language.
func translate(from, to, text string) (string, string, error) {
	u, err := url.Parse(upstream)
	if err != nil {
		return "", "", err
	}
	if from == "auto" {
		from = "Autodetect"
	}
	q := u.Query()
	q.Set("q", text)
	q.Set("langpair", from+"|"+to)
	// Skip MyMemory's public translation memory: it returns junk for common
	// phrases (wrong language, unrelated text). Machine translation only.
	q.Set("mt", "1")
	q.Set("onlyprivate", "1")
	u.RawQuery = q.Encode()

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var out struct {
		ResponseData struct {
			TranslatedText   string `json:"translatedText"`
			DetectedLanguage string `json:"detectedLanguage"`
		} `json:"responseData"`
		ResponseStatus  any    `json:"responseStatus"` // int or string upstream
		ResponseDetails string `json:"responseDetails"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", err
	}
	if fmt.Sprint(out.ResponseStatus) != "200" {
		return "", "", errors.New(out.ResponseDetails)
	}
	return out.ResponseData.TranslatedText, out.ResponseData.DetectedLanguage, nil
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	var buf bytes.Buffer
	if err := page.Execute(&buf, languages); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func handleTranslate(w http.ResponseWriter, r *http.Request) {
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	text := strings.TrimSpace(r.URL.Query().Get("text"))

	switch {
	case (from != "auto" && !supported(from)) || !supported(to):
		http.Error(w, "unsupported language", http.StatusBadRequest)
		return
	case text == "":
		http.Error(w, "empty text", http.StatusBadRequest)
		return
	case len(text) > maxBytes:
		http.Error(w, "text too long", http.StatusBadRequest)
		return
	}

	out, detected, err := translate(from, to, text)
	if err != nil {
		log.Println("translate:", err)
		http.Error(w, "translation failed", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"translation": out, "detected": detected})
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/translate", handleTranslate)
	return mux
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "watch":
			runWatch(os.Args[2:])
			return
		case "quick":
			runQuick(os.Args[2:])
			return
		}
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Println("listening on", addr)
	log.Fatal(http.ListenAndServe(addr, newMux()))
}
