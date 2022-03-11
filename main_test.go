package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestParseStructSchemaType(t *testing.T) {
	var _ = json.Marshal

	type tcase struct {
		InSchema   string
		ExpectST   *SchemaType
		ExpectJSON string
	}
	for idx, c := range []tcase{
		{
			InSchema: "struct<created_time:bigint>",
			ExpectST: &SchemaType{
				Typ: "struct",
				Fields: map[string]*SchemaType{
					"created_time": {
						Typ: "bigint",
					},
				},
				FieldOrder: []string{"created_time"},
			},
			ExpectJSON: `{"type": "object", "properties": {"created_time": {"type": "number"}}}`,
		},
		{
			InSchema: "struct<likes:array<struct<name:string,place:string,favorite:boolean>>>",
			ExpectST: &SchemaType{
				Typ: "struct",
				Fields: map[string]*SchemaType{
					"likes": {
						Typ: "array",
						Fields: map[string]*SchemaType{
							"": {
								Typ: "struct",
								Fields: map[string]*SchemaType{
									"name":     {Typ: "string"},
									"place":    {Typ: "string"},
									"favorite": {Typ: "boolean"},
								},
								FieldOrder: []string{"name", "place", "favorite"},
							},
						},
						FieldOrder: []string{""},
					},
				},
				FieldOrder: []string{"likes"},
			},
			ExpectJSON: `{"type": "object", "properties": {"likes": {"type": "array", "items": {"type": "object", "properties": {"name": {"type": "string"}, "place": {"type": "string"}, "favorite": {"type": "boolean"}}}}}}`,
		},
		{
			InSchema: "struct<birthdays:array<int>>",
			ExpectST: &SchemaType{
				Typ: "struct",
				Fields: map[string]*SchemaType{
					"birthdays": {
						Typ: "array",
						Fields: map[string]*SchemaType{
							"": {
								Typ: "int",
							},
						},
						FieldOrder: []string{""},
					},
				},
				FieldOrder: []string{"birthdays"},
			},
			ExpectJSON: `{"type": "object", "properties": {"birthdays": {"type": "array", "items": {"type": "number"}}}}`,
		},
	} {
		var scn *SvcScanner
		scn = NewSvcScanner(bytes.NewBuffer([]byte(c.InSchema)))
		got, err := ParseStruct(scn)
		if err != nil {
			t.Fatalf("err != nil; idx = %d, err = %v\n", idx, err)
		}
		if !reflect.DeepEqual(got, c.ExpectST) {
			t.Fatalf("Struct got != want test idx=%d\ngot = %#v\nwant = %#v\n", idx, got, c.ExpectST)
		}
		gj := got.Json()
		wj := c.ExpectJSON
		if gj != wj {
			t.Fatalf("JSON got != want test idx=%d\ngot  JSON = %v\nwant JSON = %v\ngot bytes  = %v\nwant bytes = %v\n", idx, gj, wj, []byte(gj), []byte(wj))
		}
	}
}
