// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
//               2022 Unikraft GmbH.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package httpclient

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/henvic/httpretty"
	"go.unikraft.io/kit/internal/httpunix"
	"go.unikraft.io/kit/internal/version"
	"go.unikraft.io/kit/iostreams"
)

// ClientOption represents an argument to NewClient
type ClientOption = func(http.RoundTripper) http.RoundTripper

var timezoneNames = map[int]string{
	-39600: "Pacific/Niue",
	-36000: "Pacific/Honolulu",
	-34200: "Pacific/Marquesas",
	-32400: "America/Anchorage",
	-28800: "America/Los_Angeles",
	-25200: "America/Chihuahua",
	-21600: "America/Chicago",
	-18000: "America/Bogota",
	-14400: "America/Caracas",
	-12600: "America/St_Johns",
	-10800: "America/Argentina/Buenos_Aires",
	-7200:  "Atlantic/South_Georgia",
	-3600:  "Atlantic/Cape_Verde",
	0:      "Europe/London",
	3600:   "Europe/Amsterdam",
	7200:   "Europe/Athens",
	10800:  "Europe/Istanbul",
	12600:  "Asia/Tehran",
	14400:  "Asia/Dubai",
	16200:  "Asia/Kabul",
	18000:  "Asia/Tashkent",
	19800:  "Asia/Kolkata",
	20700:  "Asia/Kathmandu",
	21600:  "Asia/Dhaka",
	23400:  "Asia/Rangoon",
	25200:  "Asia/Bangkok",
	28800:  "Asia/Manila",
	31500:  "Australia/Eucla",
	32400:  "Asia/Tokyo",
	34200:  "Australia/Darwin",
	36000:  "Australia/Brisbane",
	37800:  "Australia/Adelaide",
	39600:  "Pacific/Guadalcanal",
	43200:  "Pacific/Nauru",
	46800:  "Pacific/Auckland",
	49500:  "Pacific/Chatham",
	50400:  "Pacific/Kiritimati",
}

// generic authenticated HTTP client for commands
func NewHTTPClient(io *iostreams.IOStreams, unixSocket string, setAccept bool) (*http.Client, error) {
	var opts []ClientOption

	// We need to check and potentially add the unix socket roundtripper option
	// before adding any other options, since if we are going to use the unix
	// socket transport, it needs to form the base of the transport chain
	// represented by invocations of opts...
	//
	// Another approach might be to change the signature of api.NewHTTPClient to
	// take an explicit base http.RoundTripper as its first parameter (it
	// currently defaults internally to http.DefaultTransport), or add another
	// variant like api.NewHTTPClientWithBaseRoundTripper. But, the only caller
	// which would use that non-default behavior is right here, and it doesn't
	// seem worth the cognitive overhead everywhere else just to serve this one
	// use case.
	if unixSocket != "" {
		opts = append(opts, ClientOption(func(http.RoundTripper) http.RoundTripper {
			return httpunix.NewRoundTripper(unixSocket)
		}))
	}

	if verbose := os.Getenv("DEBUG"); verbose != "" {
		logTraffic := strings.Contains(verbose, "api")
		opts = append(opts, VerboseLog(io.ErrOut, logTraffic, io.IsStderrTTY()))
	}

	opts = append(opts,
		AddHeader("User-Agent", fmt.Sprintf("KraftKit CLI %s", version.Version())),
		AddHeaderFunc("Time-Zone", func(req *http.Request) (string, error) {
			if req.Method != "GET" && req.Method != "HEAD" {
				if time.Local.String() != "Local" {
					return time.Local.String(), nil
				}
				_, offset := time.Now().Zone()
				return timezoneNames[offset], nil
			}
			return "", nil
		}),
	)

	tr := http.DefaultTransport
	for _, opt := range opts {
		tr = opt(tr)
	}
	return &http.Client{Transport: tr}, nil
}

// AddHeader turns a RoundTripper into one that adds a request header
func AddHeader(name, value string) ClientOption {
	return func(tr http.RoundTripper) http.RoundTripper {
		return &funcTripper{roundTrip: func(req *http.Request) (*http.Response, error) {
			if req.Header.Get(name) == "" {
				req.Header.Add(name, value)
			}
			return tr.RoundTrip(req)
		}}
	}
}

// AddHeaderFunc is an AddHeader that gets the string value from a function
func AddHeaderFunc(name string, getValue func(*http.Request) (string, error)) ClientOption {
	return func(tr http.RoundTripper) http.RoundTripper {
		return &funcTripper{roundTrip: func(req *http.Request) (*http.Response, error) {
			if req.Header.Get(name) != "" {
				return tr.RoundTrip(req)
			}
			value, err := getValue(req)
			if err != nil {
				return nil, err
			}
			if value != "" {
				req.Header.Add(name, value)
			}
			return tr.RoundTrip(req)
		}}
	}
}

// VerboseLog enables request/response logging within a RoundTripper
func VerboseLog(out io.Writer, logTraffic bool, colorize bool) ClientOption {
	logger := &httpretty.Logger{
		Time:            true,
		TLS:             false,
		Colors:          colorize,
		RequestHeader:   logTraffic,
		RequestBody:     logTraffic,
		ResponseHeader:  logTraffic,
		ResponseBody:    logTraffic,
		Formatters:      []httpretty.Formatter{&httpretty.JSONFormatter{}},
		MaxResponseBody: 10000,
	}
	logger.SetOutput(out)
	logger.SetBodyFilter(func(h http.Header) (skip bool, err error) {
		return !inspectableMIMEType(h.Get("Content-Type")), nil
	})
	return logger.RoundTripper
}

// ReplaceTripper substitutes the underlying RoundTripper with a custom one
func ReplaceTripper(tr http.RoundTripper) ClientOption {
	return func(http.RoundTripper) http.RoundTripper {
		return tr
	}
}

// ExtractHeader extracts a named header from any response received by this
// client and, if non-blank, saves it to dest.
func ExtractHeader(name string, dest *string) ClientOption {
	return func(tr http.RoundTripper) http.RoundTripper {
		return &funcTripper{roundTrip: func(req *http.Request) (*http.Response, error) {
			res, err := tr.RoundTrip(req)
			if err == nil {
				if value := res.Header.Get(name); value != "" {
					*dest = value
				}
			}
			return res, err
		}}
	}
}

type funcTripper struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (tr funcTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return tr.roundTrip(req)
}

// Client facilitates making HTTP requests to the GitHub API
type Client struct {
	http *http.Client
}

func (c *Client) HTTP() *http.Client {
	return c.http
}

var jsonTypeRE = regexp.MustCompile(`[/+]json($|;)`)

func inspectableMIMEType(t string) bool {
	return strings.HasPrefix(t, "text/") || jsonTypeRE.MatchString(t)
}
