package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIs map[string]API `yaml:"apis"`
}

type API struct {
	Base    string  `yaml:"base"`
	Patches []Patch `yaml:"patches"` // these patches are applied to all files
	Files   []File  `yaml:"files"`
}

type File struct {
	Name    string  `yaml:"name"`
	Patches []Patch `yaml:"patches"`
}

type Patch struct {
	Replace string `yaml:"replace"`
	With    string `yaml:"with"`
}

func main() {
	_, mainfilepath, _, _ := runtime.Caller(0)
	curPath := path.Dir(mainfilepath)
	yamlBytes, err := os.ReadFile(filepath.Join(curPath, "apis.yaml"))
	if err != nil {
		log.Fatal(err)
	}

	externalapisDir := filepath.Join(curPath, "..", "..", "api", "external")
	if err := os.RemoveAll(externalapisDir); err != nil {
		log.Fatal(err)
	}

	config := &Config{}
	if err := yaml.Unmarshal(yamlBytes, config); err != nil {
		log.Fatal(err)
	}

	for name, api := range config.APIs {
		for _, file := range api.Files {
			destination := path.Join(externalapisDir, name, file.Name)
			url := fmt.Sprintf("%s/%s", api.Base, file.Name)

			if err := downloadFile(url, destination); err != nil {
				log.Fatal(err)
			}

			for _, patch := range file.Patches {
				if err := applyPatch(destination, patch); err != nil {
					log.Fatal(err)
				}
			}
			for _, patch := range api.Patches {
				if err := applyPatch(destination, patch); err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}

func downloadFile(url, destination string) error {
	destinationDir := filepath.Dir(destination)
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		return err
	}

	resp, err := http.Get(url)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return err
	}

	file, err := os.Create(destination)
	if file != nil {
		defer file.Close()
	}
	if err != nil {
		return err
	}

	_, err = io.Copy(file, resp.Body)
	return err
}

func applyPatch(file string, patch Patch) error {
	fileBytes, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	replaced := strings.ReplaceAll(string(fileBytes), patch.Replace, patch.With)
	return os.WriteFile(file, []byte(replaced), 0o644)
}
