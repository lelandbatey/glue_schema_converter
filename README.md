
# AWS Glue/Athena Table Schema converter

This tool `glue_schema_converter` converts an [AWS Glue](https://console.aws.amazon.com/glue/home#catalog:tab=tables) table schema
into a JSON schema. This tool exists because the way AWS represents the schemas
for Tables in Glue/Athena seemed unique and there doesn't seem to be anything
else out there for programatically converting them into other schema formats.

# Installation

Install using Go get:
```
go get github.com/lelandbatey/glue_schema_converter
```

It's recommended that you use this in turn with [jq](https://stedolan.github.io/jq/) and the [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html).

# Usage

```
$ glue_schema_converter -h
Usage: glue_schema_converter
glue_schema_converter accepts no options. glue_schema_converter attempts to
read in an AWS Glue table schema on stdin, parse that schema, and translate the
schema into a JSON schema. An example schema to feed this is:

	struct<created_time:bigint,user_id:int,favnumbers:array<int>,description:string>

You can fetch these table schemas from AWS with the AWS CLI and jq like so:

	aws glue get-tables --database {DATABASE_NAME} | jq -r '.TableList | .[] | .StorageDescriptor.Columns | .[] | select(.Name=="fulldocument") | .Type'
```

## Example:

By running the following command:

	echo "struct<created_time:bigint,user_id:int,favnumbers:array<int>,description:string>" | ./glue_schema_converter

The following is output:

	{"type": "object", "properties": {"description": {"type": "string"}, "created_time": {"type": "number"}, "user_id": {"type": "number"}, "favnumbers": {"type": "array", "items": {"type": {"type": "number"}}}}}

Running all that through [jq](https://stedolan.github.io/jq/) gives the following nicely formatted schema:
```
$ echo "struct<created_time:bigint,user_id:int,favnumbers:array<int>,description:string>" | ./glue_schema_converter | jq .
{
  "type": "object",
  "properties": {
    "favnumbers": {
      "type": "array",
      "items": {
        "type": {
          "type": "number"
        }
      }
    },
    "description": {
      "type": "string"
    },
    "created_time": {
      "type": "number"
    },
    "user_id": {
      "type": "number"
    }
  }
}
```
