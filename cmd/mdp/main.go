package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	"mdp/internal/html"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Run error: %s\n", err)
		os.Exit(1)
	}
}

type Options struct {
	skipPreview  bool
	targetFile   string
	templateFile string
}

func run() error {
	opts := Options{}

	flag.StringVarP(&opts.targetFile, "file", "f", "", "Markdown file to preview")
	flag.BoolVarP(&opts.skipPreview, "skip-preview", "s", false, "Skip preview and output to a file")
	flag.StringVarP(&opts.templateFile, "template", "t", "", "Custom template file")
	flag.Parse()

	// Display usage when no target filename provided
	if opts.targetFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	body, err := html.Convert(opts.targetFile, opts.templateFile)
	if err != nil {
		return fmt.Errorf("convert HTML: %w", err)
	}

	if opts.skipPreview {
		if err := html.CreateFile(body); err != nil {
			return fmt.Errorf("create file: %w", err)
		}
		return nil
	}

	errChan := make(chan error)
	defer close(errChan)

	if err := html.Preview(body, errChan); err != nil {
		return fmt.Errorf("preview HTML: %w", err)
	}

	select {
	case err := <-errChan:
		return err
	case <-time.After(5 * time.Second):
		return errors.New("server timed out")
	}
}
