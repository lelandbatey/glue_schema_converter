package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"unicode"
)

type RuneReader struct {
	Contents   []rune
	ContentLen int
	RunePos    int
	LineNo     int
}

func (self *RuneReader) ReadRune() (rune, error) {
	var toret rune = 0
	var err error

	if self.RunePos < self.ContentLen {
		toret = self.Contents[self.RunePos]
		if toret == '\n' {
			self.LineNo += 1
		}
		self.RunePos += 1
	} else {
		err = io.EOF
	}
	return toret, err
}

func (self *RuneReader) UnreadRune() error {
	if self.RunePos == 0 {
		return bufio.ErrInvalidUnreadRune
	}
	self.RunePos -= 1
	switch self.Contents[self.RunePos] {
	case '\n':
		self.LineNo -= 1
	}
	return nil
}

func NewRuneReader(r io.Reader) *RuneReader {
	b, _ := ioutil.ReadAll(r)
	contents := bytes.Runes(b)
	return &RuneReader{
		Contents:   contents,
		ContentLen: len(contents),
		RunePos:    0,
		LineNo:     1,
	}
}

func isIdent(r rune) bool {
	switch {
	case unicode.IsLetter(r):
		return true
	case unicode.IsDigit(r):
		return true
	case r == '_':
		return true
	default:
		return false
	}
}

type ScanUnit struct {
	BraceLevel int
	LineNo     int
	Value      []rune
}

func (self ScanUnit) String() string {
	cleanval := strings.Replace(string(self.Value), "\n", "\\n", -1)
	cleanval = strings.Replace(cleanval, "\t", "\\t", -1)
	cleanval = strings.Replace(cleanval, "\"", "\\\"", -1)
	return fmt.Sprintf(`{"value": "%v", "BraceLevel": %v, "LineNo": %v},`, cleanval, self.BraceLevel, self.LineNo)
}

var (
	braceLev int = 0
)

func BuildScanUnit(rr *RuneReader) (*ScanUnit, error) {
	rv := &ScanUnit{
		0,
		1,
		[]rune{},
	}
	var ch rune
	buf := make([]rune, 0)
	setReturn := func() *ScanUnit {
		rv.BraceLevel = braceLev
		rv.LineNo = rr.LineNo
		rv.Value = buf
		return rv
	}

	// Populate the buffer with at least one rune so even if it's an unknown
	// character it will at least return this
	ch, err := rr.ReadRune()
	if err != nil {
		return setReturn(), err
	}
	buf = append(buf, ch)

	switch {
	case unicode.IsSpace(ch):
		// Group consecutive white space characters
		for {
			ch, err = rr.ReadRune()
			if err != nil {
				// Don't pass along this EOF since we did find a valid 'Unit'
				// to return. This way, the next call of this function will
				// return EOF and nothing else, a more clear behavior.
				if err == io.EOF {
					return setReturn(), nil
				}
				return setReturn(), err
			} else if !unicode.IsSpace(ch) {
				rr.UnreadRune()
				break
			}
			buf = append(buf, ch)
		}
	case isIdent(ch):
		// Group consecutive letters
		for {
			ch, err = rr.ReadRune()
			if err != nil {
				if err == io.EOF {
					return setReturn(), nil
				}
				return setReturn(), err
			} else if !isIdent(ch) {
				rr.UnreadRune()
				break
			}
			buf = append(buf, ch)
		}
	case ch == '<':
		braceLev += 1
	case ch == '>':
		braceLev -= 1
	}

	// Implicitly, everything that's not a group of letters or not a group of
	// whitespace will be returned one rune at a time.

	return setReturn(), nil
}

type SvcScanner struct {
	R          *RuneReader
	BraceLevel int
	Buf        []*ScanUnit
	UnitPos    int
	lineNo     int
}

func NewSvcScanner(r io.Reader) *SvcScanner {
	b := make([]*ScanUnit, 0)
	rr := NewRuneReader(r)
	for {
		unit, err := BuildScanUnit(rr)
		if err == nil {
			b = append(b, unit)
		} else {
			break
		}
	}
	return &SvcScanner{
		R:          NewRuneReader(r),
		BraceLevel: 0,
		Buf:        b,
		UnitPos:    0,
	}
}

// ReadUnit returns the next "group" of runes found in the input stream. If the
// end of the stream is reached, io.EOF will be returned as error. No other
// errors will be returned.
func (self *SvcScanner) ReadUnit() ([]rune, error) {
	var rv []rune
	var err error
	if self.UnitPos < len(self.Buf) {
		unit := self.Buf[self.UnitPos]

		self.BraceLevel = unit.BraceLevel
		self.lineNo = unit.LineNo

		rv = unit.Value

		self.UnitPos += 1
	} else {
		err = io.EOF
	}

	return rv, err
}

func (self *SvcScanner) UnreadUnit() error {
	if self.UnitPos == 0 {
		return fmt.Errorf("Cannot unread when scanner is at start of input")
	}
	// If we're on the first unit, Unreading means setting the state of the
	// scanner back to it's defaults.
	if self.UnitPos == 1 {
		self.UnitPos = 0
		self.BraceLevel = 0
		self.lineNo = 0
	}
	self.UnitPos -= 1

	// Since the state of the scanner usually tracks one behind the `unit`
	// indicated by `UnitPos` we further subtract one when selecting the unit
	// to reflect the state of
	unit := self.Buf[self.UnitPos-1]
	self.BraceLevel = unit.BraceLevel
	self.lineNo = unit.LineNo

	return nil
}
func (self *SvcScanner) UnReadToPosition(position int) error {
	for {
		if self.UnitPos != position {
			err := self.UnreadUnit()
			if err != nil {
				return err
			}
		} else {
			break
		}
	}
	return nil
}

func (self *SvcScanner) GetLineNumber() int {
	return self.lineNo
}

func peak(scn *SvcScanner) (string, error) {
	unit, err := scn.ReadUnit()
	if err != nil {
		return "", err
	}
	str := string(unit)
	// Coalesce whitespace
	if unicode.IsSpace(unit[0]) {
		if strings.Contains(str, "\n") {
			str = "\n"
		} else {
			str = " "
		}
	}
	err = scn.UnreadUnit()
	if err != nil {
		return "", err
	}
	return str, nil
}

///////////////////////////////////////////////////////////////////

type SchemaType struct {
	Typ        string
	Fields     map[string]*SchemaType
	FieldOrder []string
}

func (st *SchemaType) AddField(name string, s *SchemaType) {
	st.Fields[name] = s
	st.FieldOrder = append(st.FieldOrder, name)
}

func (st *SchemaType) String() string {
	if st.Fields == nil || len(st.Fields) == 0 {
		return st.Typ
	}
	if st.Typ == "array" {
		return fmt.Sprintf("array<%s>", st.Fields[""].String())
	}
	fields := []string{}
	for _, fn := range st.FieldOrder {
		ft := st.Fields[fn]
		fieldstr := fmt.Sprintf("%s:%s", fn, ft.String())
		fields = append(fields, fieldstr)
	}
	return fmt.Sprintf("struct<%s>", strings.Join(fields, ","))
}

func (st *SchemaType) Json() string {
	TableTypeToJSONType := map[string]string{
		"string":  "string",
		"int":     "number",
		"bigint":  "number",
		"double":  "number",
		"struct":  "object",
		"array":   "array",
		"boolean": "boolean",
	}
	if st.Fields == nil || len(st.Fields) == 0 {
		return fmt.Sprintf(`{"type": "%s"}`, TableTypeToJSONType[st.Typ])
	}
	if st.Typ == "array" {
		return fmt.Sprintf(`{"type": "array", "items": {"type": %s}}`, st.Fields[""].Json())
	}
	if st.Typ == "struct" {
		x := `{"type": "object", "properties": {%s}}`
		properties := []string{}
		for fn, field := range st.Fields {
			properties = append(properties, fmt.Sprintf(`"%s": %s`, fn, field.Json()))
		}
		return fmt.Sprintf(x, fmt.Sprintf(strings.Join(properties, ", ")))
	}
	return "ERROR"
}

var ValidTypes map[string]int = map[string]int{
	"int":     1,
	"bigint":  1,
	"struct":  1,
	"string":  1,
	"double":  1,
	"array":   1,
	"boolean": 1,
}

func checkPanic(err error) error {
	if err == io.EOF {
		return err
	}
	if err != nil {
		panic(err)
	}
	return nil
}

func VerifyExpecting(scn *SvcScanner, expected string) error {
	return VerifyExpectingStr(scn, expected, "")
}

func VerifyExpectingStr(scn *SvcScanner, expected, context string) error {
	rs, err := scn.ReadUnit()
	if checkPanic(err) != nil {
		return err
	}
	if string(rs) != expected {
		return fmt.Errorf("expected value of %q but recieved %q at position %d, line %d%s", expected, string(rs), scn.UnitPos-1, scn.GetLineNumber(), context)
	}
	return nil
}

func parseStruct(scn *SvcScanner) (*SchemaType, error) {
	outStruct := SchemaType{
		Typ:        "struct",
		Fields:     map[string]*SchemaType{},
		FieldOrder: []string{},
	}
	err := VerifyExpecting(scn, "struct")
	if err != nil {
		return nil, err
	}
	err = VerifyExpecting(scn, "<")
	if err != nil {
		return nil, err
	}

	for {
		unit, err := scn.ReadUnit()
		if checkPanic(err) != nil {
			return nil, err
		}
		fieldname := string(unit)
		err = VerifyExpecting(scn, ":")
		if err != nil {
			return nil, err
		}
		str, err := peak(scn)
		if checkPanic(err) != nil {
			return nil, err
		}
		if str == "array" {
			ftyp, err := parseArray(scn)
			if err != nil {
				return nil, err
			}
			outStruct.AddField(fieldname, ftyp)
		} else if str == "struct" {
			ftyp, err := parseStruct(scn)
			if err != nil {
				return nil, err
			}
			outStruct.AddField(fieldname, ftyp)
		} else if str == "struct" {
		} else {
			unit, err := scn.ReadUnit()
			if checkPanic(err) != nil {
				return nil, err
			}
			str = string(unit)
			if _, ok := ValidTypes[str]; !ok {
				return nil, fmt.Errorf("type %q not in list of approved types %v", str, ValidTypes)
			}
			outStruct.AddField(fieldname, &SchemaType{Typ: str})
		}
		str, err = peak(scn)
		if checkPanic(err) != nil {
			return nil, err
		}
		if str == ">" {
			err = VerifyExpecting(scn, ">")
			if err != nil {
				return nil, err
			}
			return &outStruct, nil
		}
		err = VerifyExpectingStr(scn, ",", fmt.Sprintf(", when parsing struct after parsing field %s", fieldname))
		if err != nil {
			return nil, err
		}
	}

}

func parseArray(scn *SvcScanner) (*SchemaType, error) {
	outStruct := SchemaType{
		Typ:        "array",
		Fields:     map[string]*SchemaType{},
		FieldOrder: []string{},
	}
	err := VerifyExpecting(scn, "array")
	if err != nil {
		return nil, err
	}
	err = VerifyExpecting(scn, "<")
	if err != nil {
		return nil, err
	}
	str, err := peak(scn)
	if checkPanic(err) != nil {
		return nil, err
	}
	if str == "array" {
		ftyp, err := parseArray(scn)
		if err != nil {
			return nil, err
		}
		outStruct.AddField("", ftyp)
	} else if str == "struct" {
		ftyp, err := parseStruct(scn)
		if err != nil {
			return nil, err
		}
		outStruct.AddField("", ftyp)
	} else {
		outStruct.AddField("", &SchemaType{Typ: str})
		_, err := scn.ReadUnit()
		if checkPanic(err) != nil {
			return nil, err
		}
	}
	err = VerifyExpecting(scn, ">")
	if err != nil {
		return nil, err
	}
	return &outStruct, nil
}

func fmtUnitBuf(scn *SvcScanner) string {
	uns := ""
	us := ""
	for i, unit := range scn.Buf {
		s := string(unit.Value)
		width := len(s)
		numlen := len(fmt.Sprintf("%d", i))
		if numlen > width {
			width = numlen
		}
		wstr := fmt.Sprintf("%d", width)
		us = fmt.Sprintf("%s %"+wstr+"s", us, s)
		uns = fmt.Sprintf("%s %-"+wstr+"d", uns, i)
	}
	return fmt.Sprintf("%s\n%s", uns, us)
}

func main() {
	if len(os.Args) > 1 {
		fmt.Fprintf(os.Stderr, `Usage: %s
%s accepts no options. %s attempts to
read in an AWS Glue table schema on stdin, parse that schema, and translate the
schema into a JSON schema. An example schema to feed this is:

	struct<created_time:bigint,user_id:int,favnumbers:array<int>,description:string>

You can fetch these table schemas from AWS with the AWS CLI and jq like so:

	aws glue get-tables --database {DATABASE_NAME} | jq -r '.TableList | .[] | .StorageDescriptor.Columns | .[] | select(.Name=="fulldocument") | .Type'

Alternatively, you can simulateously fetch, parse, and convert all the Table
schemas into JSON schemas with the following command:

	aws glue get-tables --database {DATABASE_NAME} | jq -r '.TableList | .[] | "struct<\(.Name):\(.StorageDescriptor.Columns | .[] | select(.Name=="fulldocument") | .Type )>"' | while read line; do echo "$line" | ./glue_schema_converter | jq -C . ; done

`,
			os.Args[0], os.Args[0], os.Args[0])
		return
	}
	var scn *SvcScanner
	{
		scn = NewSvcScanner(os.Stdin)
	}
	val, err := parseStruct(scn)
	if err != nil {
		fmt.Printf("Recieved error while attempting to parse struct:\n%v\n", err)
		fmt.Printf("\n%s\n", fmtUnitBuf(scn))
		return
	}

	fmt.Printf("%s\n", val.Json())
}
