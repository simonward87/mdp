package html

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"

	"mdp/web"
)

// Convert takes reads the input markdown file and then uses it to generate
// safe HTML. This HTML is then embedded into a template – either a user
// defined template, or the default embedded template – written to a
// buffer, and then the final output is returned as a byte slice
func Convert(inputFilename, templateFilename string) ([]byte, error) {
	var buf bytes.Buffer
	var err error
	var t *template.Template

	switch templateFilename {
	case "": // embedded template
		templateFilename = "template/default.min.tmpl"
		t, err = template.ParseFS(web.FS, templateFilename)
	default: // user-provided template
		t, err = template.ParseFiles(templateFilename)
	}
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", templateFilename, err)
	}

	markdown, err := os.ReadFile(inputFilename)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", inputFilename, err)
	}

	body := generateHTML(markdown)
	if err = t.Execute(&buf, map[string]any{
		"Title": "Markdown Preview Tool",
		"Body":  template.HTML(body),
	}); err != nil {
		err = fmt.Errorf("execute template: %w", err)
	}

	return buf.Bytes(), err
}

func generateHTML(markdown []byte) []byte {
	return bluemonday.UGCPolicy().SanitizeBytes(blackfriday.Run(markdown))
}

// CreateFile takes HTML data and writes to a file with a generated
// filename. The filename and filepath is then printed to stdout.
func CreateFile(data []byte) error {
	f, err := os.CreateTemp("", "mdp*.html")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if err = f.Close(); err != nil {
		return fmt.Errorf("close file: %w", err)
	}

	if err = os.WriteFile(f.Name(), data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Println(f.Name())
	return nil
}

// Preview takes data, the HTML byte slice, and done, a channel to signal
// completion. It creates a server, and serves the HTML on a dynamically
// assigned port – and then launches a browser, and navigates to that
// port. Once the server has sent the HTML response, done is closed and
// signals to the main thread that it can safely exit.
func Preview(data []byte, done chan<- error) error {
	// Define OS-specific command
	cmd := defineCommand()
	if cmd.executable == "" {
		return errors.New("OS not supported")
	}

	// Locate executable in PATH
	cmdPath, err := exec.LookPath(cmd.executable)
	if err != nil {
		return fmt.Errorf("search for executable: %w", err)
	}

	// Open listener so the browser knows to wait for a response
	l, err := net.Listen("tcp", "localhost:")
	if err != nil {
		return fmt.Errorf("announce on local network address: %w", err)
	}

	cmd.params = append(cmd.params, fmt.Sprintf(
		"http://%s",
		l.Addr().String(),
	))

	// Launch browser and navigate to the listen address
	if err = exec.Command(cmdPath, cmd.params...).Run(); err != nil {
		return fmt.Errorf(
			"execute command '%s %s': %w",
			cmdPath,
			strings.Join(cmd.params, " "),
			err,
		)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var err error
		// Closing done sends a signal that the request has been handled,
		// allowing main to teardown the server and exit
		defer close(done)
		if _, err = w.Write(data); err != nil {
			err = fmt.Errorf("write HTTP response: %w", err)
		}
		done <- err
	})

	go func() {
		// Potential race condition? err could be nil when checked
		// by the main thread, even if it will actually error?
		err = http.Serve(l, nil)
		if err != nil {
			err = fmt.Errorf("serve HTTP: %w", err)
		}
	}()

	return err
}

type command struct {
	executable string
	params     []string
}

// defineCommand builds a command struct with an OS-specific executable
// and parameters
func defineCommand() command {
	switch runtime.GOOS {
	case "linux":
		return command{
			executable: "xdg-open",
		}
	case "windows":
		return command{
			executable: "cmd.exe",
			params:     []string{"/C", "start"},
		}
	case "darwin":
		return command{
			executable: "open",
		}
	default:
		return command{}
	}
}
