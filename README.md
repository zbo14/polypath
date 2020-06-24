# polypath

Request a lot of paths across many web hosts using different HTTP methods!

This was inspired by tomnomnom's tool, [meg](https://github.com/tomnomnom/meg).

## Install

`$ go get github.com/zbo14/polypath`

## Usage

```
polypath [OPTIONS] <file>

Options:
  -H     <headers/@file>  comma-separated list/file with request headers
  -X     <method>         comma-separated list of request methods to send (default: "GET")
  -d     <float>          minimum fractional difference in response payload length per host (default: 0.2)
  -e     <int>            print errors and exit after this many
  -h                      show usage information and exit
  -k                      allow insecure TLS connections
  -n     <int>            number of goroutines to run (default: 40)
  -s     <codes>          comma-separated whitelist of status codes (default: "200")
  -w     <file>           wordlist of paths to try (required)
```
