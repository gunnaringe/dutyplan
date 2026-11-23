package api

import (
	_ "embed"
	"net/http"

	"connectrpc.com/grpcreflect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/gunnaringe/dutyplan/gen/dutyplan/v1/dutyplanv1connect"
)

// schema.binpb is the API's schema as buf builds it (mise run gen): the same descriptors the generated code holds, but
// with the comments, which protoc-gen-go leaves out. Reflection answers from it, so a tool sees what each call is for.
//
//go:embed schema.binpb
var schemaBin []byte

// schema is schema.binpb read once; it panics at start if the file does not fit (gen:check keeps it in step).
var schema = func() *protoregistry.Files {
	var set descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(schemaBin, &set); err != nil {
		panic("schema.binpb: " + err.Error())
	}
	files, err := protodesc.NewFiles(&set)
	if err != nil {
		panic("schema.binpb: " + err.Error())
	}
	return files
}()

// reflection is gRPC's server reflection (v1 and the older v1alpha that some tools still ask for), for the five services.
func reflection() []func() (string, http.Handler) {
	names := []string{
		dutyplanv1connect.AccountServiceName, dutyplanv1connect.OrgServiceName, dutyplanv1connect.SetupServiceName,
		dutyplanv1connect.PlanServiceName, dutyplanv1connect.DraftServiceName,
	}
	r := grpcreflect.NewReflector(grpcreflect.NamerFunc(func() []string { return names }),
		grpcreflect.WithDescriptorResolver(schema))
	return []func() (string, http.Handler){
		func() (string, http.Handler) { return grpcreflect.NewHandlerV1(r) },
		func() (string, http.Handler) { return grpcreflect.NewHandlerV1Alpha(r) },
	}
}
