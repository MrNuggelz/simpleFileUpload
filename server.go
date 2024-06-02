package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ConfigValues struct {
	port           string
	maxSizeMB      int64
	uploadDir      string
	postProcessing string
}

func main() {
	config := parseConfigArguments()
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleUpload(config, w, r)
	})

	log.Println("starting server on :" + config.port)
	http.ListenAndServe(":"+config.port, nil)
}

func parseConfigArguments() ConfigValues {
	var maxSizeMB = flag.Int64("maxSizeMB", 32, "Maximum allowed file size in MB")
	var port = flag.String("port", "8080", "Port the server runs on")
	var uploadDirFlag = flag.String("uploadDir", "", "Specifies where the files will be uploaded to")
	var postProcessing = flag.String("postProcessing", "", "Post processing executable (with parameters). %s will be replaced with the name of the uploaded file")
	flag.Parse()

	if *maxSizeMB < 1 {
		log.Fatal("maxSizeMB must be larger than 0")
	}

	uploadDir := *uploadDirFlag
	if uploadDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatal("Unabled to get current work directory:", err)
		}
		uploadDir = cwd
	}
	log.Println("maxSizeMB is set to", *maxSizeMB)
	log.Println("port is set to", *port)
	log.Println("uploadDir is set to", uploadDir)
	log.Println("postProcessing is set to", *postProcessing)

	return ConfigValues{port: *port, maxSizeMB: *maxSizeMB, uploadDir: uploadDir, postProcessing: *postProcessing}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Add("Contexnt-Type", "text/html")
	http.ServeFile(w, r, "index.html")
}

func handleUpload(config ConfigValues, w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(config.maxSizeMB << 20); err != nil {
		log.Println(err)
		http.Error(w, "Upload size to big", http.StatusRequestEntityTooLarge)
		return
	}

	for _, fileHeader := range r.MultipartForm.File["files"] {
		log.Println("Processing ", fileHeader.Filename)

		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "Unable to open file", http.StatusInternalServerError)
			continue
		}
		defer file.Close()

		uploadPath := filepath.Join(config.uploadDir, fileHeader.Filename)

		if _, err := os.Stat(uploadPath); err == nil {
			log.Println("File already exists ", uploadPath)
			http.Error(w, "File already exists "+fileHeader.Filename, http.StatusConflict)
		} else if errors.Is(err, os.ErrNotExist) {
			outFile, err := os.Create(uploadPath)
			if err != nil {
				log.Println("Unable to create file", uploadPath)
				http.Error(w, "Unable to create file", http.StatusInternalServerError)
				continue
			}
			defer file.Close()

			if _, err := io.Copy(outFile, file); err != nil {
				log.Println("Unable to copy to file", uploadPath)
				http.Error(w, "Unable to copy to file", http.StatusInternalServerError)
				continue
			}

			fmt.Fprintf(w, "Uploaded %s\n", fileHeader.Filename)
			log.Printf("Uploaded %s\n", fileHeader.Filename)

			if config.postProcessing != "" {
				splits := strings.SplitN(config.postProcessing, " ", 2)
				cmd := splits[0]
				args := fmt.Sprintf(splits[1], fileHeader.Filename)

				if err := exec.Command(cmd, args).Run(); err != nil {
					log.Printf("Error running post processing for %s: %s", fileHeader.Filename, err)
					log.Printf("Command was %s %s", cmd, args)
					http.Error(w, "Unable run post processing for "+fileHeader.Filename, http.StatusInternalServerError)
					continue
				}
			}
		} else {
			http.Error(w, "Error uploading file:"+fileHeader.Filename, http.StatusInternalServerError)
			continue
		}
	}
}
