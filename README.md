# Translator

A small, single-binary web app for translating short text between the world's most widely spoken languages. A plain HTML page is served by a Go server, which proxies translation requests to the free [MyMemory](https://mymemory.translated.net/) API.

## Features

- Translate between 13 languages: English, Chinese (Mandarin), Hindi, Spanish, French, Arabic, Bengali, Portuguese, Russian, Urdu, Indonesian, German, and Japanese.
- Automatic source language detection (choose "Detect language" as the source). The detected language is shown with the result.
- Swap source and target languages with one click.
- Ctrl+Enter (Cmd+Enter on macOS) translates; a byte counter shows the 500-byte limit.
- Responsive UI with automatic light and dark themes.
- No frontend build step, no JavaScript dependencies. The page is embedded into the binary.

## Requirements

- Go (the version in `go.mod`, or newer)
- Internet access to reach `api.mymemory.translated.net`

## Getting started

```sh
go build -o translator .
./translator
```

Then open <http://localhost:8080>.

To listen on a different address, set `ADDR`:

```sh
ADDR=:3000 ./translator
```

## Auto-translate game chat (Windows)

`translator watch` captures a fixed screen region, OCRs it with [Tesseract](https://github.com/UB-Mannheim/tesseract/wiki), and prints a translation for every new line that appears. It only runs OCR when the pixels in the region change.

```sh
translator.exe watch -region 0,800,700,260 -to en-GB -ocr eng
```

- `-region x,y,width,height` is the chat area in screen pixels. Borderless and windowed games work; exclusive fullscreen may capture black.
- `-ocr` sets Tesseract's language(s), e.g. `eng+chi_sim`. Mixing languages lowers accuracy.
- Requires Tesseract on the PATH or passed with `-tesseract`. Install with `winget install UB-Mannheim.TesseractOCR`, then add the language packs you need.

The watcher can only run on Windows. Other platforms build but exit with an error.

## Quick translate anything on screen (Windows)

`translator quick` registers the global hotkey **Ctrl+Alt+T**. Each press OCRs the whole screen, translates it to the target language, and shows the result in a popup with the original text below.

```sh
translator.exe quick -to en-GB
```

- Works in any app, not only games.
- Tesseract runs with a wide set of languages (`-ocr`), so it can read most scripts. Accuracy is lower than a single-language run.
- Text is cut to 500 bytes before translation, the MyMemory limit.
- Ctrl+Alt+T must be free. If another app already uses it, startup fails with an error.

## Running the tests

```sh
go test ./...
```

The test suite uses a fake upstream server, so it does not hit the network.

## How it works

| Route | Method | Description |
| --- | --- | --- |
| `/` | GET | Serves the translator page (`index.html`, embedded at build time). |
| `/api/translate` | GET | Translates text. Query params: `from` (language code or `auto`), `to` (language code), `text`. Returns `{"translation": "...", "detected": "fr"}`; `detected` is set when `from=auto`. |

Errors on `/api/translate`:

- `400` for an unsupported language, empty text, or text longer than 500 bytes.
- `502` if the upstream translation request fails.

## Project layout

```
.
├── main.go          # HTTP server, language list, MyMemory client
├── main_test.go     # Endpoint tests against a fake upstream
├── index.html       # Frontend (embedded into the binary)
└── go.mod
```

## Limitations

- Input is capped at 500 bytes per request, which is a MyMemory limit.
- The anonymous MyMemory quota is roughly 5,000 characters per day. Once it is used up, translations will fail until it resets. Adding an API key or switching providers is the fix if this becomes a problem.
- Only MyMemory's machine translation is used (`mt=1&onlyprivate=1`). Its public translation memory is skipped because it returns junk for common phrases, such as the wrong language or unrelated text. Good human translations from that memory are skipped too.
- Translation quality is whatever MyMemory's machine translation returns; this app does no post-processing.
