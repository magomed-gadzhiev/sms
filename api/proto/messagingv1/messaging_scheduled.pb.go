// Hand-written protobuf-compatible types for ListScheduledMessages RPC.
// These supplement the generated messaging.pb.go until protoc is re-run.

package messagingv1

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

var (
	file_messaging_scheduled_msgTypes = make([]protoimpl.MessageInfo, 2)
)

type ListScheduledMessagesRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ClientId string `protobuf:"bytes,1,opt,name=client_id,json=clientId,proto3" json:"client_id,omitempty"`
	Limit    int32  `protobuf:"varint,2,opt,name=limit,proto3" json:"limit,omitempty"`
	Offset   int32  `protobuf:"varint,3,opt,name=offset,proto3" json:"offset,omitempty"`
}

func (x *ListScheduledMessagesRequest) Reset() {
	*x = ListScheduledMessagesRequest{}
}

func (x *ListScheduledMessagesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListScheduledMessagesRequest) ProtoMessage() {}

func (x *ListScheduledMessagesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_messaging_scheduled_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *ListScheduledMessagesRequest) GetClientId() string {
	if x != nil {
		return x.ClientId
	}
	return ""
}

func (x *ListScheduledMessagesRequest) GetLimit() int32 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *ListScheduledMessagesRequest) GetOffset() int32 {
	if x != nil {
		return x.Offset
	}
	return 0
}

type ListScheduledMessagesResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Messages []*MessageInfo `protobuf:"bytes,1,rep,name=messages,proto3" json:"messages,omitempty"`
	Total    int32          `protobuf:"varint,2,opt,name=total,proto3" json:"total,omitempty"`
	Limit    int32          `protobuf:"varint,3,opt,name=limit,proto3" json:"limit,omitempty"`
	Offset   int32          `protobuf:"varint,4,opt,name=offset,proto3" json:"offset,omitempty"`
}

func (x *ListScheduledMessagesResponse) Reset() {
	*x = ListScheduledMessagesResponse{}
}

func (x *ListScheduledMessagesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListScheduledMessagesResponse) ProtoMessage() {}

func (x *ListScheduledMessagesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_messaging_scheduled_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *ListScheduledMessagesResponse) GetMessages() []*MessageInfo {
	if x != nil {
		return x.Messages
	}
	return nil
}

func (x *ListScheduledMessagesResponse) GetTotal() int32 {
	if x != nil {
		return x.Total
	}
	return 0
}

func (x *ListScheduledMessagesResponse) GetLimit() int32 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *ListScheduledMessagesResponse) GetOffset() int32 {
	if x != nil {
		return x.Offset
	}
	return 0
}
