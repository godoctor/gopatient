// Copyright 2014 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/godoctor/gopatient/cmd/gopatient-helper-download/internal/github.com/cheggaaa/pb"
	_ "github.com/godoctor/gopatient/cmd/gopatient-helper-download/internal/github.com/mattn/go-sqlite3"
)

var (
	coresFlag = flag.Int("p", runtime.NumCPU(),
		"Parallelism: number of threads to run simultaneously (default: number of cores)")
	countFlag = flag.Int("n", -1,
		"Number of repositories to download (multiple of 50)")
)

func main() {
	flag.Parse()
	if *countFlag < 1 {
		flag.PrintDefaults()
		os.Exit(2)
	} else if *countFlag%50 != 0 {
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "Number of repositories must be a multiple of 50")
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}

	gopath := os.Getenv("GOPATH")
	gopath, err = filepath.Abs(gopath)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	if cwd != gopath {
		fmt.Fprintf(os.Stderr, "This must be run from the root of the GOPATH (%s)\n", gopath)
		os.Exit(1)
	}

	runtime.GOMAXPROCS(*coresFlag)
	getgit()
	return
}

// getgit uses the GitHub API to identify repositories using Go that have the
// best star rating, then uses "go get -u -t -fix" to clone them and their
// dependencies
func getgit() {
	fmt.Fprintln(os.Stderr, "Cloning repostories from GitHub...")

	repos := make(chan string)
	bar := pb.StartNew(*countFlag)
	bar.Output = os.Stderr
	defer bar.Finish()

	go func() {
		defer close(repos)
		for i := 1; i <= *countFlag/50; i++ {
			resp, err := http.Get("https://api.github.com/search/repositories?q=+language:go&sort=stars&per_page=50&page=" + strconv.Itoa(i))

			if err != nil {
				fmt.Println(err)
			}

			info := struct {
				Items []map[string]interface{} `json:"items"`
			}{}

			err = json.NewDecoder(resp.Body).Decode(&info)

			if err != nil {
				fmt.Println(err)
			}

			for _, v := range info.Items {
				repos <- v["full_name"].(string)
			}
		}
	}()

	var wait sync.WaitGroup
	pipes := 50
	wait.Add(pipes)

	for i := 0; i < pipes; i++ {
		go func() {
			defer wait.Done()
			for r := range repos {
				//fmt.Println("getting github.com/" + r)
				c := make(chan string, 1)
				go func() {
					// sending all output for debugging, uncomment in select to see
					out, err := exec.Command("go", "get", "-u", "-t", "-fix", "github.com/"+r).CombinedOutput()
					if err != nil {
						c <- string(out)
					} else {
						c <- ""
					}
				}()
				select {
				case <-c:
					// uncomment to see error messages from "go get" -- most errs aren't actually an issue
					//if err != "" {
					//fmt.Println(err)
					//}
				case <-time.After(time.Duration(30) * time.Second): // timed out
				}
				//fmt.Println("got github.com/" + r)
				bar.Increment()
			}
		}()
	}

	wait.Wait()
}
