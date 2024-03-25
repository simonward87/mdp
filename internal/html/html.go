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

	if templateFilename != "" {
		// User-provided template
		t, err = template.ParseFiles(templateFilename)
	} else {
		// Default embedded template
		t, err = template.ParseFS(web.FS, "template/default.tmpl")
	}
	if err != nil {
		return nil, err
	}

	markdown, err := os.ReadFile(inputFilename)
	if err != nil {
		return nil, err
	}
	body := generateHTML(markdown)
	err = t.Execute(&buf, map[string]any{
		"Title": "Markdown Preview Tool",
		"Body":  template.HTML(body),
	})

	return buf.Bytes(), err
}

func generateHTML(markdown []byte) []byte {
	return bluemonday.UGCPolicy().SanitizeBytes(blackfriday.Run(markdown))
}

// CreateFile takes HTML data and writes to a file with a generated
// filename. The filename and filepath is then printed to stdout.
func CreateFile(data []byte) error {
	file, err := os.CreateTemp("", "mdp*.html")
	if err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	defer fmt.Println(file.Name())
	return os.WriteFile(file.Name(), data, 0644)
}

// Preview takes data, the HTML byte slice, and done, a channel to signal
// completion. It creates a server, and serves the HTML on a dynamically
// assigned port – and then launches a browser, and navigates to that
// port. Once the server has sent the HTML response, done is closed and
// signals to the main thread that it can safely exit.
func Preview(data []byte, done chan<- error) error {
	// Define OS-specific command
	command := defineCommand()
	if command.executable == "" {
		return errors.New("OS not supported")
	}

	// Locate executable in PATH
	commandPath, err := exec.LookPath(command.executable)
	if err != nil {
		return err
	}

	// Open listener so the browser knows to wait for a response
	l, err := net.Listen("tcp", "localhost:")
	if err != nil {
		return err
	}

	// Launch browser and navigate to the listen address
	command.params = append(command.params, fmt.Sprintf(
		"http://%s",
		l.Addr().String(),
	))
	err = exec.Command(commandPath, command.params...).Run()
	if err != nil {
		return err
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Closing this channel sends a signal that the request has
		// been sent a response – main() will then teardown the server
		// and exit
		defer close(done)
		if _, err := w.Write(data); err != nil {
			done <- err
		}
	})

	go func() {
		err = http.Serve(l, nil)
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
