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
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {
	var (
		markdownFile = flag.StringP("file", "f", "", "Markdown file to preview")
		skipPreview  = flag.BoolP("skip-preview", "s", false, "Skip auto-preview and output to a file")
		templateFile = flag.StringP("template", "t", "", "Alternate template file")
	)
	flag.Parse()

	// When no filename is provided, display usage
	if *markdownFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	data, err := html.Convert(*markdownFile, *templateFile)
	if err != nil {
		return fmt.Errorf("html.ConvertToHTML: %w", err)
	}

	if *skipPreview {
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
