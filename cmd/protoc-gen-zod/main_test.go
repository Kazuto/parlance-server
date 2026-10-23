package main

import (
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestFieldTypeWireRepresentations(t *testing.T) {
	optional := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	repeated := descriptorpb.FieldDescriptorProto_LABEL_REPEATED
	field := func(name string, number int32, kind descriptorpb.FieldDescriptorProto_Type, typeName string, label descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto {
		return &descriptorpb.FieldDescriptorProto{
			Name: proto.String(name), Number: proto.Int32(number), Type: &kind,
			TypeName: proto.String(typeName), Label: &label,
		}
	}
	file := &descriptorpb.FileDescriptorProto{
		Name: proto.String("fixture.proto"), Package: proto.String("fixture"), Syntax: proto.String("proto3"),
		Options: &descriptorpb.FileOptions{GoPackage: proto.String("example.com/fixture")},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Node"),
			Field: []*descriptorpb.FieldDescriptorProto{
				field("child", 1, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".fixture.Node", optional),
				field("labels", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".fixture.Node.LabelsEntry", repeated),
				field("ids", 3, descriptorpb.FieldDescriptorProto_TYPE_INT64, "", repeated),
				field("data", 4, descriptorpb.FieldDescriptorProto_TYPE_BYTES, "", optional),
				field("state", 5, descriptorpb.FieldDescriptorProto_TYPE_ENUM, ".fixture.State", optional),
				field("ratio", 6, descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, "", optional),
			},
			NestedType: []*descriptorpb.DescriptorProto{{
				Name: proto.String("LabelsEntry"), Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
				Field: []*descriptorpb.FieldDescriptorProto{
					field("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", optional),
					field("value", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING, "", optional),
				},
			}},
		}},
		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name:  proto.String("State"),
			Value: []*descriptorpb.EnumValueDescriptorProto{{Name: proto.String("STATE_UNSPECIFIED"), Number: proto.Int32(0)}},
		}},
	}

	plugin, err := (protogen.Options{}).New(&pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"fixture.proto"}, ProtoFile: []*descriptorpb.FileDescriptorProto{file},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []struct{ schema, typ string }{
		{"NodeSchema", "Node"},
		{"z.record(z.string(), z.string())", "Record<string, string>"},
		{`z.array(z.string().regex(/^-?\d+$/))`, "Array<string>"},
		{"z.string()", "string"},
		{"StateSchema", "State"},
		{`z.union([z.number(), z.enum(["NaN", "Infinity", "-Infinity"])])`, `number | "NaN" | "Infinity" | "-Infinity"`},
	}

	for i, field := range plugin.Files[0].Messages[0].Fields {
		schema, typ, err := fieldType(field)
		if err != nil || schema != want[i].schema || typ != want[i].typ {
			t.Errorf("%s: got (%q, %q, %v), want (%q, %q)", field.Desc.Name(), schema, typ, err, want[i].schema, want[i].typ)
		}
	}

	if got := messages(plugin.Files[0].Messages); len(got) != 1 {
		t.Fatalf("map-entry messages must not be emitted: got %d messages", len(got))
	}
}
