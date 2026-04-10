package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var httpClient = &http.Client{
	Transport: &http.Transport{
		ForceAttemptHTTP2: false,
	},
	Timeout: 30 * time.Second,
}

func doJSON(method, endpoint, apiKey string, body interface{}) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, endpoint, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return httpClient.Do(req)
}

func doJSONWithHeader(method, endpoint, apiKey string, body interface{}, headerKey, headerVal string) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, endpoint, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set(headerKey, headerVal)
	return httpClient.Do(req)
}

// printOut writes indented JSON to stdout.
func printOut(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// exitErr writes a JSON error to stderr and exits 1.
func exitErr(msg string) {
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]string{"error": msg})
	os.Exit(1)
}

// handleResp decodes the response body. Errors go to stderr; success to stdout.
func handleResp(resp *http.Response) {
	defer resp.Body.Close()
	var result interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		exitErr(fmt.Sprintf("failed to decode response: %v", err))
	}
	if resp.StatusCode == 401 {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "API Key 已失效，请访问社区网站个人中心重置:")
		fmt.Fprintf(os.Stderr, "  %s/#/me\n", getBaseURL())
		fmt.Fprintln(os.Stderr, "")
		os.Exit(1)
	}
	if resp.StatusCode >= 400 {
		enc := json.NewEncoder(os.Stderr)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		os.Exit(1)
	}
	printOut(result)
}
