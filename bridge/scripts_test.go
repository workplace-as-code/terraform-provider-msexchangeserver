package bridge

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ifElseEmptyRe matches `Key = if (...) { ... } else { @() }`. PowerShell
// unrolls the output of the if statement: an empty list becomes $null and a
// one-item list a single value, so the JSON has null (or {} in Windows PowerShell 5.1), "a" or ["a","b"]
// instead of an array.
var ifElseEmptyRe = regexp.MustCompile(`=\s*if\s*\(.*\}\s*else\s*\{\s*@\(\)\s*\}`)

// TestScriptListsAreArrays keeps the lists of the scripts' JSON output arrays:
// write `Key = @(if (...) { ... })`, which is always an array.
func TestScriptListsAreArrays(t *testing.T) {
	files, err := filepath.Glob("scripts/*.ps1")
	if err != nil || len(files) == 0 {
		t.Fatalf("no bridge scripts found: %v", err)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "#") && ifElseEmptyRe.MatchString(line) {
				t.Errorf("%s:%d: write Key = @(if (...) { ... }): with else { @() } an empty list is null in JSON",
					f, i+1)
			}
		}
	}
}

// retryBlockRe matches Invoke-BridgeRetry and its block, when the block has no
// nested braces.
var retryBlockRe = regexp.MustCompile(`Invoke-BridgeRetry((?: -[A-Za-z]+)*) \{([^{}]*)\}`)

// creationRe matches the Exchange cmdlets that create an object.
var creationRe = regexp.MustCompile(`\b(?:New|Add)-(?:[A-Z][A-Za-z]+|\$[A-Za-z]+)`)

// TestCreationsAreNotResent: a creation sent again after a lost answer fails
// on the object the first attempt created. Invoke-BridgeRetry -Create only
// retries the errors that say the request was not processed.
func TestCreationsAreNotResent(t *testing.T) {
	files, err := filepath.Glob("scripts/*.ps1")
	if err != nil || len(files) == 0 {
		t.Fatalf("no bridge scripts found: %v", err)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range retryBlockRe.FindAllStringSubmatch(string(data), -1) {
			if c := creationRe.FindString(m[2]); c != "" && !strings.Contains(m[1], "-Create") {
				t.Errorf("%s: %s runs in Invoke-BridgeRetry without -Create", f, c)
			}
		}
	}
}
