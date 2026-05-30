package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	port := flag.Int("port", 42069, "port to listen on")
	format := flag.String("format", "pdf", "output format passed to typst watch (pdf, png, svg)")
	diagnosticFormat := flag.String("diagnostic-format", "short", "typst diagnostic format (human, short)")
	root := flag.String("root", "", "root directory for typst file access (default: input file directory)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "go-typstwatch [-port N] [-format pdf|png|svg] [-diagnostic-format human|short]")
		fmt.Fprintln(os.Stderr, "              [-root DIR] <file.typ>")
		fmt.Fprintln(os.Stderr, "")
		flag.Usage()
		os.Exit(1)
	}

	inputPath, err := filepath.Abs(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	outputPath, typstCmd := resolveTargets(inputPath, *format, *diagnosticFormat, *root)

	if typstCmd != nil {
		if err := launchTypstWatch(typstCmd); err != nil {
			log.Fatalf("failed to start typst watch: %v", err)
		}
		killOnShutdown(typstCmd)
	} else {
		if _, err := os.Stat(outputPath); err != nil {
			log.Fatalf("cannot open %s: %v", outputPath, err)
		}
	}

	go watchOutput(outputPath)

	outputDir := filepath.Dir(outputPath)
	stem := strings.TrimSuffix(filepath.Base(outputPath), filepath.Ext(outputPath))
	ext := filepath.Ext(outputPath)

	page := buildHTML(*format, stem)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, page)
	})
	http.HandleFunc("/events", eventsHandler)

	if *format == "pdf" {
		http.HandleFunc("/output", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, outputPath)
		})
	} else {
		http.HandleFunc("/pages", func(w http.ResponseWriter, r *http.Request) {
			pagesHandler(w, r, outputDir, stem, ext)
		})
		http.HandleFunc("/output/", func(w http.ResponseWriter, r *http.Request) {
			outputFileHandler(w, r, outputDir)
		})
	}

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	log.Printf("serving %s at http://%s", filepath.Base(outputPath), addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
