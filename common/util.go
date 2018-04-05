package common

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// CheckForDockerCompose returns error if current directory is a
// not a docker-compose project
func CheckForDockerCompose(cwd string) error {
	dockerComposeYML := filepath.Join(cwd, "docker-compose.yml")
	dockerComposeYAML := filepath.Join(cwd, "docker-compose.yaml")
	_, err := os.Stat(dockerComposeYML)
	YMLpresent := os.IsNotExist(err)
	_, err = os.Stat(dockerComposeYAML)
	YAMLpresent := os.IsNotExist(err)
	if YMLpresent && YAMLpresent {
		return errors.New("this does not appear to be a docker-compose project - currently,\n" +
			"Inertia only supports docker-compose projects.")
	}
	return nil
}

// RemoveContents removes all files within given directory, returns nil if successful
func RemoveContents(directory string) error {
	d, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(directory, name))
		if err != nil {
			return err
		}
	}
	return nil
}

// FlushRoutine continuously writes everything in given ReadCloser
// to a ResponseWriter. Use this as a goroutine.
func FlushRoutine(w io.Writer, rc io.ReadCloser) {
	buffer := make([]byte, 100)
	for {
		// Read from pipe then write to ResponseWriter and flush it,
		// sending the copied content to the client.
		err := Flush(w, rc, buffer)
		if err != nil {
			break
		}
	}
}

// Flush emptires reader into buffer and flushes it to writer
func Flush(w io.Writer, rc io.ReadCloser, buffer []byte) error {
	n, err := rc.Read(buffer)
	if err != nil {
		rc.Close()
		return err
	}
	data := buffer[0:n]
	w.Write(data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Clear the buffer.
	for i := 0; i < n; i++ {
		buffer[i] = 0
	}
	return nil
}