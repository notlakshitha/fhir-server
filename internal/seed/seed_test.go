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

package seed

import (
	"encoding/csv"
	"slices"
	"strings"
	"testing"

	"github.com/wso2/fhir-server/internal/fhirpath"
)

type csvRow struct {
	line       int
	param      string
	resource   string
	expression string
}

// embeddedRows reads the embedded search-parameter CSV the same way
// seedFromReader does.
func embeddedRows(t *testing.T) []csvRow {
	t.Helper()
	f, err := csvFS.Open("fhir-r4-search-params.csv")
	if err != nil {
		t.Fatalf("open embedded csv: %v", err)
	}
	defer f.Close()

	cr := csv.NewReader(f)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	records, err := cr.ReadAll()
	if err != nil {
		t.Fatalf("read embedded csv: %v", err)
	}
	colIdx := csvColumnIndex(records[0])

	var rows []csvRow
	for i, rec := range records[1:] {
		if len(rec) <= colIdx.maxIdx() || colIdx.expression >= len(rec) {
			continue
		}
		rows = append(rows, csvRow{
			line:       i + 2,
			param:      strings.TrimSpace(rec[colIdx.paramName]),
			resource:   strings.TrimSpace(rec[colIdx.resourceType]),
			expression: strings.TrimSpace(rec[colIdx.expression]),
		})
	}
	return rows
}

// unionPaths splits an expression on top-level '|'.
func unionPaths(expr string) []string {
	var paths []string
	depth, start := 0, 0
	for i, c := range expr {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case '|':
			if depth == 0 {
				paths = append(paths, strings.TrimSpace(expr[start:i]))
				start = i + 1
			}
		}
	}
	return append(paths, strings.TrimSpace(expr[start:]))
}

// The FHIRPath evaluator drops an expression's leading type name, so a path
// copied from another resource is read against the wrong fields and indexes
// nothing (Encounter's identifier row once read
// "DocumentReference.masterIdentifier"). Every path that names a type must
// name the row's own resource.
func TestEmbeddedCSV_PathsStartAtOwnResource(t *testing.T) {
	for _, row := range embeddedRows(t) {
		if row.expression == "" {
			continue
		}
		for _, path := range unionPaths(row.expression) {
			root := strings.TrimLeft(path, "(")
			if i := strings.IndexAny(root, ". ("); i >= 0 {
				root = root[:i]
			}
			if root == "" || root[0] < 'A' || root[0] > 'Z' {
				continue // relative path, e.g. InsurancePlan's "name"
			}
			if root != row.resource {
				t.Errorf("line %d: %s %s: path %q starts at %s", row.line, row.resource, row.param, path, root)
			}
		}
	}
}

// The onset-* and abatement-* rows were cut off at ".as", which matches no
// field, so those Condition searches never matched. Each must select only its
// own onset[x] / abatement[x] types.
func TestEmbeddedCSV_ConditionOnsetAbatement(t *testing.T) {
	exprs := map[string]string{}
	for _, row := range embeddedRows(t) {
		if row.resource == "Condition" {
			exprs[row.param] = row.expression
		}
	}
	values := map[string]any{
		"DateTime": "2023-11-14",
		"Period":   map[string]any{"start": "2020-01-01", "end": "2020-06-30"},
		"Age":      map[string]any{"value": 7, "code": "a"},
		"Range":    map[string]any{"low": map[string]any{"value": 5}},
		"String":   "early adulthood",
	}
	wants := map[string][]string{
		"onset-date":       {"DateTime", "Period"},
		"onset-age":        {"Age", "Range"},
		"onset-info":       {"String"},
		"abatement-date":   {"DateTime", "Period"},
		"abatement-age":    {"Age", "Range"},
		"abatement-string": {"String"},
	}
	for param, types := range wants {
		element, _, _ := strings.Cut(param, "-")
		for suffix, v := range values {
			cond := map[string]any{"resourceType": "Condition", element + suffix: v}
			got, err := fhirpath.EvaluatePolymorphic(exprs[param], cond)
			if err != nil {
				t.Fatalf("%s: evaluate %q: %v", param, exprs[param], err)
			}
			want := 0
			if slices.Contains(types, suffix) {
				want = 1
			}
			if len(got) != want {
				t.Errorf("%s on %s%s: got %d values, want %d (expression %q)", param, element, suffix, len(got), want, exprs[param])
			}
		}
	}
}
