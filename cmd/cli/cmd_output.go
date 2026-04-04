package main

import (
	"encoding/json"
	"os"
)

// jsonMode is set by the global --json/-j flag.
var jsonMode bool

// isJSON returns true when --json mode is enabled.
func isJSON() bool {
	return jsonMode
}

// outputJSON writes data as indented JSON to stdout.
func outputJSON(data interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}

// outputJSONError writes {"error": msg} to stderr and exits 1.
func outputJSONError(msg string) {
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]string{"error": msg})
	os.Exit(1)
}
