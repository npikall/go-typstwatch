package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
)

var (
	clients   = make(map[chan struct{}]struct{})
	clientsMu sync.Mutex
)

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

func pagesHandler(w http.ResponseWriter, r *http.Request, outputDir, stem, ext string) {
	matches, err := filepath.Glob(filepath.Join(outputDir, stem+"*"+ext))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = filepath.Base(m)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(names)
}

// outputFileHandler serves individual page files. Uses filepath.Base to
// prevent path traversal outside the output directory.
func outputFileHandler(w http.ResponseWriter, r *http.Request, outputDir string) {
	filename := filepath.Base(r.URL.Path)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, filepath.Join(outputDir, filename))
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
