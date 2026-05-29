package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

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
<title>Go TypstWatcher</title>
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
	port := flag.Int("port", 42069, "port to listen on")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("go-typstwatcher [-port N] <file.typ>")
		fmt.Println()
		flag.Usage()
		os.Exit(1)
	}

	inputPath, err := filepath.Abs(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	pdfPath, typstCmd := resolveTargets(inputPath)

	if typstCmd != nil {
		if err := launchTypstWatch(typstCmd); err != nil {
			log.Fatalf("failed to start typst watch: %v", err)
		}
		killOnShutdown(typstCmd)
	} else {
		if _, err := os.Stat(pdfPath); err != nil {
			log.Fatalf("cannot open %s: %v", pdfPath, err)
		}
	}

	go watchPDF(pdfPath)

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/pdf", func(w http.ResponseWriter, r *http.Request) {
		pdfHandler(w, r, pdfPath)
	})
	http.HandleFunc("/events", eventsHandler)

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	log.Printf("serving %s at http://%s", filepath.Base(pdfPath), addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// resolveTargets returns the PDF path to serve/watch and, for .typ input,
// a ready-to-start "typst watch" command.
func resolveTargets(inputPath string) (pdfPath string, typstCmd *exec.Cmd) {
	if strings.EqualFold(filepath.Ext(inputPath), ".typ") {
		stem := strings.TrimSuffix(inputPath, filepath.Ext(inputPath))
		return stem + ".pdf", exec.Command("typst", "watch", inputPath)
	}
	return inputPath, nil
}

func launchTypstWatch(cmd *exec.Cmd) error {
	cmd.Stdout = io.Discard

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	go forwardTypstErrors(stderr)

	log.Printf("starting: %s", strings.Join(cmd.Args, " "))
	return cmd.Start()
}

func forwardTypstErrors(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !isTypstStatusLine(line) {
			log.Printf("[typst] %s", line)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func isTypstStatusLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	return strings.HasPrefix(lower, "watching") ||
		strings.Contains(lower, "compiled successfully")
}

// killOnShutdown kills the child process when SIGINT or SIGTERM is received.
func killOnShutdown(cmd *exec.Cmd) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		os.Exit(0)
	}()
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

func watchPDF(pdfPath string) {
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
					log.Printf("pdf updated: %s", base)
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
