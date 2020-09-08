package main

import (
	"context"
	"encoding/json"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/runtime/protoimpl"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)
import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	reflect "reflect"
)

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные

// Interceptors
func NewUnaryACLInterceptor(consumers map[string][]string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, grpc.Errorf(codes.Unauthenticated, "metadata parsing error")
		}

		names := md.Get("consumer")
		if len(names) == 0 {
			return nil, grpc.Errorf(codes.Unauthenticated, "no consumer's names")
		}
		if len(names) > 1 {
			return nil, grpc.Errorf(codes.Unauthenticated, "request must contains single consumer name")
		}

		name := names[0]
		methods, ok := consumers[name]
		if !ok {
			return nil, grpc.Errorf(codes.Unauthenticated, "unknown consumer's name: "+name)
		}

		var found = false
		for idx := range methods {
			isMultiAccess := methods[idx][len(methods[idx])-1:] == "*"
			if isMultiAccess {
				multiAccess := methods[idx][:len(methods[idx])-1]
				if strings.HasPrefix(info.FullMethod, multiAccess) {
					found = true
					break
				}
			}
			if methods[idx] == info.FullMethod {
				found = true
				break
			}
		}
		if !found {
			return nil, grpc.Errorf(codes.Unauthenticated, "unknown method: "+info.FullMethod)
		}

		h, err := handler(ctx, req)
		
		return h, err
	}
}

func NewStreamACLInterceptor(consumers map[string][]string) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		log.Printf(info.FullMethod)
		methName := strings.ReplaceAll(info.FullMethod, "AdminManager", "Admin")

		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return grpc.Errorf(codes.Unauthenticated, "metadata parsing error")
		}

		names := md.Get("consumer")
		if len(names) == 0 {
			return grpc.Errorf(codes.Unauthenticated, "no consumer's names")
		}
		if len(names) > 1 {
			return grpc.Errorf(codes.Unauthenticated, "request must contains single consumer name")
		}

		name := names[0]
		methods, ok := consumers[name]
		if !ok {
			return grpc.Errorf(codes.Unauthenticated, "unknown consumer's name: "+name)
		}

		var found bool
		for idx := range methods {
			isMultiAccess := methods[idx][len(methods[idx])-1:] == "*"
			if isMultiAccess {
				multiAccess := methods[idx][:len(methods[idx])-1]
				if strings.HasPrefix(methName, multiAccess) {
					found = true
					break
				}
			}
			if methods[idx] == methName {
				found = true
				break
			}
		}

		if !found {
			return grpc.Errorf(codes.Unauthenticated, "unknown method called: %s", methName)
		}

		log.Printf("OK")

		err := handler(srv, ss)

		return err
	}
}

func NewStat() *Stat {
	return &Stat{
		Timestamp:     time.Now().Unix(),
		ByMethod:      make(map[string]uint64),
		ByConsumer:    make(map[string]uint64),
	}
}

func GenerateEvt(ctx context.Context, c chan<- *Event, method string) {
	md, _ := metadata.FromIncomingContext(ctx)
	name := md.Get("consumer")[0]
	c <- &Event{
		Timestamp:     time.Now().Unix(),
		Consumer:      name,
		Method:        method,
		Host:          "127.0.0.1:8081",
	}
}

// AdminManager
type AdminManager struct {
	sync.Mutex
	Done       context.Context
	Chan       chan *Event

	Loggers    []LoggerView
	Staters      []StaterView
}

type LoggerView struct {
	Chan   chan <- *Event
	Done   context.Context
	Cancel context.CancelFunc
}

type StaterView struct {
	Chan   chan <- *Event
	Done   context.Context
	Cancel context.CancelFunc
}

func NewAdminManager(done context.Context) *AdminManager {
	a := &AdminManager{
		Done:        done,
		Chan:       make(chan *Event, 10),
	}
	go func() {
		// log.Printf("logging stream started")
		for {
			select {
			case <-done.Done():
				a.Lock()
				for _, logger := range a.Loggers {
					logger.Cancel()
				}
				for _, stater := range a.Staters {
					stater.Cancel()
				}
				a.Unlock()

				return
			case evt := <-a.Chan:
				a.Lock()
				for _, logger := range a.Loggers {
					logger.Chan <- evt
				}
				var newStaters []StaterView
				for _, stater := range a.Staters {
					select {
					case <-stater.Done.Done():
					default:
						stater.Chan <- evt
						newStaters = append(newStaters, stater)
					}
				}
				a.Staters = newStaters
				a.Unlock()
			}
		}
	}()

	return a
}

func (a *AdminManager) AddLogger(c chan <- *Event) context.Context {
	a.Lock()
	defer a.Unlock()

	ctx, cancel := context.WithCancel(a.Done)
	a.Loggers = append(a.Loggers,
		LoggerView{
			Chan:   c,
			Done:   ctx,
			Cancel: cancel,
		},
	)

	return ctx
}

func (a *AdminManager) AddStater(c chan <- *Event) context.Context {
	a.Lock()
	defer a.Unlock()

	ctx, cancel := context.WithCancel(a.Done)
	a.Staters = append(a.Staters,
		StaterView{
			Chan:   c,
			Done:   ctx,
			Cancel: cancel,
		},
	)

	return ctx
}

func (a *AdminManager) Logging(_ *Nothing, server Admin_LoggingServer) error {
	GenerateEvt(server.Context(), a.Chan, "/main.Admin/Logging")
	log.Printf("logger began")

	time.Sleep(200*time.Microsecond)
	c   := make(chan *Event, 10)
	ctx := a.AddLogger(c)

	for {
		select {
		case <-ctx.Done():
			close(c)
			log.Printf("logger ended")
			return status.Errorf(codes.OK, "logger method stopped")
		case evt := <-c:
			err := server.Send(evt)
			if err != nil {
				log.Printf("event send error: %s", err)
				return status.Errorf(codes.Internal, err.Error())
			}
		}
	}
}

func (a *AdminManager) Statistics(interval *StatInterval, server Admin_StatisticsServer) error {
	GenerateEvt(server.Context(), a.Chan, "/main.Admin/Statistics")
	time.Sleep(200*time.Microsecond)
	log.Printf("stater began")

	c   := make(chan *Event, 10)
	ctx := a.AddStater(c)

	stat := NewStat()

	var seconds = time.Duration(interval.IntervalSeconds)
	log.Printf("duration: %d", seconds)
	t := time.NewTimer(seconds*time.Second)

	for {
		select {
		case <-ctx.Done():
			t.Stop()
			log.Printf("stater ended")

			return status.Errorf(codes.OK, "stater stopped by context.Done()")
		case <-t.C:
			err := server.Send(stat)
			if err != nil {
				log.Printf("event sent error: %s", err)
				return status.Errorf(codes.Internal, err.Error())
			}
			stat = NewStat()

			t.Reset(seconds*time.Second)
		case evt := <-c:
			log.Printf("stater sended")
			stat.Timestamp = time.Now().Unix()

			if evt == nil {
				log.Printf("event is nil")
				continue
			}
			if _, ok := stat.ByMethod[evt.Method]; ok {
				stat.ByMethod[evt.Method]++
			} else {
				stat.ByMethod[evt.Method] = 1
			}

			if _, ok := stat.ByConsumer[evt.Consumer]; ok {
				stat.ByConsumer[evt.Consumer]++
			} else {
				stat.ByConsumer[evt.Consumer] = 1
			}
		}
	}
}

// Biz
type BizManager struct {
	AdminChann chan <- *Event
}

func NewBizManager(c chan <- *Event) *BizManager {
	return &BizManager{
		AdminChann: c,
	}
}

func (b *BizManager) Check(ctx context.Context, _ *Nothing) (*Nothing, error) {
	GenerateEvt(ctx, b.AdminChann, "/main.Biz/Check")

	return &Nothing{}, nil
}
func (b *BizManager) Add(ctx context.Context, _ *Nothing) (*Nothing, error) {
	GenerateEvt(ctx, b.AdminChann, "/main.Biz/Add")

	return &Nothing{}, nil
}
func (b *BizManager) Test(ctx context.Context, _ *Nothing) (*Nothing, error) {
	GenerateEvt(ctx, b.AdminChann, "/main.Biz/Test")

	return &Nothing{}, nil
}

func StartMyMicroservice(ctx context.Context, addr, aclData string) error {
	// Верификация Access Control List
	var acl map[string][]string
	err := json.Unmarshal([]byte(aclData), &acl)
	if err != nil {
		return err
	}

	// Настройка RPC-сервера
	server := grpc.NewServer(
		grpc.UnaryInterceptor(NewUnaryACLInterceptor(acl)),
		grpc.StreamInterceptor(NewStreamACLInterceptor(acl)),
	)

	adminCtx, finishAdmin := context.WithCancel(ctx)
	admin := NewAdminManager(adminCtx)
	biz   := NewBizManager(admin.Chan)

	RegisterAdminServer(server, admin)
	RegisterBizServer  (server, biz)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	// Запуск goroutine для сервера и для завершения работы сервера
	var c = make(chan error, 1)
	go func() {
		//log.Printf("server was ran")
		c <- server.Serve(lis)
	}()
	go func() {
		select {
		case <-ctx.Done():
			//log.Printf("server was finished")
		case err = <-c:
			//log.Printf("server was errored: %s", err)
		}

		finishAdmin()
		server.Stop()
	}()

	return nil
}

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type Event struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Timestamp int64  `protobuf:"varint,1,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	Consumer  string `protobuf:"bytes,2,opt,name=consumer,proto3" json:"consumer,omitempty"`
	Method    string `protobuf:"bytes,3,opt,name=method,proto3" json:"method,omitempty"`
	Host      string `protobuf:"bytes,4,opt,name=host,proto3" json:"host,omitempty"`
}

func (x *Event) Reset() {
	*x = Event{}
	if protoimpl.UnsafeEnabled {
		mi := &file_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Event) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Event) ProtoMessage() {}

func (x *Event) ProtoReflect() protoreflect.Message {
	mi := &file_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Event.ProtoReflect.Descriptor instead.
func (*Event) Descriptor() ([]byte, []int) {
	return file_service_proto_rawDescGZIP(), []int{0}
}

func (x *Event) GetTimestamp() int64 {
	if x != nil {
		return x.Timestamp
	}
	return 0
}

func (x *Event) GetConsumer() string {
	if x != nil {
		return x.Consumer
	}
	return ""
}

func (x *Event) GetMethod() string {
	if x != nil {
		return x.Method
	}
	return ""
}

func (x *Event) GetHost() string {
	if x != nil {
		return x.Host
	}
	return ""
}

type Stat struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Timestamp  int64             `protobuf:"varint,1,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	ByMethod   map[string]uint64 `protobuf:"bytes,2,rep,name=by_method,json=byMethod,proto3" json:"by_method,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"varint,2,opt,name=value,proto3"`
	ByConsumer map[string]uint64 `protobuf:"bytes,3,rep,name=by_consumer,json=byConsumer,proto3" json:"by_consumer,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"varint,2,opt,name=value,proto3"`
}

func (x *Stat) Reset() {
	*x = Stat{}
	if protoimpl.UnsafeEnabled {
		mi := &file_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Stat) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Stat) ProtoMessage() {}

func (x *Stat) ProtoReflect() protoreflect.Message {
	mi := &file_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Stat.ProtoReflect.Descriptor instead.
func (*Stat) Descriptor() ([]byte, []int) {
	return file_service_proto_rawDescGZIP(), []int{1}
}

func (x *Stat) GetTimestamp() int64 {
	if x != nil {
		return x.Timestamp
	}
	return 0
}

func (x *Stat) GetByMethod() map[string]uint64 {
	if x != nil {
		return x.ByMethod
	}
	return nil
}

func (x *Stat) GetByConsumer() map[string]uint64 {
	if x != nil {
		return x.ByConsumer
	}
	return nil
}

type StatInterval struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	IntervalSeconds uint64 `protobuf:"varint,1,opt,name=interval_seconds,json=intervalSeconds,proto3" json:"interval_seconds,omitempty"`
}

func (x *StatInterval) Reset() {
	*x = StatInterval{}
	if protoimpl.UnsafeEnabled {
		mi := &file_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StatInterval) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StatInterval) ProtoMessage() {}

func (x *StatInterval) ProtoReflect() protoreflect.Message {
	mi := &file_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StatInterval.ProtoReflect.Descriptor instead.
func (*StatInterval) Descriptor() ([]byte, []int) {
	return file_service_proto_rawDescGZIP(), []int{2}
}

func (x *StatInterval) GetIntervalSeconds() uint64 {
	if x != nil {
		return x.IntervalSeconds
	}
	return 0
}

type Nothing struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Dummy bool `protobuf:"varint,1,opt,name=dummy,proto3" json:"dummy,omitempty"`
}

func (x *Nothing) Reset() {
	*x = Nothing{}
	if protoimpl.UnsafeEnabled {
		mi := &file_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Nothing) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Nothing) ProtoMessage() {}

func (x *Nothing) ProtoReflect() protoreflect.Message {
	mi := &file_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Nothing.ProtoReflect.Descriptor instead.
func (*Nothing) Descriptor() ([]byte, []int) {
	return file_service_proto_rawDescGZIP(), []int{3}
}

func (x *Nothing) GetDummy() bool {
	if x != nil {
		return x.Dummy
	}
	return false
}

var File_service_proto protoreflect.FileDescriptor

var file_service_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x04, 0x6d, 0x61, 0x69, 0x6e, 0x22, 0x6d, 0x0a, 0x05, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x12, 0x1c,
	0x0a, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x03, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x1a, 0x0a, 0x08,
	0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6d, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08,
	0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6d, 0x65, 0x72, 0x12, 0x16, 0x0a, 0x06, 0x6d, 0x65, 0x74, 0x68,
	0x6f, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64,
	0x12, 0x12, 0x0a, 0x04, 0x68, 0x6f, 0x73, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x68, 0x6f, 0x73, 0x74, 0x22, 0x94, 0x02, 0x0a, 0x04, 0x53, 0x74, 0x61, 0x74, 0x12, 0x1c, 0x0a,
	0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x35, 0x0a, 0x09, 0x62,
	0x79, 0x5f, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x18,
	0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x2e, 0x42, 0x79, 0x4d, 0x65, 0x74,
	0x68, 0x6f, 0x64, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x08, 0x62, 0x79, 0x4d, 0x65, 0x74, 0x68,
	0x6f, 0x64, 0x12, 0x3b, 0x0a, 0x0b, 0x62, 0x79, 0x5f, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6d, 0x65,
	0x72, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x53,
	0x74, 0x61, 0x74, 0x2e, 0x42, 0x79, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6d, 0x65, 0x72, 0x45, 0x6e,
	0x74, 0x72, 0x79, 0x52, 0x0a, 0x62, 0x79, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6d, 0x65, 0x72, 0x1a,
	0x3b, 0x0a, 0x0d, 0x42, 0x79, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x45, 0x6e, 0x74, 0x72, 0x79,
	0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b,
	0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x04, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x1a, 0x3d, 0x0a, 0x0f,
	0x42, 0x79, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6d, 0x65, 0x72, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12,
	0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65,
	0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x39, 0x0a, 0x0c, 0x53,
	0x74, 0x61, 0x74, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x12, 0x29, 0x0a, 0x10, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x5f, 0x73, 0x65, 0x63, 0x6f, 0x6e, 0x64, 0x73, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x53,
	0x65, 0x63, 0x6f, 0x6e, 0x64, 0x73, 0x22, 0x1f, 0x0a, 0x07, 0x4e, 0x6f, 0x74, 0x68, 0x69, 0x6e,
	0x67, 0x12, 0x14, 0x0a, 0x05, 0x64, 0x75, 0x6d, 0x6d, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x05, 0x64, 0x75, 0x6d, 0x6d, 0x79, 0x32, 0x64, 0x0a, 0x05, 0x41, 0x64, 0x6d, 0x69, 0x6e,
	0x12, 0x29, 0x0a, 0x07, 0x4c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67, 0x12, 0x0d, 0x2e, 0x6d, 0x61,
	0x69, 0x6e, 0x2e, 0x4e, 0x6f, 0x74, 0x68, 0x69, 0x6e, 0x67, 0x1a, 0x0b, 0x2e, 0x6d, 0x61, 0x69,
	0x6e, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x22, 0x00, 0x30, 0x01, 0x12, 0x30, 0x0a, 0x0a, 0x53,
	0x74, 0x61, 0x74, 0x69, 0x73, 0x74, 0x69, 0x63, 0x73, 0x12, 0x12, 0x2e, 0x6d, 0x61, 0x69, 0x6e,
	0x2e, 0x53, 0x74, 0x61, 0x74, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x1a, 0x0a, 0x2e,
	0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x22, 0x00, 0x30, 0x01, 0x32, 0x7d, 0x0a,
	0x03, 0x42, 0x69, 0x7a, 0x12, 0x27, 0x0a, 0x05, 0x43, 0x68, 0x65, 0x63, 0x6b, 0x12, 0x0d, 0x2e,
	0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x4e, 0x6f, 0x74, 0x68, 0x69, 0x6e, 0x67, 0x1a, 0x0d, 0x2e, 0x6d,
	0x61, 0x69, 0x6e, 0x2e, 0x4e, 0x6f, 0x74, 0x68, 0x69, 0x6e, 0x67, 0x22, 0x00, 0x12, 0x25, 0x0a,
	0x03, 0x41, 0x64, 0x64, 0x12, 0x0d, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x4e, 0x6f, 0x74, 0x68,
	0x69, 0x6e, 0x67, 0x1a, 0x0d, 0x2e, 0x6d, 0x61, 0x69, 0x6e, 0x2e, 0x4e, 0x6f, 0x74, 0x68, 0x69,
	0x6e, 0x67, 0x22, 0x00, 0x12, 0x26, 0x0a, 0x04, 0x54, 0x65, 0x73, 0x74, 0x12, 0x0d, 0x2e, 0x6d,
	0x61, 0x69, 0x6e, 0x2e, 0x4e, 0x6f, 0x74, 0x68, 0x69, 0x6e, 0x67, 0x1a, 0x0d, 0x2e, 0x6d, 0x61,
	0x69, 0x6e, 0x2e, 0x4e, 0x6f, 0x74, 0x68, 0x69, 0x6e, 0x67, 0x22, 0x00, 0x42, 0x08, 0x5a, 0x06,
	0x2e, 0x3b, 0x6d, 0x61, 0x69, 0x6e, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_service_proto_rawDescOnce sync.Once
	file_service_proto_rawDescData = file_service_proto_rawDesc
)

func file_service_proto_rawDescGZIP() []byte {
	file_service_proto_rawDescOnce.Do(func() {
		file_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_service_proto_rawDescData)
	})
	return file_service_proto_rawDescData
}

var file_service_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_service_proto_goTypes = []interface{}{
	(*Event)(nil),        // 0: main.Event
	(*Stat)(nil),         // 1: main.Stat
	(*StatInterval)(nil), // 2: main.StatInterval
	(*Nothing)(nil),      // 3: main.Nothing
	nil,                  // 4: main.Stat.ByMethodEntry
	nil,                  // 5: main.Stat.ByConsumerEntry
}
var file_service_proto_depIdxs = []int32{
	4, // 0: main.Stat.by_method:type_name -> main.Stat.ByMethodEntry
	5, // 1: main.Stat.by_consumer:type_name -> main.Stat.ByConsumerEntry
	3, // 2: main.Admin.Logging:input_type -> main.Nothing
	2, // 3: main.Admin.Statistics:input_type -> main.StatInterval
	3, // 4: main.Biz.Check:input_type -> main.Nothing
	3, // 5: main.Biz.Add:input_type -> main.Nothing
	3, // 6: main.Biz.Test:input_type -> main.Nothing
	0, // 7: main.Admin.Logging:output_type -> main.Event
	1, // 8: main.Admin.Statistics:output_type -> main.Stat
	3, // 9: main.Biz.Check:output_type -> main.Nothing
	3, // 10: main.Biz.Add:output_type -> main.Nothing
	3, // 11: main.Biz.Test:output_type -> main.Nothing
	7, // [7:12] is the sub-list for method output_type
	2, // [2:7] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_service_proto_init() }
func file_service_proto_init() {
	if File_service_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Event); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_service_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Stat); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_service_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StatInterval); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_service_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Nothing); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   2,
		},
		GoTypes:           file_service_proto_goTypes,
		DependencyIndexes: file_service_proto_depIdxs,
		MessageInfos:      file_service_proto_msgTypes,
	}.Build()
	File_service_proto = out.File
	file_service_proto_rawDesc = nil
	file_service_proto_goTypes = nil
	file_service_proto_depIdxs = nil
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// AdminClient is the client API for Admin service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type AdminClient interface {
	Logging(ctx context.Context, in *Nothing, opts ...grpc.CallOption) (Admin_LoggingClient, error)
	Statistics(ctx context.Context, in *StatInterval, opts ...grpc.CallOption) (Admin_StatisticsClient, error)
}

type adminClient struct {
	cc grpc.ClientConnInterface
}

func NewAdminClient(cc grpc.ClientConnInterface) AdminClient {
	return &adminClient{cc}
}

func (c *adminClient) Logging(ctx context.Context, in *Nothing, opts ...grpc.CallOption) (Admin_LoggingClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Admin_serviceDesc.Streams[0], "/main.Admin/Logging", opts...)
	if err != nil {
		return nil, err
	}
	x := &adminLoggingClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Admin_LoggingClient interface {
	Recv() (*Event, error)
	grpc.ClientStream
}

type adminLoggingClient struct {
	grpc.ClientStream
}

func (x *adminLoggingClient) Recv() (*Event, error) {
	m := new(Event)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *adminClient) Statistics(ctx context.Context, in *StatInterval, opts ...grpc.CallOption) (Admin_StatisticsClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Admin_serviceDesc.Streams[1], "/main.Admin/Statistics", opts...)
	if err != nil {
		return nil, err
	}
	x := &adminStatisticsClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Admin_StatisticsClient interface {
	Recv() (*Stat, error)
	grpc.ClientStream
}

type adminStatisticsClient struct {
	grpc.ClientStream
}

func (x *adminStatisticsClient) Recv() (*Stat, error) {
	m := new(Stat)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// AdminServer is the server API for Admin service.
type AdminServer interface {
	Logging(*Nothing, Admin_LoggingServer) error
	Statistics(*StatInterval, Admin_StatisticsServer) error
}

// UnimplementedAdminServer can be embedded to have forward compatible implementations.
type UnimplementedAdminServer struct {
}

func (*UnimplementedAdminServer) Logging(*Nothing, Admin_LoggingServer) error {
	return status.Errorf(codes.Unimplemented, "method Logging not implemented")
}
func (*UnimplementedAdminServer) Statistics(*StatInterval, Admin_StatisticsServer) error {
	return status.Errorf(codes.Unimplemented, "method Statistics not implemented")
}

func RegisterAdminServer(s *grpc.Server, srv AdminServer) {
	s.RegisterService(&_Admin_serviceDesc, srv)
}

func _Admin_Logging_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Nothing)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(AdminServer).Logging(m, &adminLoggingServer{stream})
}

type Admin_LoggingServer interface {
	Send(*Event) error
	grpc.ServerStream
}

type adminLoggingServer struct {
	grpc.ServerStream
}

func (x *adminLoggingServer) Send(m *Event) error {
	return x.ServerStream.SendMsg(m)
}

func _Admin_Statistics_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(StatInterval)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(AdminServer).Statistics(m, &adminStatisticsServer{stream})
}

type Admin_StatisticsServer interface {
	Send(*Stat) error
	grpc.ServerStream
}

type adminStatisticsServer struct {
	grpc.ServerStream
}

func (x *adminStatisticsServer) Send(m *Stat) error {
	return x.ServerStream.SendMsg(m)
}

var _Admin_serviceDesc = grpc.ServiceDesc{
	ServiceName: "main.Admin",
	HandlerType: (*AdminServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Logging",
			Handler:       _Admin_Logging_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Statistics",
			Handler:       _Admin_Statistics_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "service.proto",
}

// BizClient is the client API for Biz service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type BizClient interface {
	Check(ctx context.Context, in *Nothing, opts ...grpc.CallOption) (*Nothing, error)
	Add(ctx context.Context, in *Nothing, opts ...grpc.CallOption) (*Nothing, error)
	Test(ctx context.Context, in *Nothing, opts ...grpc.CallOption) (*Nothing, error)
}

type bizClient struct {
	cc grpc.ClientConnInterface
}

func NewBizClient(cc grpc.ClientConnInterface) BizClient {
	return &bizClient{cc}
}

func (c *bizClient) Check(ctx context.Context, in *Nothing, opts ...grpc.CallOption) (*Nothing, error) {
	out := new(Nothing)
	err := c.cc.Invoke(ctx, "/main.Biz/Check", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bizClient) Add(ctx context.Context, in *Nothing, opts ...grpc.CallOption) (*Nothing, error) {
	out := new(Nothing)
	err := c.cc.Invoke(ctx, "/main.Biz/Add", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *bizClient) Test(ctx context.Context, in *Nothing, opts ...grpc.CallOption) (*Nothing, error) {
	out := new(Nothing)
	err := c.cc.Invoke(ctx, "/main.Biz/Test", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// BizServer is the server API for Biz service.
type BizServer interface {
	Check(context.Context, *Nothing) (*Nothing, error)
	Add(context.Context, *Nothing) (*Nothing, error)
	Test(context.Context, *Nothing) (*Nothing, error)
}

// UnimplementedBizServer can be embedded to have forward compatible implementations.
type UnimplementedBizServer struct {
}

func (*UnimplementedBizServer) Check(context.Context, *Nothing) (*Nothing, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Check not implemented")
}
func (*UnimplementedBizServer) Add(context.Context, *Nothing) (*Nothing, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Add not implemented")
}
func (*UnimplementedBizServer) Test(context.Context, *Nothing) (*Nothing, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Test not implemented")
}

func RegisterBizServer(s *grpc.Server, srv BizServer) {
	s.RegisterService(&_Biz_serviceDesc, srv)
}

func _Biz_Check_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Nothing)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BizServer).Check(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/main.Biz/Check",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BizServer).Check(ctx, req.(*Nothing))
	}
	return interceptor(ctx, in, info, handler)
}

func _Biz_Add_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Nothing)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BizServer).Add(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/main.Biz/Add",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BizServer).Add(ctx, req.(*Nothing))
	}
	return interceptor(ctx, in, info, handler)
}

func _Biz_Test_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Nothing)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BizServer).Test(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/main.Biz/Test",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BizServer).Test(ctx, req.(*Nothing))
	}
	return interceptor(ctx, in, info, handler)
}

var _Biz_serviceDesc = grpc.ServiceDesc{
	ServiceName: "main.Biz",
	HandlerType: (*BizServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Check",
			Handler:    _Biz_Check_Handler,
		},
		{
			MethodName: "Add",
			Handler:    _Biz_Add_Handler,
		},
		{
			MethodName: "Test",
			Handler:    _Biz_Test_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "service.proto",
}

