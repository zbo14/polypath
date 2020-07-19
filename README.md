# polypath

Request a lot of paths across many web hosts using different HTTP methods!

`polypath` reports a response from a host when the payload length differs a certain magnitude from previous payloads the host sent.

This was inspired by tomnomnom's tool, [meg](https://github.com/tomnomnom/meg).

## Install

`$ go get github.com/zbo14/polypath`

## Usage

```
polypath [OPTIONS] <file>

Options:
  -H     <headers/@file>  comma-separated list/file with request headers
  -X     <methods>        comma-separated list of request methods to send (default: "GET")
  -d     <float>          minimum fractional difference in response payload length per host (default: 0.2)
  -e     <int>            print errors and exit after this many
  -h                      show usage information and exit
  -k                      allow insecure TLS connections
  -n     <int>            number of goroutines to run (default: 40)
  -s     <codes>          comma-separated list of acceptable status codes (default: "200")
  -w     <file>           wordlist of paths to try (required)
```

`polypath` expects a `<file>` of URLs and a wordlist of paths (`-w`). Then it iterates over the specified request methods (`-X`), sending a request to each host for a single path. It covers each host before moving to the next path, and each path before moving to the next request method (if any).

`polypath` only reports responses with payload lengths that are "different enough". For each request method, it keeps a record of response payload lengths from each host. When it receives a response with a payload length that is "different enough" from previous ones, it prints the result to stdout. You can tune this paramter with the `-d` option. For instance, increasing the fraction requires a larger difference to report a response. Lowering it to 0 requires no difference at all.
