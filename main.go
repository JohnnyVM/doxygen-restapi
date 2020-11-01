package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var doxygen_template_conf string = "template.conf"

func createDoxygenConf(tmpdir string) (string, error) {
	tmpfile, err := os.Create(filepath.Join(tmpdir, "doxygen.conf"))
	if err != nil {
		return "", err
	}
	defer tmpfile.Close()
	doxygen_template_conf_env, ok := os.LookupEnv("DOXYGEN_TEMPLATE_CONF")
	if ok {
		log.Print("Setting template from environment variable")
		doxygen_template_conf = doxygen_template_conf_env
	}
	content, err := ioutil.ReadFile(doxygen_template_conf)
	if err != nil {
		return "", err
	}
	_, err = tmpfile.Write(content)
	if err != nil {
		return "", err
	}
	return tmpfile.Name(), nil
}

func untar(tarball io.Reader, target string) error {
	tarReader := tar.NewReader(tarball)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		path := filepath.Join(target, header.Name)
		info := header.FileInfo()
		if info.IsDir() {
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return err
			}
			continue
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(file, tarReader)
		if err != nil {
			return err
		}
	}
	return nil
}

func tarball(source, target string) (string, error) {
	filename := filepath.Base(source)
	target = filepath.Join(target, fmt.Sprintf("%s.tar", filename))
	tarfile, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer tarfile.Close()

	tarball := tar.NewWriter(tarfile)
	defer tarball.Close()

	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	return target, filepath.Walk(source,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return nil
			}

			if baseDir != "" {
				header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
			}

			if err := tarball.WriteHeader(header); err != nil {
				return nil
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer file.Close()
			_, err = io.Copy(tarball, file)
			return nil
		})
}

func Gzip(source, target string) (string, error) {
	reader, err := os.Open(source)
	if err != nil {
		return "", err
	}

	filename := filepath.Base(source)
	target = filepath.Join(target, fmt.Sprintf("%s.gz", filename))
	writer, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer writer.Close()

	archiver := gzip.NewWriter(writer)
	archiver.Name = filename
	defer archiver.Close()

	_, err = io.Copy(archiver, reader)
	return target, err
}

func handler(w http.ResponseWriter, r *http.Request) {
	log.Println("Request arrive")
	if r.Header.Get("Content-Type") != "application/tar+gzip" {
		log.Println("Error bad headers")
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}
	tmpdir, err := ioutil.TempDir("", "doxygen-")
	if err != nil {
		log.Println("Error createing temp dir: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	//defer os.RemoveAll(tmpdir)
	log.Print("New dir: ", tmpdir)
	doxyconf, err := createDoxygenConf(tmpdir)
	if err != nil {
		log.Println("Error creating conf file: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	inputdir := filepath.Join(tmpdir, "input")
	err = os.Mkdir(inputdir, 0755)
	if err != nil {
		log.Println("Error createing temp input dir: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	outputdir := filepath.Join(tmpdir, "html")
	err = os.Mkdir(outputdir, 0755)
	if err != nil {
		log.Println("Error createing temp output dir: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	content, err := ioutil.ReadFile(doxyconf)
	if err != nil {
		log.Println("Error opening config file: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	input_regexp := regexp.MustCompile("(?m)^\\s*INPUT\\s*=.*$")
	output_regexp := regexp.MustCompile("(?m)^\\s*HTML_OUTPUT\\s*=.*$")
	memory_file := input_regexp.ReplaceAll(content, []byte(fmt.Sprint("INPUT = ", inputdir)))
	memory_file = output_regexp.ReplaceAll(memory_file, []byte(fmt.Sprint("HTML_OUTPUT = ", outputdir)))
	err = ioutil.WriteFile(doxyconf, memory_file, 0644)
	if err != nil {
		log.Println("Error opening config(out) file: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	ungzip, err := gzip.NewReader(r.Body)
	if err != nil {
		log.Println("Error unzipping body: ", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer ungzip.Close()
	err = untar(ungzip, inputdir)
	if err != nil {
		log.Println("Error untar file: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	rundock := exec.Command("docker", "run", "-v", "/tmp:/tmp", "--rm", "johnnyvm90/doxygen", doxyconf)
	err = rundock.Run()
	if err != nil {
		log.Println("Error docker: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tarfile, err := tarball(outputdir, tmpdir)
	if err != nil {
		log.Println("Error creating Tar: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	gzipfile, err := Gzip(tarfile, tmpdir)
	if err != nil {
		log.Println("Error creating Zip: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Print("Gzip file created: ", gzipfile)
	zfile, err := ioutil.ReadFile(gzipfile)
	if err != nil {
		log.Println("Error reading gzip: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/tar+gzip")
	w.Write(zfile)

	w.WriteHeader(http.StatusOK)
}

func main() {
	http.HandleFunc("/doxygen", handler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
