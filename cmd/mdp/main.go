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
	help         bool
	skipPreview  bool
	templateFile string
}

func run() error {
	var opts Options
	flag.BoolVarP(&opts.help, "help", "h", false, "Show usage information")
	flag.BoolVarP(&opts.skipPreview, "skip-preview", "s", false, "Skip preview and output to a file")
	flag.StringVarP(&opts.templateFile, "template", "t", "", "Custom template file")
	flag.Parse()
	flag.Usage = usage

	if len(os.Args) == 1 {
		flag.Usage()
		os.Exit(1)
	}
	if opts.help {
		flag.Usage()
		os.Exit(0)
	}

	// TODO: Enable parsing of multiple files
	target := os.Args[1]
	body, err := html.Convert(target, opts.templateFile)
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

func usage() {
	fmt.Fprintf(
		os.Stderr,
		"%s - markdown preview\n\n",
		os.Args[0],
	)
	fmt.Fprintf(os.Stderr, "Usage: mdp [options] file\n\n")
	flag.PrintDefaults()
	fmt.Fprintf(
		os.Stderr,
		"\nDeveloped by Simon Ward. Copyright %d.\n",
		time.Now().Year(),
	)
}
