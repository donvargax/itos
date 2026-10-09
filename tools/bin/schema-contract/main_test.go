package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func schemaFixture(t *testing.T, text string) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var schema map[string]any
	if err := decoder.Decode(&schema); err != nil {
		t.Fatalf("decode schema fixture: %v", err)
	}
	return schema
}

func assertFindings(t *testing.T, got, want []finding) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("findings = %#v, want %#v", got, want)
	}
}

func TestCompareSame(t *testing.T) {
	tests := []struct {
		name, old, new, path string
		want                 []finding
	}{
		{
			name: "enum value removed and added",
			old:  `{"type":"string","enum":["red","blue"]}`,
			new:  `{"type":"string","enum":["red","green"]}`,
			path: "color",
			want: []finding{
				{Path: "color", Change: `enum value "blue" removed`, Breaking: true},
				{Path: "color", Change: `enum value "green" added`, Breaking: false},
			},
		},
		{
			name: "enum introduced",
			old:  `{"type":"string"}`,
			new:  `{"type":"string","enum":["red"]}`,
			path: "color",
			want: []finding{{Path: "color", Change: `values narrowed: only ["red"] is now accepted`, Breaking: true}},
		},
		{
			name: "enum removed",
			old:  `{"type":"string","enum":["red"]}`,
			new:  `{"type":"string"}`,
			path: "color",
			want: []finding{{Path: "color", Change: `values widened: any string is now accepted, not only ["red"]`, Breaking: false}},
		},
		{
			name: "array item recursion",
			old:  `{"type":"array","items":{"type":"string","enum":["a","b"]}}`,
			new:  `{"type":"array","items":{"type":"string","enum":["a"]}}`,
			path: "items",
			want: []finding{{Path: "items[]", Change: `enum value "b" removed`, Breaking: true}},
		},
		{
			name: "closed object adds and removes keys",
			old:  `{"type":"object","properties":{"old":{"type":"string"}},"additionalProperties":false}`,
			new:  `{"type":"object","properties":{"new":{"type":"string"}},"additionalProperties":false}`,
			path: "color",
			want: []finding{
				{Path: "color.new", Change: "key added", Breaking: false},
				{Path: "color.old", Change: "key removed", Breaking: true},
			},
		},
		{
			name: "open object closes",
			old:  `{"type":"object"}`,
			new:  `{"type":"object","additionalProperties":false}`,
			path: "color",
			want: []finding{{Path: "color", Change: "object closed: a key it does not list is now refused", Breaking: true}},
		},
		{
			name: "closed object opens",
			old:  `{"type":"object","additionalProperties":false}`,
			new:  `{"type":"object"}`,
			path: "color",
			want: []finding{{Path: "color", Change: "object opened: a key it does not list is now accepted", Breaking: false}},
		},
		{
			name: "schema-valued additional properties change",
			old:  `{"type":"object","additionalProperties":{"type":"string"}}`,
			new:  `{"type":"object","additionalProperties":{"type":"number"}}`,
			path: "color",
			want: []finding{
				{Path: "color.<key>", Change: "type narrowed: a string is no longer accepted (now a number)", Breaking: true},
				{Path: "color.<key>", Change: "type widened: a number is now accepted", Breaking: false},
			},
		},
		{
			name: "required key added and removed",
			old:  `{"type":"object","required":["before","stable"]}`,
			new:  `{"type":"object","required":["after","stable"]}`,
			path: "color",
			want: []finding{
				{Path: "color.after", Change: "key newly required", Breaking: true},
				{Path: "color.before", Change: "key no longer required", Breaking: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []finding
			compareSame(schemaFixture(t, tt.old), schemaFixture(t, tt.new), typeOf(schemaFixture(t, tt.old)), tt.path, &got)
			assertFindings(t, got, tt.want)
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name, old, new, path string
		want                 []finding
	}{
		{
			name: "default added",
			old:  `{"type":"string"}`,
			new:  `{"type":"string","default":"ready"}`,
			path: "value",
			want: []finding{{Path: "value", Change: `default added ("ready"), where there was none`, Breaking: true}},
		},
		{
			name: "default removed",
			old:  `{"type":"string","default":"ready"}`,
			new:  `{"type":"string"}`,
			path: "value",
			want: []finding{{Path: "value", Change: `default removed (was "ready")`, Breaking: true}},
		},
		{
			name: "default changed",
			old:  `{"type":"number","default":1}`,
			new:  `{"type":"number","default":2}`,
			path: "value",
			want: []finding{{Path: "value", Change: "default changed from 1 to 2", Breaking: true}},
		},
		{
			name: "type widened and narrowed",
			old:  `{"type":"string"}`,
			new:  `{"type":"number"}`,
			path: "value",
			want: []finding{
				{Path: "value", Change: "type narrowed: a string is no longer accepted (now a number)", Breaking: true},
				{Path: "value", Change: "type widened: a number is now accepted", Breaking: false},
			},
		},
		{
			name: "unconstrained schema widens",
			old:  `{"type":"string"}`,
			new:  `{}`,
			path: "value",
			want: []finding{{Path: "value", Change: "type widened: any value is now accepted (was a string)", Breaking: false}},
		},
		{
			name: "unconstrained schema narrows",
			old:  `{}`,
			new:  `{"type":"string"}`,
			path: "value",
			want: []finding{{Path: "value", Change: "type narrowed: any value was accepted, now only a string", Breaking: true}},
		},
		{
			name: "anyOf widening",
			old:  `{"anyOf":[{"type":"string"}]}`,
			new:  `{"anyOf":[{"type":"string"},{"type":"number"}]}`,
			path: "value",
			want: []finding{{Path: "value", Change: "type widened: a number is now accepted", Breaking: false}},
		},
		{
			name: "anyOf narrowing",
			old:  `{"anyOf":[{"type":"string"},{"type":"number"}]}`,
			new:  `{"anyOf":[{"type":"string"}]}`,
			path: "value",
			want: []finding{{Path: "value", Change: "type narrowed: a number is no longer accepted (now a string)", Breaking: true}},
		},
		{
			name: "unchanged schema",
			old:  `{"type":"string","enum":["ready"]}`,
			new:  `{"type":"string","enum":["ready"]}`,
			path: "value",
		},
		{
			name: "nested object property path",
			old:  `{"type":"object","properties":{"child":{"type":"object","properties":{"name":{"type":"string"}}}}}`,
			new:  `{"type":"object","properties":{"child":{"type":"object","properties":{"name":{"type":"number"}}}}}`,
			path: "",
			want: []finding{
				{Path: "child.name", Change: "type narrowed: a string is no longer accepted (now a number)", Breaking: true},
				{Path: "child.name", Change: "type widened: a number is now accepted", Breaking: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []finding
			compare(schemaFixture(t, tt.old), schemaFixture(t, tt.new), tt.path, &got)
			assertFindings(t, got, tt.want)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name, schema, wantError string
	}{
		{
			name:   "all supported nested shapes",
			schema: `{"$schema":"draft","title":"root","description":"schema","type":"object","enum":["x"],"required":["items"],"default":{},"properties":{"items":{"type":"array","items":{"type":"string"}},"open":{"additionalProperties":{"type":"number"}},"closed":{"additionalProperties":false},"choice":{"anyOf":[{"type":"string"},{"type":"number"}]}}}`,
		},
		{name: "unknown nested keyword path", schema: `{"type":"object","properties":{"child":{"wat":true}}}`, wantError: `child has "wat", which the comparison does not read`},
		{name: "wrong type shape", schema: `{"properties":{"child":{"type":["string"]}}}`, wantError: `child has a type that is not a string`},
		{name: "wrong properties shape", schema: `{"properties":[]}`, wantError: ` has properties that are not an object`},
		{name: "property is not a schema", schema: `{"properties":{"child":true}}`, wantError: `child is not a schema`},
		{name: "array item is not a schema", schema: `{"properties":{"rows":{"type":"array","items":[]}}}`, wantError: `rows has items that is not a schema`},
		{name: "additional properties is neither schema nor boolean", schema: `{"additionalProperties":[]}`, wantError: ` has additionalProperties that is not a schema`},
		{name: "anyOf is not a list", schema: `{"properties":{"choice":{"anyOf":{}}}}`, wantError: `choice has an anyOf that is not a list`},
		{name: "anyOf entry is not a schema", schema: `{"anyOf":["string"]}`, wantError: ` has an anyOf entry that is not a schema`},
		{name: "nested additional properties keyword path", schema: `{"properties":{"settings":{"additionalProperties":{"unsupported":true}}}}`, wantError: `settings.<key> has "unsupported"`},
		{name: "enum and required must be lists but members are not validated", schema: `{"enum":[1,{}],"required":[false,4]}`},
		{name: "enum is not a list", schema: `{"properties":{"child":{"enum":"red"}}}`, wantError: `child has enum that is not a list`},
		{name: "required is not a list", schema: `{"properties":{"child":{"required":"name"}}}`, wantError: `child has required that is not a list`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(schemaFixture(t, tt.schema), "")
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("validate() error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}
