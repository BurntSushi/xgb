package main

import (
	"encoding/xml"
	"io/ioutil"
	"log"
)

type XML struct {
	// Root 'xcb' element properties.
	XMLName        xml.Name `xml:"xcb"`
	Header         string   `xml:"header,attr"`
	ExtensionXName string   `xml:"extension-xname,attr"`
	ExtensionName  string   `xml:"extension-name,attr"`
	MajorVersion   string   `xml:"major-version,attr"`
	MinorVersion   string   `xml:"minor-version,attr"`

	// Types for all top-level elements.
	// First are the simple ones.
	Imports     XMLImports     `xml:"import"`
	Enums       XMLEnums       `xml:"enum"`
	Xids        XMLXids        `xml:"xidtype"`
	XidUnions   XMLXids        `xml:"xidunion"`
	TypeDefs    XMLTypeDefs    `xml:"typedef"`
	EventCopies XMLEventCopies `xml:"eventcopy"`
	ErrorCopies XMLErrorCopies `xml:"errorcopy"`

	// Here are the complex ones, i.e., anything with "structure contents"
	Structs  XMLStructs  `xml:"struct"`
	Unions   XMLUnions   `xml:"union"`
	Requests XMLRequests `xml:"request"`
	Events   XMLEvents   `xml:"event"`
	Errors   XMLErrors   `xml:"error"`
}

type XMLImports []*XMLImport

func (imports XMLImports) Eval() {
	for _, imp := range imports {
		xmlBytes, err := ioutil.ReadFile(*protoPath + "/" + imp.Name + ".xml")
		if err != nil {
			log.Fatalf("Could not read X protocol description for import "+
				"'%s' because: %s", imp.Name, err)
		}

		imp.xml = &XML{}
		err = xml.Unmarshal(xmlBytes, imp.xml)
		if err != nil {
			log.Fatal("Could not parse X protocol description for import "+
				"'%s' because: %s", imp.Name, err)
		}

		// recursive imports...
		imp.xml.Imports.Eval()
	}
}

type XMLImport struct {
	Name string `xml:",chardata"`
	xml  *XML   `xml:"-"`
}

type XMLEnums []XMLEnum

type XMLEnum struct {
	Name  string         `xml:"name,attr"`
	Items []*XMLEnumItem `xml:"item"`
}

type XMLEnumItem struct {
	Name string         `xml:"name,attr"`
	Expr *XMLExpression `xml:",any"`
}

type XMLXids []*XMLXid

type XMLXid struct {
	XMLName xml.Name
	Name    string `xml:"name,attr"`
}

type XMLTypeDefs []*XMLTypeDef

type XMLTypeDef struct {
	Old string `xml:"oldname,attr"`
	New string `xml:"newname,attr"`
}

type XMLEventCopies []*XMLEventCopy

type XMLEventCopy struct {
	Name   string `xml:"name,attr"`
	Number int    `xml:"number,attr"`
	Ref    string `xml:"ref,attr"`
}

type XMLErrorCopies []*XMLErrorCopy

type XMLErrorCopy struct {
	Name   string `xml:"name,attr"`
	Number int    `xml:"number,attr"`
	Ref    string `xml:"ref,attr"`
}

type XMLStructs []*XMLStruct

type XMLStruct struct {
	Name   string    `xml:"name,attr"`
	Fields XMLFields `xml:",any"`
}

type XMLUnions []*XMLUnion

type XMLUnion struct {
	Name   string    `xml:"name,attr"`
	Fields XMLFields `xml:",any"`
}

type XMLRequests []*XMLRequest

type XMLRequest struct {
	Name    string    `xml:"name,attr"`
	Opcode  int       `xml:"opcode,attr"`
	Combine bool      `xml:"combine-adjacent,attr"`
	Fields  XMLFields `xml:",any"`
	Reply   *XMLReply `xml:"reply"`
}

type XMLReply struct {
	Fields XMLFields `xml:",any"`
}

type XMLEvents []*XMLEvent

type XMLEvent struct {
	Name       string    `xml:"name,attr"`
	Number     int       `xml:"number,attr"`
	NoSequence bool      `xml:"no-sequence-number,true"`
	Fields     XMLFields `xml:",any"`
}

type XMLErrors []*XMLError

type XMLError struct {
	Name   string    `xml:"name,attr"`
	Number int       `xml:"number,attr"`
	Fields XMLFields `xml:",any"`
}
