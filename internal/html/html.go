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
func Convert(inputName, templateName string) ([]byte, error) {
	var buf bytes.Buffer

	t, err := parseTemplate(&templateName)
	if err != nil {
		return nil, fmt.Errorf("parse template %q: %w", templateName, err)
	}

	markdown, err := os.ReadFile(inputName)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", inputName, err)
	}

	content, err := generateHTML(markdown)
	if err != nil {
		return nil, fmt.Errorf("generate HTML: %w", err)
	}

	if err = t.Execute(&buf, map[string]any{
		"Title": "Markdown Preview Tool",
		"Body":  template.HTML(content),
	}); err != nil {
		err = fmt.Errorf("execute template: %w", err)
	}

	return buf.Bytes(), err
}

func parseTemplate(name *string) (*template.Template, error) {
	switch *name {
	case "": // embedded template
		*name = "template/default.tmpl"
		return template.ParseFS(web.FS, *name)
	default: // user-provided template
		return template.ParseFiles(*name)
	}
}

func generateHTML(markdown []byte) ([]byte, error) {
	b := blackfriday.Run(markdown)
	if len(b) == 0 {
		return nil, errors.New("render: malformed input")
	}

	b = bluemonday.UGCPolicy().SanitizeBytes(b)
	if len(b) == 0 {
		return nil, errors.New("sanitize: malformed input")
	}

	return b, nil
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

// Preview takes data, the HTML byte slice, and ch, a channel to signal
// completion. It creates a server, and serves the HTML on a dynamically
// assigned port – and then launches a browser, and navigates to that
// port. Once the server has sent the HTML response, ch signals to
// the main thread that it can safely exit.
func Preview(data []byte, ch chan<- error) error {
	// Define OS-specific command
	cmd := defineCommand()
	if cmd.executable == "" {
		return errors.New("operating system not supported")
	}

	// Locate executable in PATH
	cmdPath, err := exec.LookPath(cmd.executable)
	if err != nil {
		return fmt.Errorf("search path for executable: %w", err)
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

	go func() {
		m := http.NewServeMux()
		m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")

			if _, err = w.Write(data); err != nil {
				ch <- fmt.Errorf("write HTTP response: %w", err)
			}
			ch <- nil
		})

		if err := http.Serve(l, m); err != nil {
			ch <- fmt.Errorf("serve HTTP: %w", err)
		}
	}()

	return nil
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
