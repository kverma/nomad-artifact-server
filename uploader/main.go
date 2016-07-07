package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
)

var (
	baseuri  string
	jobid    string
	filename string
)

// readFromStdin returns everything in stdin.
func readFromStdin() []byte {
	stdin, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		logrus.Fatalf("reading from stdin failed: %v", err)
	}
	return stdin
}

// readFromFile returns the contents of a file.
func readFromFile(filename string) []byte {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		logrus.Fatalf("No such file or directory: %q", filename)
	}

	file, err := ioutil.ReadFile(filename)
	if err != nil {
		logrus.Fatalf("reading from file %q failed: %v", filename, err)
	}
	return file
}

// postJobFile uploads the file content to the server
// and returns the JobID and URI.
func postJobFile(content []byte, fileName string, jobid string) (string, string, error) {
	// create the request
	req, err := http.NewRequest("POST", baseuri+"jobs", bytes.NewBuffer(content))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-JOB-FILENAME", fileName)
	if jobid != "" {
		req.Header.Set("X-JOB-ID", jobid)
	}

	// do the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("request to %sjobs failed: %v", baseuri, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("reading response body failed: %v", err)
	}

	var response map[string]string
	if err = json.Unmarshal(body, &response); err != nil {
		return "", "", fmt.Errorf("parsing body as json failed: %v", err)
	}

	if respError, ok := response["error"]; ok {
		return "", "", fmt.Errorf("server responded with %s", respError)
	}

	uploadedFileUri, ok := response["uri"]
	if !ok {
		return "", "", fmt.Errorf("Upload Error - Server Response %s", string(body))
	}

	jobId, ok := response["jobId"]
	if !ok {
		return "", "", fmt.Errorf("Upload Error - Server Response %s", string(body))
	}

	return uploadedFileUri, jobId, nil
}

func init() {
	flag.StringVar(&baseuri, "b", "http://localhost/", "server base url")
	flag.StringVar(&jobid, "j", "", "jobid to use for upload, if not set, server will assign one for future use.")
	flag.Parse()

	// make sure uri ends with trailing /
	if !strings.HasSuffix(baseuri, "/") {
		baseuri += "/"
	}
	// make sure it starts with http(s)://
	if !strings.HasPrefix(baseuri, "http") {
		baseuri = "http://" + baseuri
	}
}

func main() {
	args := flag.Args()

	// check if we are reading from a file or stdin
	var content []byte
	if len(args) == 0 {
		filename = "tempfile"
		content = readFromStdin()
	} else {
		tempfilename := args[0]
		filename = url.QueryEscape(tempfilename)
		content = readFromFile(tempfilename)
	}

	fileUrl, jobId, err := postJobFile(content, filename, jobid)
	if err != nil {
		logrus.Fatal(err)
	}

	fmt.Printf("Your File has been uploaded successfully \n[JobId]: %s\n[URL]: %s\n", jobId, fileUrl)
}
