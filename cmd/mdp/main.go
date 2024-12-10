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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	file := flag.StringP("file", "f", "", "Markdown file to preview")
	skip := flag.BoolP("skip-preview", "s", false, "Skip preview and output to a file")
	tFile := flag.StringP("template", "t", "", "Custom template file")
	flag.Parse()

	// When no filename is provided, display usage
	if *file == "" {
		flag.Usage()
		os.Exit(1)
	}

	data, err := html.Convert(*file, *tFile)
	if err != nil {
		return fmt.Errorf("html.Convert: %w", err)
	}

	if *skip {
		if err := html.CreateFile(data); err != nil {
			return fmt.Errorf("html.CreateFile: %w", err)
		}
		return nil
	}

	done := make(chan error)
	if err := html.Preview(data, done); err != nil {
		return fmt.Errorf("html.Preview: %w", err)
	}
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		return errors.New("timed out")
	}
}
