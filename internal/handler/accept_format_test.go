// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package handler

import (
	"net/http/httptest"
	"testing"
)

func TestNegotiatedFormatAccept(t *testing.T) {
	cases := []struct {
		name, accept, want string
	}{
		{"empty", "", "json"},
		{"fhir json", "application/fhir+json", "json"},
		{"fhir xml", "application/fhir+xml", "xml"},
		{"mixed case xml (#95)", "Application/FHIR+XML", "xml"},
		{"mixed case turtle", "Text/Turtle", "turtle"},
		{"q-preferred json (#96)", "application/fhir+xml;q=0.5, application/fhir+json;q=0.9", "json"},
		{"xml q=0 never wins (#96)", "application/fhir+xml;q=0, application/fhir+json", "json"},
		{"q-preferred xml", "application/fhir+json;q=0.1, application/fhir+xml", "xml"},
		{"tie keeps header order", "application/fhir+json, application/fhir+xml", "json"},
		{"browser default", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", "xml"},
		{"wildcard", "*/*", "json"},
		{"unservable only", "text/html", "json"},
		{"uppercase Q param", "application/fhir+xml;Q=0", "json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/fhir/r4/Patient", nil)
			if tc.accept != "" {
				r.Header.Set("Accept", tc.accept)
			}
			if got := negotiatedFormat(r); got != tc.want {
				t.Fatalf("Accept %q: got %q, want %q", tc.accept, got, tc.want)
			}
		})
	}
}

func TestNegotiatedFormatQueryOverridesAccept(t *testing.T) {
	r := httptest.NewRequest("GET", "/fhir/r4/Patient?_format=xml", nil)
	r.Header.Set("Accept", "application/fhir+json")
	if got := negotiatedFormat(r); got != "xml" {
		t.Fatalf("got %q, want xml", got)
	}
}
