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

package handler_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

const testLastUpdated = "2026-10-25T10:00:00.123Z"
const testLastModified = "Sun, 25 Oct 2026 10:00:00 GMT"

func savedEncounter(id, version string) map[string]any {
	return map[string]any{
		"resourceType": "Encounter", "id": id, "status": "finished",
		"class": map[string]any{"code": "AMB"},
		"meta":  map[string]any{"versionId": version, "lastUpdated": testLastUpdated},
	}
}

func checkWriteHeaders(t *testing.T, h http.Header, location, etag string) {
	t.Helper()
	if got := h.Get("Location"); !strings.HasSuffix(got, location) {
		t.Errorf("Location = %q, want suffix %q", got, location)
	}
	if got := h.Get("ETag"); got != etag {
		t.Errorf("ETag = %q, want %q", got, etag)
	}
	if got := h.Get("Last-Modified"); got != testLastModified {
		t.Errorf("Last-Modified = %q, want %q", got, testLastModified)
	}
}

func TestCreate_PreferReturnMinimal(t *testing.T) {
	ms := &mockStore{}
	ms.createFn = func(_ context.Context, _ string, _ map[string]any) (map[string]any, error) {
		return savedEncounter("e1", "1"), nil
	}
	h := newRouter(ms)

	w := doWithHeaders(t, h, http.MethodPost, "/fhir/r4/Encounter", savedEncounter("", "1"),
		map[string]string{"Prefer": "return=minimal"})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("return=minimal should have no body, got %q", w.Body.String())
	}
	checkWriteHeaders(t, w.Header(), "/Encounter/e1/_history/1", `W/"1"`)
}

func TestUpdate_PreferReturn(t *testing.T) {
	ms := &mockStore{}
	ms.updateOrCreateFn = func(_ context.Context, _, id string, _ map[string]any, _ int) (map[string]any, bool, error) {
		return savedEncounter(id, "2"), false, nil
	}
	h := newRouter(ms)

	cases := []struct {
		prefer string
		want   string // resourceType of the body, "" for none
	}{
		{"", "Encounter"},
		{"return=representation", "Encounter"},
		{"return=minimal", ""},
		{"return=OperationOutcome", "OperationOutcome"},
		{`handling=strict, return="minimal"`, ""},
		{"return=minimal; foo=bar", ""},
		// "return=minimal" here is a parameter of "handling", not a preference.
		{"handling=strict; return=minimal", "Encounter"},
	}
	for _, tc := range cases {
		w := doWithHeaders(t, h, http.MethodPut, "/fhir/r4/Encounter/e1", savedEncounter("e1", "1"),
			map[string]string{"Prefer": tc.prefer})
		if w.Code != http.StatusOK {
			t.Fatalf("Prefer %q: want 200, got %d", tc.prefer, w.Code)
		}
		checkWriteHeaders(t, w.Header(), "/Encounter/e1/_history/2", `W/"2"`)
		if tc.want == "" {
			if w.Body.Len() != 0 {
				t.Errorf("Prefer %q: want no body, got %q", tc.prefer, w.Body.String())
			}
			continue
		}
		if got := decodeJSON(t, w)["resourceType"]; got != tc.want {
			t.Errorf("Prefer %q: body resourceType = %v, want %s", tc.prefer, got, tc.want)
		}
	}
}

func TestPatch_SetsLocationAndLastModified(t *testing.T) {
	ms := &mockStore{}
	ms.patchFn = func(_ context.Context, _, id string, _ map[string]any) (map[string]any, error) {
		return savedEncounter(id, "3"), nil
	}
	h := newRouter(ms)

	w := doWithHeaders(t, h, http.MethodPatch, "/fhir/r4/Encounter/e1", map[string]any{"status": "cancelled"},
		map[string]string{"Content-Type": "application/merge-patch+json"})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	checkWriteHeaders(t, w.Header(), "/Encounter/e1/_history/3", `W/"3"`)
}

func TestRead_SetsLastModified(t *testing.T) {
	ms := &mockStore{}
	ms.readFn = func(_ context.Context, _, id string) (map[string]any, error) {
		return savedEncounter(id, "4"), nil
	}
	h := newRouter(ms)

	w := do(t, h, http.MethodGet, "/fhir/r4/Encounter/e1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if got := w.Header().Get("Last-Modified"); got != testLastModified {
		t.Errorf("Last-Modified = %q, want %q", got, testLastModified)
	}
}
