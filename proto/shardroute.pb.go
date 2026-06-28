

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type CheckRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Key           string                 `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Cost          int32                  `protobuf:"varint,2,opt,name=cost,proto3" json:"cost,omitempty"`
	LimitName     string                 `protobuf:"bytes,3,opt,name=limit_name,json=limitName,proto3" json:"limit_name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CheckRequest) Reset() {
	*x = CheckRequest{}
	mi := &file_proto_shardroute_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CheckRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CheckRequest) ProtoMessage() {}

func (x *CheckRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_shardroute_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CheckRequest.ProtoReflect.Descriptor instead.
func (*CheckRequest) Descriptor() ([]byte, []int) {
	return file_proto_shardroute_proto_rawDescGZIP(), []int{0}
}

func (x *CheckRequest) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *CheckRequest) GetCost() int32 {
	if x != nil {
		return x.Cost
	}
	return 0
}

func (x *CheckRequest) GetLimitName() string {
	if x != nil {
		return x.LimitName
	}
	return ""
}

type CheckResponse struct {
	state           protoimpl.MessageState `protogen:"open.v1"`
	Allowed         bool                   `protobuf:"varint,1,opt,name=allowed,proto3" json:"allowed,omitempty"`
	TokensRemaining float64                `protobuf:"fixed64,2,opt,name=tokens_remaining,json=tokensRemaining,proto3" json:"tokens_remaining,omitempty"`
	RetryAfterMs    int64                  `protobuf:"varint,3,opt,name=retry_after_ms,json=retryAfterMs,proto3" json:"retry_after_ms,omitempty"`
	Error           string                 `protobuf:"bytes,4,opt,name=error,proto3" json:"error,omitempty"`
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}

func (x *CheckResponse) Reset() {
	*x = CheckResponse{}
	mi := &file_proto_shardroute_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CheckResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CheckResponse) ProtoMessage() {}

func (x *CheckResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_shardroute_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CheckResponse.ProtoReflect.Descriptor instead.
func (*CheckResponse) Descriptor() ([]byte, []int) {
	return file_proto_shardroute_proto_rawDescGZIP(), []int{1}
}

func (x *CheckResponse) GetAllowed() bool {
	if x != nil {
		return x.Allowed
	}
	return false
}

func (x *CheckResponse) GetTokensRemaining() float64 {
	if x != nil {
		return x.TokensRemaining
	}
	return 0
}

func (x *CheckResponse) GetRetryAfterMs() int64 {
	if x != nil {
		return x.RetryAfterMs
	}
	return 0
}

func (x *CheckResponse) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

var File_proto_shardroute_proto protoreflect.FileDescriptor

const file_proto_shardroute_proto_rawDesc = "" +
	"\n" +
	"\x16proto/shardroute.proto\x12\rshardroute.v1\"S\n" +
	"\fCheckRequest\x12\x10\n" +
	"\x03key\x18\x01 \x01(\tR\x03key\x12\x12\n" +
	"\x04cost\x18\x02 \x01(\x05R\x04cost\x12\x1d\n" +
	"\n" +
	"limit_name\x18\x03 \x01(\tR\tlimitName\"\x90\x01\n" +
	"\rCheckResponse\x12\x18\n" +
	"\aallowed\x18\x01 \x01(\bR\aallowed\x12)\n" +
	"\x10tokens_remaining\x18\x02 \x01(\x01R\x0ftokensRemaining\x12$\n" +
	"\x0eretry_after_ms\x18\x03 \x01(\x03R\fretryAfterMs\x12\x14\n" +
	"\x05error\x18\x04 \x01(\tR\x05error2\x9f\x01\n" +
	"\vRateLimiter\x12B\n" +
	"\x05Check\x12\x1b.shardroute.v1.CheckRequest\x1a\x1c.shardroute.v1.CheckResponse\x12L\n" +
	"\vStreamCheck\x12\x1b.shardroute.v1.CheckRequest\x1a\x1c.shardroute.v1.CheckResponse(\x010\x01B+Z)github.com/psychic-coder/shardroute/protob\x06proto3"

var (
	file_proto_shardroute_proto_rawDescOnce sync.Once
	file_proto_shardroute_proto_rawDescData []byte
)

func file_proto_shardroute_proto_rawDescGZIP() []byte {
	file_proto_shardroute_proto_rawDescOnce.Do(func() {
		file_proto_shardroute_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_proto_shardroute_proto_rawDesc), len(file_proto_shardroute_proto_rawDesc)))
	})
	return file_proto_shardroute_proto_rawDescData
}

var file_proto_shardroute_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_proto_shardroute_proto_goTypes = []any{
	(*CheckRequest)(nil),
	(*CheckResponse)(nil),
}
var file_proto_shardroute_proto_depIdxs = []int32{
	0,
	0,
	1,
	1,
	2,
	0,
	0,
	0,
	0,
}

func init() { file_proto_shardroute_proto_init() }
func file_proto_shardroute_proto_init() {
	if File_proto_shardroute_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_proto_shardroute_proto_rawDesc), len(file_proto_shardroute_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_proto_shardroute_proto_goTypes,
		DependencyIndexes: file_proto_shardroute_proto_depIdxs,
		MessageInfos:      file_proto_shardroute_proto_msgTypes,
	}.Build()
	File_proto_shardroute_proto = out.File
	file_proto_shardroute_proto_goTypes = nil
	file_proto_shardroute_proto_depIdxs = nil
}
