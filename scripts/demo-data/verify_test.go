package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type verificationTransport func(*http.Request) (*http.Response, error)

func (f verificationTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestVerificationRequiresUIResponseEnvelope(t *testing.T) {
	for _, test := range []struct {
		name, body string
		passed     bool
	}{
		{"code zero", `{"code":0,"data":{"total":30,"data":[{},{}]}}`, true},
		{"bare list", `{"total":30,"data":[{},{}]}`, false},
		{"code 200 incompatible", `{"code":200,"data":{"total":30,"data":[{},{}]}}`, false},
		{"code failure", `{"code":100,"data":{"total":30,"data":[{},{}]}}`, false},
		{"null rows", `{"code":0,"data":{"total":30,"data":null}}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: verificationTransport(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host != "127.0.0.1:5666" || req.Method != "POST" {
					t.Fatal("request escaped local read-only endpoint")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(test.body)), Header: make(http.Header)}, nil
			})}
			result := runAPICheck(client, "test-only", apiCheck{entity: "test", method: "POST", path: "/sys-api/user/list", minimum: 2})
			if result.Passed != test.passed {
				t.Fatalf("got passed=%t failure=%s", result.Passed, result.Failure)
			}
		})
	}
}

func TestVerificationListShapes(t *testing.T) {
	for _, test := range []struct {
		name        string
		payload     any
		tree        bool
		rows, total int
		found       bool
	}{
		{"array", []any{Row{}, Row{}}, false, 2, 2, true},
		{"paged", Row{"total": 30, "data": []any{Row{}, Row{}}}, false, 2, 30, true},
		{"nested", Row{"data": Row{"total": "30", "data": []any{Row{}, Row{}}}}, false, 2, 30, true},
		{"session uppercase", Row{"Total": 30, "Items": []any{Row{}, Row{}}}, false, 2, 30, true},
		{"tree", Row{"total": 1, "data": []any{Row{"children": []any{Row{}, Row{}}}}}, true, 3, 3, true},
		{"null", nil, false, 0, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows, total, found := apiListCounts(test.payload, test.tree)
			if rows != test.rows || total != test.total || found != test.found {
				t.Fatalf("got %d/%d/%t", rows, total, found)
			}
		})
	}
}
