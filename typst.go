package main

import (
	"bufio"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

// resolveTargets returns the output path to serve/watch and, for .typ input,
// a ready-to-start "typst watch" command with the requested format.
func resolveTargets(inputPath, format, diagnosticFormat, root string) (outputPath string, typstCmd *exec.Cmd) {
	if !strings.EqualFold(filepath.Ext(inputPath), ".typ") {
		return inputPath, nil
	}
	stem := strings.TrimSuffix(inputPath, filepath.Ext(inputPath))
	outPath := stem + "." + format

	args := []string{"watch", inputPath}
	if format != "pdf" {
		// PNG/SVG produce one file per page; typst requires {p} template in output path.
		args = append(args, stem+"-{p}."+format)
	}
	args = append(args, "--format", format, "--diagnostic-format", diagnosticFormat)

	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		root = cwd
	}
	args = append(args, "--root", root)

	cmd := exec.Command("typst", args...)
	cmd.Dir = filepath.Dir(inputPath)
	return outPath, cmd
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
