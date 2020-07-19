package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var HTTP_METHODS = map[string]struct{}{
	"GET":     struct{}{},
	"HEAD":    struct{}{},
	"POST":    struct{}{},
	"PUT":     struct{}{},
	"PATCH":   struct{}{},
	"DELETE":  struct{}{},
	"CONNECT": struct{}{},
	"OPTIONS": struct{}{},
	"TRACE":   struct{}{},
}

type Job struct {
	done   bool
	method string
	path   string
	target *url.URL
}

type Result struct {
	length int
	method string
	path   string
	status int
	target *url.URL
}

func main() {
	path, err := os.Executable()

	if err != nil {
		panic(err)
	}

	dir := filepath.Dir(path)

	var headers string
	var help bool
	var httpmethods string
	var insecure bool
	var maxerrors int
	var mindiff float64
	var nroutines int
	var statuscodes string
	var wordlist string

	flag.StringVar(&headers, "H", "", "comma-separated list/file with request headers")
	flag.StringVar(&httpmethods, "X", "GET", "comma-separated list of request methods to send (default: \"GET\")")
	flag.Float64Var(&mindiff, "d", 0.2, "minimum fractional difference in response payload length per host (default: 0.2)")
	flag.IntVar(&maxerrors, "e", 0, "print errors and exit after this many")
	flag.BoolVar(&help, "h", false, "show usage information and exit")
	flag.BoolVar(&insecure, "k", false, "allow insecure TLS connections")
	flag.IntVar(&nroutines, "n", 40, "number of goroutines to run (default: 40)")
	flag.StringVar(&statuscodes, "s", "200", "comma-separated list of acceptable status codes (default: \"200\")")
	flag.StringVar(&wordlist, "w", "", "wordlist of paths to try (required)")

	flag.Parse()

	if help {
		fmt.Fprintln(os.Stderr, `polypath [OPTIONS] <file>

Options:
  -H     <headers/@file>  comma-separated list/file with request headers
  -X     <methods>        comma-separated list of request methods to send (default: "GET")
  -d     <float>          minimum fractional difference in response payload length per host (default: 0.2)
  -e     <int>            print errors and exit after this many
  -h                      show usage information and exit
  -k                      allow insecure TLS connections
  -n     <int>            number of goroutines to run (default: 40)
  -s     <codes>          comma-separated list of acceptable status codes (default: "200")
  -w     <file>           wordlist of paths to try (required)`)

		os.Exit(0)
	}

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "[!] Expected one argument <file>")
		os.Exit(1)
	}

	if wordlist == "" {
		fmt.Fprintln(os.Stderr, "[!] No wordlist specified")
		os.Exit(1)
	}

	methods := strings.Split(httpmethods, ",")
	i := 0

	for _, method := range methods {
		method = strings.ToUpper(strings.Trim(method, " "))

		if method == "" {
			continue
		}

		if _, ok := HTTP_METHODS[method]; !ok {
			fmt.Fprintln(os.Stderr, "[!] Unrecognized HTTP method:", method)
			os.Exit(1)
		}

		methods[i] = method
		i++
	}

	methods = methods[:i]
	targetfile := flag.Arg(0)
	targetdata, err := ioutil.ReadFile(targetfile)

	if err != nil {
		fmt.Fprintln(os.Stderr, "[!] Couldn't read <file>")
		os.Exit(1)
	}

	targets := strings.Split(string(targetdata), "\n")
	ntargets := len(targets)
	targeturls := make([]*url.URL, ntargets, ntargets)
	i = 0

	for _, target := range targets {
		if target = strings.Trim(target, " "); target == "" {
			continue
		}

		targeturl, err := url.Parse(target)

		if err != nil || !targeturl.IsAbs() {
			fmt.Fprintln(os.Stderr, "[!] Invalid URL:", target)
			os.Exit(1)
		}

		target = targeturl.String()
		lastidx := len(target) - 1

		if target[lastidx] == 47 {
			target = target[:lastidx]
		}

		targeturls[i], _ = url.Parse(target)
		i++
	}

	targeturls = targeturls[:i]
	ntargets = len(targeturls)

	var headerlines []string

	if headers != "" {
		if strings.HasPrefix(headers, "@") {
			filename := string([]rune(headers)[1:])
			headerdata, err := ioutil.ReadFile(filename)

			if err != nil {
				fmt.Fprintln(os.Stderr, "[!] Can't find file with headers:", filename)
				os.Exit(1)
			}

			headerlines = strings.Split(string(headerdata), "\n")
		} else {
			headerlines = strings.Split(headers, ",")
		}
	}

	pathdata, err := ioutil.ReadFile(wordlist)

	if err != nil {
		fmt.Fprintln(os.Stderr, "[!] Can't find wordlist:", wordlist)
		os.Exit(1)
	}

	paths := strings.Split(string(pathdata), "\n")
	i = 0

	for _, path := range paths {
		path = strings.Trim(path, " ")

		if path != "" {
			if path[0] != 47 {
				path = "/" + path
			}

			paths[i] = path
			i++
		}
	}

	paths = paths[:i]
	strcodes := strings.Split(statuscodes, ",")
	ncodes := len(strcodes)
	codes := make([]int, ncodes, ncodes)

	for i, strcode := range strcodes {
		trimcode := strings.Trim(strcode, " ")
		code, err := strconv.Atoi(trimcode)

		if err != nil || code < 100 || code > 599 {
			fmt.Fprintln(os.Stderr, "[!] Invalid status code:", trimcode)
			os.Exit(1)
		}

		codes[i] = code
	}

	npaths := len(paths)
	nmethods := len(methods)
	nrequests := ntargets*npaths*nmethods + ntargets*nmethods
	banner, err := ioutil.ReadFile(filepath.Join(dir, "banner"))

	if err != nil {
		panic(err)
	}

	fmt.Fprintln(os.Stderr, string(banner))

	fmt.Fprintf(os.Stderr, "[-] Identified %d targets\n", ntargets)
	fmt.Fprintf(os.Stderr, "[-] Loaded %d paths\n", npaths)
	fmt.Fprintf(os.Stderr, "[-] Total requests: %d\n", nrequests)
	fmt.Fprintln(os.Stderr, "[-] Request methods:", strings.Join(methods, ","))
	fmt.Fprintln(os.Stderr, "[-] Status codes:", statuscodes)

	headermap := make(map[string]string)

	for _, line := range headerlines {
		kv := strings.SplitN(line, ":", 2)

		if len(kv) == 2 {
			key := strings.Trim(kv[0], " ")
			value := strings.Trim(kv[1], " ")
			headermap[key] = value

			fmt.Fprintf(os.Stderr, "[-] Request header > \"%s: %s\"\n", key, value)
		}
	}

	fmt.Fprintln(os.Stderr, "[-] Number of goroutines:", nroutines)
	fmt.Fprintln(os.Stderr, "[-] Minimum diff:", mindiff)

	client := &http.Client{Timeout: 3 * time.Second}
	jobs := make(chan *Job)
	errs := make(chan error)
	results := make(chan *Result)

	if insecure {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	log.SetOutput(ioutil.Discard)

	var wg sync.WaitGroup

	for i := 0; i < nroutines; i++ {
		go func() {
			for job := range jobs {
				if job.done {
					return
				}

				target := job.target.String()

				if job.path != "" {
					target += job.path
				}

				req, err := http.NewRequest(job.method, target, nil)

				if err != nil {
					errs <- err
					continue
				}

				for key, value := range headermap {
					req.Header.Add(key, value)
				}

				resp, err := client.Do(req)

				wg.Done()

				if err != nil {
					errs <- err
					continue
				}

				data, err := ioutil.ReadAll(resp.Body)
				resp.Body.Close()

				if err != nil {
					errs <- err
					continue
				}

				results <- &Result{
					length: len(data),
					method: job.method,
					path:   job.path,
					status: resp.StatusCode,
					target: job.target,
				}
			}
		}()
	}

	go func() {
		for _, method := range methods {
			wg.Add(ntargets)

			for _, targeturl := range targeturls {
				jobs <- &Job{
					method: method,
					target: targeturl,
				}
			}

			wg.Wait()

			fmt.Fprintf(os.Stderr, "[-] Finished %s reference requests\n", method)

			wg.Add(ntargets * npaths)

			for _, path := range paths {
				for _, targeturl := range targeturls {
					jobs <- &Job{
						method: method,
						path:   path,
						target: targeturl,
					}
				}
			}

			wg.Wait()
		}

		for i := 0; i < nroutines; i++ {
			jobs <- &Job{done: true}
		}

		close(jobs)
	}()

	var nerrors = 0
	var size string

	all_lengths := make(map[string][]int)

	for i := 0; i < nrequests; i++ {
		select {
		case res := <-results:
			target := res.target.String()
			host := res.target.Hostname()

			if res.path == "" {
				all_lengths[host] = []int{res.length}
				break
			}

			if res.length == 0 {
				break
			}

		loop:
			for _, code := range codes {
				if code == res.status {
					lengths, _ := all_lengths[host]

					for _, length := range lengths {
						diff := math.Abs(float64(length-res.length)) * 2 / float64(length+res.length)

						if diff < mindiff {
							break loop
						}
					}

					all_lengths[host] = append(lengths, res.length)

					if res.length > 1000000 {
						size = fmt.Sprintf("%.2fMB", float64(res.length)/1000000)
					} else if res.length > 1000 {
						size = fmt.Sprintf("%.2fKB", float64(res.length)/1000)
					} else {
						size = fmt.Sprintf("%dB", res.length)
					}

					fmt.Printf("%d - %s %s (%s)\n", res.status, res.method, target+res.path, size)

					break loop
				}
			}

		case err := <-errs:
			if maxerrors == 0 {
				break
			}

			fmt.Fprintf(os.Stderr, "[!] %v\n", err)

			nerrors++

			if nerrors == maxerrors {
				fmt.Fprintln(os.Stderr, "[!] Reached max number of errors")
				fmt.Fprintln(os.Stderr, "[!] Exiting")
				os.Exit(1)
			}
		}
	}

	close(errs)
	close(results)

	fmt.Fprintln(os.Stderr, "[-] Done!")
}
