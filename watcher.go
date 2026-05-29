package main

import (
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceDuration = 50 * time.Millisecond

func watchOutput(outputPath string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Watch parent dir to catch typst's atomic rename (temp file → final output)
	if err := watcher.Add(filepath.Dir(outputPath)); err != nil {
		log.Fatal(err)
	}

	stem := strings.TrimSuffix(filepath.Base(outputPath), filepath.Ext(outputPath))
	var debounce *time.Timer
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if matchesOutputStem(event.Name, stem) {
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
					name := filepath.Base(event.Name)
					if debounce != nil {
						debounce.Stop()
					}
					debounce = time.AfterFunc(debounceDuration, func() {
						log.Printf("output updated: %s", name)
						broadcast()
					})
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

// matchesOutputStem returns true for the output file itself (document.png)
// and for numbered page files typst generates (document-1.png, document-2.png).
func matchesOutputStem(eventPath, stem string) bool {
	base := filepath.Base(eventPath)
	fileStem := strings.TrimSuffix(base, filepath.Ext(base))
	return fileStem == stem || strings.HasPrefix(fileStem, stem+"-")
}
