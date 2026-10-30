package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTranslateEndpoint(t *testing.T) {
	// Fake upstream so the test never hits the network.
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pair := r.URL.Query().Get("langpair")
		if pair != "en-GB|es-ES" && pair != "Autodetect|es-ES" {
			t.Errorf("langpair = %q", pair)
		}
		w.Write([]byte(`{"responseData":{"translatedText":"Hola mundo","detectedLanguage":"en"},"responseStatus":200}`))
	}))
	defer fake.Close()
	upstream = fake.URL
	defer func() { upstream = "https://api.mymemory.translated.net/get" }()

	cases := []struct {
		name, query string
		want        int
	}{
		{"ok", "from=en-GB&to=es-ES&text=Hello+world", http.StatusOK},
		{"auto detect", "from=auto&to=es-ES&text=Hello+world", http.StatusOK},
		{"bad language", "from=xx&to=es-ES&text=Hi", http.StatusBadRequest},
		{"empty text", "from=en-GB&to=es-ES&text=+", http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newMux().ServeHTTP(rec, httptest.NewRequest("GET", "/api/translate?"+c.query, nil))
			if rec.Code != c.want {
				t.Fatalf("status = %d, want %d", rec.Code, c.want)
			}
			if c.want == http.StatusOK {
				var got map[string]string
				json.Unmarshal(rec.Body.Bytes(), &got)
				if got["translation"] != "Hola mundo" {
					t.Fatalf("translation = %q", got["translation"])
				}
			}
		})
	}
}

func TestTranslateSkipsPublicMemory(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("mt") != "1" || q.Get("onlyprivate") != "1" {
			t.Errorf("mt = %q, onlyprivate = %q, want both 1", q.Get("mt"), q.Get("onlyprivate"))
		}
		w.Write([]byte(`{"responseData":{"translatedText":"Hello, how are you today?"},"responseStatus":200}`))
	}))
	defer fake.Close()
	upstream = fake.URL
	defer func() { upstream = "https://api.mymemory.translated.net/get" }()

	if _, _, err := translate("de-DE", "en-GB", "Hallo, wie geht es dir heute?"); err != nil {
		t.Fatal(err)
	}
}
