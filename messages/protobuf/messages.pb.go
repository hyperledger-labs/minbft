// Code generated by protoc-gen-go. DO NOT EDIT.
// source: messages.proto

package protobuf

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type Message struct {
	// Types that are valid to be assigned to Type:
	//	*Message_Request
	//	*Message_Reply
	//	*Message_Prepare
	//	*Message_Commit
	Type                 isMessage_Type `protobuf_oneof:"type"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *Message) Reset()         { *m = Message{} }
func (m *Message) String() string { return proto.CompactTextString(m) }
func (*Message) ProtoMessage()    {}
func (*Message) Descriptor() ([]byte, []int) {
	return fileDescriptor_4dc296cbfe5ffcd5, []int{0}
}

func (m *Message) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Message.Unmarshal(m, b)
}
func (m *Message) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Message.Marshal(b, m, deterministic)
}
func (m *Message) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Message.Merge(m, src)
}
func (m *Message) XXX_Size() int {
	return xxx_messageInfo_Message.Size(m)
}
func (m *Message) XXX_DiscardUnknown() {
	xxx_messageInfo_Message.DiscardUnknown(m)
}

var xxx_messageInfo_Message proto.InternalMessageInfo

type isMessage_Type interface {
	isMessage_Type()
}

type Message_Request struct {
	Request *Request `protobuf:"bytes,1,opt,name=request,proto3,oneof"`
}

type Message_Reply struct {
	Reply *Reply `protobuf:"bytes,2,opt,name=reply,proto3,oneof"`
}

type Message_Prepare struct {
	Prepare *Prepare `protobuf:"bytes,3,opt,name=prepare,proto3,oneof"`
}

type Message_Commit struct {
	Commit *Commit `protobuf:"bytes,4,opt,name=commit,proto3,oneof"`
}

func (*Message_Request) isMessage_Type() {}

func (*Message_Reply) isMessage_Type() {}

func (*Message_Prepare) isMessage_Type() {}

func (*Message_Commit) isMessage_Type() {}

func (m *Message) GetType() isMessage_Type {
	if m != nil {
		return m.Type
	}
	return nil
}

func (m *Message) GetRequest() *Request {
	if x, ok := m.GetType().(*Message_Request); ok {
		return x.Request
	}
	return nil
}

func (m *Message) GetReply() *Reply {
	if x, ok := m.GetType().(*Message_Reply); ok {
		return x.Reply
	}
	return nil
}

func (m *Message) GetPrepare() *Prepare {
	if x, ok := m.GetType().(*Message_Prepare); ok {
		return x.Prepare
	}
	return nil
}

func (m *Message) GetCommit() *Commit {
	if x, ok := m.GetType().(*Message_Commit); ok {
		return x.Commit
	}
	return nil
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*Message) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*Message_Request)(nil),
		(*Message_Reply)(nil),
		(*Message_Prepare)(nil),
		(*Message_Commit)(nil),
	}
}

type Request struct {
	Msg                  *Request_M `protobuf:"bytes,1,opt,name=msg,proto3" json:"msg,omitempty"`
	Signature            []byte     `protobuf:"bytes,2,opt,name=signature,proto3" json:"signature,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *Request) Reset()         { *m = Request{} }
func (m *Request) String() string { return proto.CompactTextString(m) }
func (*Request) ProtoMessage()    {}
func (*Request) Descriptor() ([]byte, []int) {
	return fileDescriptor_4dc296cbfe5ffcd5, []int{1}
}

func (m *Request) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Request.Unmarshal(m, b)
}
func (m *Request) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Request.Marshal(b, m, deterministic)
}
func (m *Request) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Request.Merge(m, src)
}
func (m *Request) XXX_Size() int {
	return xxx_messageInfo_Request.Size(m)
}
func (m *Request) XXX_DiscardUnknown() {
	xxx_messageInfo_Request.DiscardUnknown(m)
}

var xxx_messageInfo_Request proto.InternalMessageInfo

func (m *Request) GetMsg() *Request_M {
	if m != nil {
		return m.Msg
	}
	return nil
}

func (m *Request) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

type Request_M struct {
	ClientId             uint32   `protobuf:"varint,1,opt,name=client_id,json=clientId,proto3" json:"client_id,omitempty"`
	Seq                  uint64   `protobuf:"varint,2,opt,name=seq,proto3" json:"seq,omitempty"`
	Payload              []byte   `protobuf:"bytes,3,opt,name=payload,proto3" json:"payload,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Request_M) Reset()         { *m = Request_M{} }
func (m *Request_M) String() string { return proto.CompactTextString(m) }
func (*Request_M) ProtoMessage()    {}
func (*Request_M) Descriptor() ([]byte, []int) {
	return fileDescriptor_4dc296cbfe5ffcd5, []int{1, 0}
}

func (m *Request_M) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Request_M.Unmarshal(m, b)
}
func (m *Request_M) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Request_M.Marshal(b, m, deterministic)
}
func (m *Request_M) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Request_M.Merge(m, src)
}
func (m *Request_M) XXX_Size() int {
	return xxx_messageInfo_Request_M.Size(m)
}
func (m *Request_M) XXX_DiscardUnknown() {
	xxx_messageInfo_Request_M.DiscardUnknown(m)
}

var xxx_messageInfo_Request_M proto.InternalMessageInfo

func (m *Request_M) GetClientId() uint32 {
	if m != nil {
		return m.ClientId
	}
	return 0
}

func (m *Request_M) GetSeq() uint64 {
	if m != nil {
		return m.Seq
	}
	return 0
}

func (m *Request_M) GetPayload() []byte {
	if m != nil {
		return m.Payload
	}
	return nil
}

type Reply struct {
	Msg                  *Reply_M `protobuf:"bytes,1,opt,name=msg,proto3" json:"msg,omitempty"`
	Signature            []byte   `protobuf:"bytes,2,opt,name=signature,proto3" json:"signature,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Reply) Reset()         { *m = Reply{} }
func (m *Reply) String() string { return proto.CompactTextString(m) }
func (*Reply) ProtoMessage()    {}
func (*Reply) Descriptor() ([]byte, []int) {
	return fileDescriptor_4dc296cbfe5ffcd5, []int{2}
}

func (m *Reply) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Reply.Unmarshal(m, b)
}
func (m *Reply) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Reply.Marshal(b, m, deterministic)
}
func (m *Reply) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Reply.Merge(m, src)
}
func (m *Reply) XXX_Size() int {
	return xxx_messageInfo_Reply.Size(m)
}
func (m *Reply) XXX_DiscardUnknown() {
	xxx_messageInfo_Reply.DiscardUnknown(m)
}

var xxx_messageInfo_Reply proto.InternalMessageInfo

func (m *Reply) GetMsg() *Reply_M {
	if m != nil {
		return m.Msg
	}
	return nil
}

func (m *Reply) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

type Reply_M struct {
	ReplicaId            uint32   `protobuf:"varint,1,opt,name=replica_id,json=replicaId,proto3" json:"replica_id,omitempty"`
	ClientId             uint32   `protobuf:"varint,2,opt,name=client_id,json=clientId,proto3" json:"client_id,omitempty"`
	Seq                  uint64   `protobuf:"varint,3,opt,name=seq,proto3" json:"seq,omitempty"`
	Result               []byte   `protobuf:"bytes,4,opt,name=result,proto3" json:"result,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Reply_M) Reset()         { *m = Reply_M{} }
func (m *Reply_M) String() string { return proto.CompactTextString(m) }
func (*Reply_M) ProtoMessage()    {}
func (*Reply_M) Descriptor() ([]byte, []int) {
	return fileDescriptor_4dc296cbfe5ffcd5, []int{2, 0}
}

func (m *Reply_M) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Reply_M.Unmarshal(m, b)
}
func (m *Reply_M) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Reply_M.Marshal(b, m, deterministic)
}
func (m *Reply_M) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Reply_M.Merge(m, src)
}
func (m *Reply_M) XXX_Size() int {
	return xxx_messageInfo_Reply_M.Size(m)
}
func (m *Reply_M) XXX_DiscardUnknown() {
	xxx_messageInfo_Reply_M.DiscardUnknown(m)
}

var xxx_messageInfo_Reply_M proto.InternalMessageInfo

func (m *Reply_M) GetReplicaId() uint32 {
	if m != nil {
		return m.ReplicaId
	}
	return 0
}

func (m *Reply_M) GetClientId() uint32 {
	if m != nil {
		return m.ClientId
	}
	return 0
}

func (m *Reply_M) GetSeq() uint64 {
	if m != nil {
		return m.Seq
	}
	return 0
}

func (m *Reply_M) GetResult() []byte {
	if m != nil {
		return m.Result
	}
	return nil
}

type Prepare struct {
	Msg                  *Prepare_M `protobuf:"bytes,1,opt,name=msg,proto3" json:"msg,omitempty"`
	ReplicaUi            []byte     `protobuf:"bytes,2,opt,name=replica_ui,json=replicaUi,proto3" json:"replica_ui,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *Prepare) Reset()         { *m = Prepare{} }
func (m *Prepare) String() string { return proto.CompactTextString(m) }
func (*Prepare) ProtoMessage()    {}
func (*Prepare) Descriptor() ([]byte, []int) {
	return fileDescriptor_4dc296cbfe5ffcd5, []int{3}
}

func (m *Prepare) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Prepare.Unmarshal(m, b)
}
func (m *Prepare) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Prepare.Marshal(b, m, deterministic)
}
func (m *Prepare) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Prepare.Merge(m, src)
}
func (m *Prepare) XXX_Size() int {
	return xxx_messageInfo_Prepare.Size(m)
}
func (m *Prepare) XXX_DiscardUnknown() {
	xxx_messageInfo_Prepare.DiscardUnknown(m)
}

var xxx_messageInfo_Prepare proto.InternalMessageInfo

func (m *Prepare) GetMsg() *Prepare_M {
	if m != nil {
		return m.Msg
	}
	return nil
}

func (m *Prepare) GetReplicaUi() []byte {
	if m != nil {
		return m.ReplicaUi
	}
	return nil
}

type Prepare_M struct {
	View                 uint64   `protobuf:"varint,1,opt,name=view,proto3" json:"view,omitempty"`
	ReplicaId            uint32   `protobuf:"varint,2,opt,name=replica_id,json=replicaId,proto3" json:"replica_id,omitempty"`
	Request              *Request `protobuf:"bytes,3,opt,name=request,proto3" json:"request,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Prepare_M) Reset()         { *m = Prepare_M{} }
func (m *Prepare_M) String() string { return proto.CompactTextString(m) }
func (*Prepare_M) ProtoMessage()    {}
func (*Prepare_M) Descriptor() ([]byte, []int) {
	return fileDescriptor_4dc296cbfe5ffcd5, []int{3, 0}
}

func (m *Prepare_M) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Prepare_M.Unmarshal(m, b)
}
func (m *Prepare_M) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Prepare_M.Marshal(b, m, deterministic)
}
func (m *Prepare_M) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Prepare_M.Merge(m, src)
}
func (m *Prepare_M) XXX_Size() int {
	return xxx_messageInfo_Prepare_M.Size(m)
}
func (m *Prepare_M) XXX_DiscardUnknown() {
	xxx_messageInfo_Prepare_M.DiscardUnknown(m)
}

var xxx_messageInfo_Prepare_M proto.InternalMessageInfo

func (m *Prepare_M) GetView() uint64 {
	if m != nil {
		return m.View
	}
	return 0
}

func (m *Prepare_M) GetReplicaId() uint32 {
	if m != nil {
		return m.ReplicaId
	}
	return 0
}

func (m *Prepare_M) GetRequest() *Request {
	if m != nil {
		return m.Request
	}
	return nil
}

type Commit struct {
	Msg                  *Commit_M `protobuf:"bytes,1,opt,name=msg,proto3" json:"msg,omitempty"`
	ReplicaUi            []byte    `protobuf:"bytes,2,opt,name=replica_ui,json=replicaUi,proto3" json:"replica_ui,omitempty"`
	XXX_NoUnkeyedLiteral struct{}  `json:"-"`
	XXX_unrecognized     []byte    `json:"-"`
	XXX_sizecache        int32     `json:"-"`
}

func (m *Commit) Reset()         { *m = Commit{} }
func (m *Commit) String() string { return proto.CompactTextString(m) }
func (*Commit) ProtoMessage()    {}
func (*Commit) Descriptor() ([]byte, []int) {
	return fileDescriptor_4dc296cbfe5ffcd5, []int{4}
}

func (m *Commit) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Commit.Unmarshal(m, b)
}
func (m *Commit) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Commit.Marshal(b, m, deterministic)
}
func (m *Commit) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Commit.Merge(m, src)
}
func (m *Commit) XXX_Size() int {
	return xxx_messageInfo_Commit.Size(m)
}
func (m *Commit) XXX_DiscardUnknown() {
	xxx_messageInfo_Commit.DiscardUnknown(m)
}

var xxx_messageInfo_Commit proto.InternalMessageInfo

func (m *Commit) GetMsg() *Commit_M {
	if m != nil {
		return m.Msg
	}
	return nil
}

func (m *Commit) GetReplicaUi() []byte {
	if m != nil {
		return m.ReplicaUi
	}
	return nil
}

type Commit_M struct {
	View                 uint64   `protobuf:"varint,1,opt,name=view,proto3" json:"view,omitempty"`
	ReplicaId            uint32   `protobuf:"varint,2,opt,name=replica_id,json=replicaId,proto3" json:"replica_id,omitempty"`
	PrimaryId            uint32   `protobuf:"varint,3,opt,name=primary_id,json=primaryId,proto3" json:"primary_id,omitempty"`
	Request              *Request `protobuf:"bytes,4,opt,name=request,proto3" json:"request,omitempty"`
	PrimaryUi            []byte   `protobuf:"bytes,5,opt,name=primary_ui,json=primaryUi,proto3" json:"primary_ui,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Commit_M) Reset()         { *m = Commit_M{} }
func (m *Commit_M) String() string { return proto.CompactTextString(m) }
func (*Commit_M) ProtoMessage()    {}
func (*Commit_M) Descriptor() ([]byte, []int) {
	return fileDescriptor_4dc296cbfe5ffcd5, []int{4, 0}
}

func (m *Commit_M) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Commit_M.Unmarshal(m, b)
}
func (m *Commit_M) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Commit_M.Marshal(b, m, deterministic)
}
func (m *Commit_M) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Commit_M.Merge(m, src)
}
func (m *Commit_M) XXX_Size() int {
	return xxx_messageInfo_Commit_M.Size(m)
}
func (m *Commit_M) XXX_DiscardUnknown() {
	xxx_messageInfo_Commit_M.DiscardUnknown(m)
}

var xxx_messageInfo_Commit_M proto.InternalMessageInfo

func (m *Commit_M) GetView() uint64 {
	if m != nil {
		return m.View
	}
	return 0
}

func (m *Commit_M) GetReplicaId() uint32 {
	if m != nil {
		return m.ReplicaId
	}
	return 0
}

func (m *Commit_M) GetPrimaryId() uint32 {
	if m != nil {
		return m.PrimaryId
	}
	return 0
}

func (m *Commit_M) GetRequest() *Request {
	if m != nil {
		return m.Request
	}
	return nil
}

func (m *Commit_M) GetPrimaryUi() []byte {
	if m != nil {
		return m.PrimaryUi
	}
	return nil
}

func init() {
	proto.RegisterType((*Message)(nil), "protobuf.Message")
	proto.RegisterType((*Request)(nil), "protobuf.Request")
	proto.RegisterType((*Request_M)(nil), "protobuf.Request.M")
	proto.RegisterType((*Reply)(nil), "protobuf.Reply")
	proto.RegisterType((*Reply_M)(nil), "protobuf.Reply.M")
	proto.RegisterType((*Prepare)(nil), "protobuf.Prepare")
	proto.RegisterType((*Prepare_M)(nil), "protobuf.Prepare.M")
	proto.RegisterType((*Commit)(nil), "protobuf.Commit")
	proto.RegisterType((*Commit_M)(nil), "protobuf.Commit.M")
}

func init() { proto.RegisterFile("messages.proto", fileDescriptor_4dc296cbfe5ffcd5) }

var fileDescriptor_4dc296cbfe5ffcd5 = []byte{
	// 421 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x92, 0x5f, 0xaa, 0xd3, 0x40,
	0x14, 0xc6, 0x3b, 0x4d, 0x9a, 0xb4, 0xc7, 0xaa, 0xed, 0x08, 0x12, 0xaa, 0x05, 0x89, 0x8a, 0xa2,
	0x98, 0x07, 0xdd, 0x81, 0xbe, 0xb4, 0x60, 0x41, 0x06, 0xfa, 0x2c, 0x69, 0x32, 0x96, 0x81, 0xa4,
	0x99, 0xce, 0x24, 0x4a, 0xf6, 0xe2, 0x8b, 0x7b, 0xd0, 0x1d, 0xb8, 0x21, 0x77, 0x20, 0xf3, 0x27,
	0x37, 0x69, 0xee, 0x2d, 0xb7, 0xdc, 0xa7, 0xcc, 0x9c, 0xef, 0x9b, 0xc3, 0xef, 0x3b, 0x39, 0xf0,
	0x20, 0xa7, 0x52, 0xc6, 0x7b, 0x2a, 0x23, 0x2e, 0x8a, 0xb2, 0xc0, 0x63, 0xfd, 0xd9, 0x55, 0xdf,
	0xc2, 0xbf, 0x08, 0xfc, 0x8d, 0x11, 0xf1, 0x3b, 0xf0, 0x05, 0x3d, 0x56, 0x54, 0x96, 0x01, 0x7a,
	0x86, 0x5e, 0xdf, 0x7b, 0x3f, 0x8f, 0x1a, 0x5f, 0x44, 0x8c, 0xb0, 0x1a, 0x90, 0xc6, 0x83, 0x5f,
	0xc1, 0x48, 0x50, 0x9e, 0xd5, 0xc1, 0x50, 0x9b, 0x1f, 0x76, 0xcd, 0x3c, 0xab, 0x57, 0x03, 0x62,
	0x74, 0xd5, 0x97, 0x0b, 0xca, 0x63, 0x41, 0x03, 0xa7, 0xdf, 0xf7, 0x8b, 0x11, 0x54, 0x5f, 0xeb,
	0xc1, 0x6f, 0xc0, 0x4b, 0x8a, 0x3c, 0x67, 0x65, 0xe0, 0x6a, 0xf7, 0xac, 0x75, 0x7f, 0xd2, 0xf5,
	0xd5, 0x80, 0x58, 0xc7, 0x47, 0x0f, 0xdc, 0xb2, 0xe6, 0x34, 0xfc, 0x89, 0xc0, 0xb7, 0x88, 0xf8,
	0x25, 0x38, 0xb9, 0xdc, 0xdb, 0x08, 0x8f, 0xae, 0x45, 0x88, 0x36, 0x44, 0xe9, 0xf8, 0x29, 0x4c,
	0x24, 0xdb, 0x1f, 0xe2, 0xb2, 0x12, 0x54, 0x47, 0x98, 0x92, 0xb6, 0xb0, 0xf8, 0x0c, 0x68, 0x83,
	0x9f, 0xc0, 0x24, 0xc9, 0x18, 0x3d, 0x94, 0x5f, 0x59, 0xaa, 0xfb, 0xdd, 0x27, 0x63, 0x53, 0x58,
	0xa7, 0x78, 0x06, 0x8e, 0xa4, 0x47, 0xfd, 0xd2, 0x25, 0xea, 0x88, 0x03, 0xf0, 0x79, 0x5c, 0x67,
	0x45, 0x9c, 0xea, 0x9c, 0x53, 0xd2, 0x5c, 0xc3, 0x3f, 0x08, 0x46, 0x7a, 0x28, 0xf8, 0x79, 0x17,
	0x6e, 0xde, 0x1b, 0xd9, 0x65, 0x68, 0x4c, 0xa1, 0x2d, 0x01, 0xd4, 0x70, 0x59, 0x12, 0xb7, 0x6c,
	0x13, 0x5b, 0x59, 0xa7, 0xa7, 0xe4, 0xc3, 0x9b, 0xc9, 0x9d, 0x96, 0xfc, 0x31, 0x78, 0x82, 0xca,
	0x2a, 0x33, 0x23, 0x9f, 0x12, 0x7b, 0x0b, 0x7f, 0x23, 0xf0, 0xed, 0x1f, 0x3a, 0x3b, 0x56, 0xab,
	0x37, 0xec, 0x1d, 0xb0, 0x8a, 0x35, 0xf0, 0xb6, 0xb2, 0x65, 0x8b, 0x44, 0xc1, 0x63, 0x70, 0xbf,
	0x33, 0xfa, 0x43, 0xf7, 0x72, 0x89, 0x3e, 0xf7, 0x02, 0x0d, 0xfb, 0x81, 0xde, 0xb6, 0xbb, 0xe9,
	0x9c, 0xd9, 0xcd, 0xab, 0xcd, 0x0c, 0xff, 0x21, 0xf0, 0xcc, 0xaa, 0xe0, 0x17, 0x5d, 0x6a, 0xdc,
	0xdf, 0xa4, 0x0b, 0xa1, 0x7f, 0xa1, 0x3b, 0x52, 0x2f, 0x01, 0xb8, 0x60, 0x79, 0x2c, 0x6a, 0x25,
	0x3b, 0x46, 0xb6, 0x95, 0xd3, 0x50, 0xee, 0x6d, 0xa1, 0xba, 0xbd, 0x2a, 0x16, 0x8c, 0x0c, 0xa3,
	0xad, 0x6c, 0xd9, 0xce, 0xd3, 0x2f, 0x3f, 0xfc, 0x0f, 0x00, 0x00, 0xff, 0xff, 0x0e, 0x89, 0xd4,
	0xc4, 0xeb, 0x03, 0x00, 0x00,
}
