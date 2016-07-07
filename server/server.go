package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
)

var (
	baseuri  string
	port     string
	storage  string
	certFile string
	keyFile  string
	filename string
)

// JSONResponse is a map[string]string
// response from the web server.
type JSONResponse map[string]string

// String returns the string representation of the
// JSONResponse object.
func (j JSONResponse) String() string {
	str, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{
			"error": "%v"
		}`, err)
	}

	return string(str)
}

// pasteUploadHander is the request handler for /paste
// it creates a uuid for the paste and saves the contents of
// the paste to that file.
func jobsHandler(w http.ResponseWriter, r *http.Request) {

	subDirName := "output"
	// create a unique id for the paste
	uniqId, err := uuid()
	if err != nil {
		writeError(w, fmt.Sprintf("uuid generation failed: %v", err))
		return
	}
	// set the content type and check to make sure they are POST-ing a paste
	w.Header().Set("Content-Type", "application/json")
	if r.Method != "POST" {
		writeError(w, "not a valid endpoint")
		return
	}

	jobFileName := r.Header.Get("X-JOB-FILENAME")

	jobId := r.Header.Get("X-JOB-ID")
	if jobId == "" {
		jobId = uniqId
		subDirName = "input"

	}

	// read the body of the paste
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		writeError(w, fmt.Sprintf("reading from body failed: %v", err))
		return
	}

	//	filename = string(byteheader)
	// write to file
	indir := filepath.Join(storage, jobId, "input")
	outdir := filepath.Join(storage, jobId, "output")
	os.MkdirAll(indir, os.ModeDir)
	os.MkdirAll(outdir, os.ModeDir)

	file := filepath.Join(storage, jobId, subDirName, jobFileName)
	if err := ioutil.WriteFile(file, content, 0755); err != nil {
		writeError(w, fmt.Sprintf("writing file to %q failed: %v", file, err))
		return
	}

	// serve the uri for the paste to the requester
	fmt.Fprint(w, JSONResponse{
		"jobId":            jobId,
		"uploadedFileName": jobFileName,
		"uri":              baseuri + jobId + "/" + subDirName + "/" + jobFileName,
	})
	logrus.Infof("jobFile %q uploaded successfully", jobId)
	return
}

// uuid generates a uuid for the paste.
// This really does not need to be perfect.
func uuid() (string, error) {
	var chars = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

	length := 8
	b := make([]byte, length)
	r := make([]byte, length+(length/4))
	maxrb := 256 - (256 % len(chars))
	i := 0
	for {
		if _, err := io.ReadFull(rand.Reader, r); err != nil {
			return "", err
		}
		for _, rb := range r {
			c := int(rb)
			if c > maxrb {
				continue
			}
			b[i] = chars[c%len(chars)]
			i++
			if i == length {
				return string(b), nil
			}
		}
	}
}

// writeError sends an error back to the requester
// and also logs the error.
func writeError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, JSONResponse{
		"error": msg,
	})
	logrus.Printf("writing error: %s", msg)
	return
}

func init() {
	flag.StringVar(&baseuri, "b", "http://localhost/", "url base for this domain")
	flag.StringVar(&port, "p", "80", "port for server to run on")
	flag.StringVar(&storage, "s", "./storage/", "directory to store pastes")
	flag.Parse()

	// ensure uri has trailing slash
	if !strings.HasSuffix(baseuri, "/") {
		baseuri += "/"
	}
}

func main() {
	// create the storage directory
	if err := os.MkdirAll(storage, 0755); err != nil {
		logrus.Fatalf("creating storage directory %q failed: %v", storage, err)
	}

	// create mux server
	mux := http.NewServeMux()

	// static file server
	staticHandler := http.StripPrefix("/", http.FileServer(http.Dir(storage)))
	mux.Handle("/", staticHandler)

	// pastes & view handlers
	mux.HandleFunc("/jobs", jobsHandler) // paste upload handler

	// set up the server
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	logrus.Infof("Starting server on port %q with baseuri %q", port, baseuri)
	if certFile != "" && keyFile != "" {
		logrus.Fatal(server.ListenAndServeTLS(certFile, keyFile))
	} else {
		logrus.Fatal(server.ListenAndServe())
	}
}
