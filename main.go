package main

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	flag "github.com/spf13/pflag"
)

//go:embed "web"
var fs embed.FS

func main() {
	var (
		filename         = flag.StringP("file", "f", "", "Markdown file to preview")
		skipPreview      = flag.BoolP("skip-preview", "s", false, "Skip auto-preview and output to a file")
		templateFilename = flag.StringP("template", "t", "", "Alternate template file")
	)
	flag.Parse()

	// show usage when no filename is provided
	if *filename == "" {
		flag.Usage()
		os.Exit(1)
	}

	data, err := convertToHTML(*filename, *templateFilename)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if *skipPreview {
		if err := createFile(data); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		done := make(chan struct{})
		if err := preview(data, done); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		select {
		case <-done:
			os.Exit(0)
		case <-time.After(5 * time.Second):
			fmt.Println("main(): timed out")
			os.Exit(1)
		}
	}
}

// convertToHTML takes reads the input file and then uses it to generate
// safe HTML. This HTML is then embedded into a template – either a user
// defined template, or the default embedded template – written to a
// buffer, and then the final output is returned as a byte slice
func convertToHTML(inputFilename, templateFilename string) ([]byte, error) {
	input, err := os.ReadFile(inputFilename)
	if err != nil {
		return nil, fmt.Errorf("convertToHTML(): %w", err)
	}

	// Generate HTML
	output := blackfriday.Run(input)

	// Sanitize HTML
	body := bluemonday.UGCPolicy().SanitizeBytes(output)

	var tmpl *template.Template
	if templateFilename != "" {
		// User-provided template file, if exists
		tmpl, err = template.ParseFiles(templateFilename)
	} else {
		// Default, embedded template file
		tmpl, err = template.ParseFS(fs, "web/template/default.tmpl")
	}
	if err != nil {
		return nil, fmt.Errorf("convertToHTML(): %w", err)
	}

	content := map[string]any{
		"Title": "Markdown Preview Tool",
		"Body":  template.HTML(body),
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, content); err != nil {
		return nil, fmt.Errorf("convertToHTML(): %w", err)
	}

	return buf.Bytes(), nil
}

// createFile takes HTML data and writes to a file with a generated
// filename. The filename and filepath is then printed to stdout.
func createFile(data []byte) error {
	file, err := os.CreateTemp("", "mdp*.html")
	if err != nil {
		return fmt.Errorf("createFile(): %w", err)
	}

	if err = file.Close(); err != nil {
		return fmt.Errorf("createFile(): %w", err)
	}

	fn := file.Name()
	if err = os.WriteFile(fn, data, 0644); err != nil {
		return fmt.Errorf("createFile(): %w", err)
	}
	fmt.Println(fn)

	return nil
}

// preview takes data, the HTML byte slice, and done, a channel to signal
// completion. It creates a server, and serves the HTML on a dynamically
// assigned port – and then launches a browser, and navigates to that
// port. Once the server has sent the HTML response, done is closed and
// signals to the main thread that it can safely exit.
func preview(data []byte, done chan<- struct{}) error {
	// Define OS-specific command
	command := defineCommand()
	if command.executable == "" {
		return errors.New("preview(): OS not supported")
	}

	// Locate executable in PATH
	commandPath, err := exec.LookPath(command.executable)
	if err != nil {
		return fmt.Errorf("preview(): %w", err)
	}

	// Open listener so the browser knows to wait for a response
	l, err := net.Listen("tcp", "localhost:")
	if err != nil {
		return fmt.Errorf("preview(): %w", err)
	}

	// Launch browser and navigate to the listen address
	command.params = append(command.params, fmt.Sprintf("http://%s", l.Addr().String()))
	if err = exec.Command(commandPath, command.params...).Run(); err != nil {
		return fmt.Errorf("preview(): %w", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Closing this channel sends a signal that the request has
		// been sent a response – main() will then teardown the server
		// and exit
		defer close(done)
		w.Write(data)
	})

	go http.Serve(l, nil)

	return nil
}

type Command struct {
	executable string
	params     []string
}

// defineCommand builds a Command struct with an OS-specific executable
// and parameters
func defineCommand() Command {
	switch runtime.GOOS {
	case "linux":
		return Command{
			executable: "xdg-open",
		}
	case "windows":
		return Command{
			executable: "cmd.exe",
			params:     []string{"/C", "start"},
		}
	case "darwin":
		return Command{
			executable: "open",
		}
	default:
		return Command{}
	}
}
