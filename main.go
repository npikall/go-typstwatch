package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

var (
	clients   = make(map[chan struct{}]struct{})
	clientsMu sync.Mutex
)

const html = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>LivePDF</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
html, body { height: 100%; background: #404040; }
iframe { width: 100%; height: 100%; border: none; display: block; }
</style>
</head>
<body>
<iframe id="pdf" src="/pdf?t=0"></iframe>
<script>
const es = new EventSource('/events');
es.onmessage = () => {
  const f = document.getElementById('pdf');
  f.src = '/pdf?t=' + Date.now();
};
</script>
</body>
</html>`

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: livepdf <file.pdf>")
	}
	pdfPath, err := filepath.Abs(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stat(pdfPath); err != nil {
		log.Fatalf("cannot open %s: %v", pdfPath, err)
	}

	go watch(pdfPath)

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/pdf", func(w http.ResponseWriter, r *http.Request) {
		pdfHandler(w, r, pdfPath)
	})
	http.HandleFunc("/events", eventsHandler)

	addr := "127.0.0.1:7331"
	log.Printf("serving %s at http://%s", pdfPath, addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

func pdfHandler(w http.ResponseWriter, r *http.Request, pdfPath string) {
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, pdfPath)
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan struct{}, 1)
	clientsMu.Lock()
	clients[ch] = struct{}{}
	clientsMu.Unlock()
	defer func() {
		clientsMu.Lock()
		delete(clients, ch)
		clientsMu.Unlock()
	}()

	for {
		select {
		case <-ch:
			fmt.Fprintf(w, "data: reload\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func broadcast() {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for ch := range clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func watch(pdfPath string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Watch parent dir to catch typst's atomic rename (temp file → final PDF)
	if err := watcher.Add(filepath.Dir(pdfPath)); err != nil {
		log.Fatal(err)
	}

	base := filepath.Base(pdfPath)
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if filepath.Base(event.Name) == base {
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
					broadcast()
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("watcher error:", err)
		}
	}
}
