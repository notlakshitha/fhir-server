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
	"strings"
	"testing"
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
