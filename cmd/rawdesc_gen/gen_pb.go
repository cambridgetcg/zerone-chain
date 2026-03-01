//go:build ignore

package main

import (
	"fmt"
	"os"
	"strings"

	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

func main() {
	which := os.Args[1]
	switch which {
	case "types":
		genTypesPB()
	case "genesis":
		genGenesisPB()
	case "tx":
		genTxPB()
	case "query":
		genQueryPB()
	}
}

func readRawDesc(file string) ([]byte, *descriptorpb.FileDescriptorProto) {
	bz, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}
	// Parse the raw desc string from the file
	// Actually, we re-generate it
	return bz, nil
}

func goFieldType(f *descriptorpb.FieldDescriptorProto) string {
	switch f.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		if f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
			return "[]string"
		}
		return "string"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64:
		return "uint64"
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32:
		return "uint32"
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return "bool"
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		tn := f.GetTypeName()
		parts := strings.Split(tn, ".")
		name := parts[len(parts)-1]
		if f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
			return "[]*" + name
		}
		return "*" + name
	}
	return "interface{}"
}

func goFieldName(name string) string {
	parts := strings.Split(name, "_")
	result := ""
	for _, p := range parts {
		if p == "id" {
			result += "Id"
		} else if p == "bps" {
			result += "Bps"
		} else if p == "op" {
			result += "Op"
		} else if p == "bvm" {
			result += "BVM"
		} else {
			result += strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return result
}

func protoTag(f *descriptorpb.FieldDescriptorProto) string {
	wireType := "bytes"
	switch f.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64, descriptorpb.FieldDescriptorProto_TYPE_UINT32, descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		wireType = "varint"
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		wireType = "bytes"
	}
	
	rep := "opt"
	if f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
		rep = "rep"
	}
	
	jsonName := f.GetJsonName()
	return fmt.Sprintf("`protobuf:\"%s,%d,%s,name=%s,json=%s,proto3\" json:\"%s,omitempty\"`",
		wireType, f.GetNumber(), rep, f.GetName(), jsonName, f.GetName())
}

func goDefaultValue(f *descriptorpb.FieldDescriptorProto) string {
	switch f.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		if f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
			return "nil"
		}
		return `""`
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64, descriptorpb.FieldDescriptorProto_TYPE_UINT32:
		return "0"
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return "false"
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		return "nil"
	}
	return "nil"
}

type fileInfo struct {
	protoFile string
	varPrefix string
	messages  []*descriptorpb.DescriptorProto
	numMsgs   int
	depIdxs   []string // dep index entries
	goTypes   []string // goType entries
	depTypes  []string // external dep types
}

func genTypesPB() {
	// The complete types.pb.go is quite long and complex.
	// Instead of generating the full thing, let's just output the NEW struct definitions
	// that need to be added, plus the updated rawDesc/goTypes/etc.
	fmt.Println("// This file would contain the full types.pb.go")
	fmt.Println("// For now, outputting the Mentorship and FormationMatch struct patterns")
	
	_ = proto.Marshal
}

func genGenesisPB() { fmt.Println("genesis") }
func genTxPB()      { fmt.Println("tx") }
func genQueryPB()   { fmt.Println("query") }
