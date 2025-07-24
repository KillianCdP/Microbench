package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"sync"
	"time"

	pb "github.com/KillianCdP/MicroBench/pkg/proto"
	"github.com/KillianCdP/MicroBench/pkg/tracing"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"go.opentelemetry.io/otel/attribute"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type Service struct {
	pb.UnimplementedBenchmarkServiceServer
	name            string
	outServices     []string
	processingDelay time.Duration
	rps             int
	connPool        map[string]*grpc.ClientConn
	connPoolMu      sync.RWMutex
	tracer          *tracing.Tracer
	otelTracer      trace.Tracer
	topology        string
	cni             string
}

type ServiceConfig struct {
	Name            string
	OutServices     []string
	ProcessingDelay time.Duration
	RPS             int
	Logger          *slog.Logger
	Topology        string
	CNI             string
	TracerProvider  *tracesdk.TracerProvider
}

type TraceLog struct {
	Topology       string `json:"topology"`
	BenchID        string `json:"bench_id"`
	CNI            string `json:"cni"`
	Operation      string `json:"operation"`
	Timestamp      int64  `json:"timestamp"`
	TraceID        string `json:"trace_id"`
	ServiceName    string `json:"service_name"`
	RelatedService string `json:"related_service"`
}

func NewService(config ServiceConfig) *Service {
	tracer := tracing.NewTracer(config.Logger, config.Topology, config.CNI, config.Name)

	var otelTracer trace.Tracer
	if config.TracerProvider != nil {
		otelTracer = config.TracerProvider.Tracer("service-tracer")
	}

	return &Service{
		name:            config.Name,
		outServices:     config.OutServices,
		processingDelay: config.ProcessingDelay,
		rps:             config.RPS,
		connPool:        make(map[string]*grpc.ClientConn),
		tracer:          tracer,
		otelTracer:      otelTracer,
		topology:        config.Topology,
		cni:             config.CNI,
	}
}

func (s *Service) Preconnect() {
	for _, svc := range s.outServices {
		if _, err := s.getConnection(svc); err != nil {
			slog.Error("preconnect failed", "service", svc, "error", err)
		} else {
			slog.Info("preconnect success", "service", svc)
		}
	}
}

func (s *Service) callService(ctx context.Context, serviceName, benchID string, traceID string, depth int32) (*pb.Message, error) {
	if s.otelTracer != nil {
		var span trace.Span
		ctx, span = s.otelTracer.Start(ctx, "call_service",
			trace.WithAttributes(
				semconv.RPCSystemKey.String("grpc"),
				semconv.RPCServiceKey.String(serviceName),
				attribute.String("peer.service", serviceName),
				attribute.String("trace.id", traceID),
				attribute.String("bench.id", benchID),
			))
		defer span.End()
	}

	conn, err := s.getConnection(serviceName)
	if err != nil {
		return nil, err
	}

	client := pb.NewBenchmarkServiceClient(conn)
	req := &pb.Message{
		From:    s.name,
		BenchId: benchID,
		TraceId: traceID,
		Depth:   depth,
	}
	return client.Process(ctx, req)
}

func (s *Service) Process(ctx context.Context, req *pb.Message) (*pb.Message, error) {
	if s.otelTracer != nil {
		var span trace.Span
		ctx, span = s.otelTracer.Start(ctx, "Process",
			trace.WithAttributes(
				semconv.ServiceNameKey.String(s.name),
				attribute.String("trace.id", req.TraceId),
				attribute.String("bench.id", req.BenchId),
				attribute.String("from.service", req.From),
				attribute.Int64("depth", int64(req.Depth)),
			))
		defer span.End()
	}

	s.tracer.LogTrace("process_start", req.BenchId, req.TraceId, req.From)

	if s.processingDelay > 0 && s.otelTracer != nil {
		_, processingSpan := s.otelTracer.Start(ctx, "processing_delay")
		time.Sleep(s.processingDelay)
		processingSpan.End()
	} else {
		time.Sleep(s.processingDelay)
	}

	thisDepth := req.Depth + 1

	if len(s.outServices) == 0 {
		s.tracer.LogTrace("process_end", req.BenchId, req.TraceId, req.From)
		return &pb.Message{
			From:    s.name,
			BenchId: req.BenchId,
			TraceId: req.TraceId,
			Depth:   thisDepth,
		}, nil
	}

	var wg sync.WaitGroup
	responses := make(chan *pb.Message, len(s.outServices))
	errors := make(chan error, len(s.outServices))

	for _, outService := range s.outServices {
		wg.Add(1)
		go func(serviceName string) {
			defer wg.Done()
			s.tracer.LogTrace("req_start", req.BenchId, req.TraceId, serviceName)
			resp, err := s.callService(ctx, serviceName, req.BenchId, req.TraceId, thisDepth)
			if err != nil {
				errors <- err
			} else if resp != nil {
				responses <- resp
				s.tracer.LogTrace("req_resp", resp.BenchId, resp.TraceId, resp.From)
			}
		}(outService)
	}

	wg.Wait()
	close(responses)
	close(errors)

	for err := range errors {
		log.Println(err.Error())
	}

	s.tracer.LogTrace("process_end", req.BenchId, req.TraceId, req.From)

	return &pb.Message{
		BenchId: req.BenchId,
		From:    s.name,
		TraceId: req.TraceId,
		Depth:   0,
	}, nil
}

func (s *Service) handle_http(ctx *fasthttp.RequestCtx) {
	start := time.Now()
	traceId := fmt.Sprintf("%d", start.UnixNano())

	var benchId string
	smethod := string(ctx.Method())
	if smethod == fasthttp.MethodGet {
		benchId = "default"
	} else if smethod == fasthttp.MethodPost {
		benchId = string(ctx.PostBody())
	} else {
		ctx.Error("Unsupported method", fasthttp.StatusMethodNotAllowed)
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if s.otelTracer != nil {
		var span trace.Span
		reqCtx, span = s.otelTracer.Start(reqCtx, "http_request",
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(smethod),
				semconv.URLFullKey.String(string(ctx.URI().Path())),
				semconv.URLPathKey.String(string(ctx.URI().Path())),
				attribute.String("trace.id", traceId),
				attribute.String("bench.id", benchId),
			))
		defer span.End()
	}

	s.tracer.LogTrace("http_start", benchId, traceId, "")

	req := &pb.Message{
		BenchId: benchId,
		From:    s.name,
		TraceId: traceId,
		Depth:   0,
	}
	_, err := s.Process(reqCtx, req)
	if err != nil {
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		return
	}
	s.tracer.LogTrace("http_end", benchId, traceId, "")

	ctx.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)

	response := map[string]interface{}{
		"traceId":     traceId,
		"serviceTime": time.Since(start).Nanoseconds(),
		"benchId":     benchId,
		"topology":    s.topology,
		"cni":         s.cni,
	}

	if err := json.NewEncoder(ctx).Encode(response); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		return
	}
}

func (s *Service) getConnection(serviceName string) (*grpc.ClientConn, error) {
	s.connPoolMu.RLock()
	conn, exists := s.connPool[serviceName]
	s.connPoolMu.RUnlock()

	if exists {
		return conn, nil
	}

	s.connPoolMu.Lock()
	defer s.connPoolMu.Unlock()

	// Check again in case another goroutine created the connection
	if conn, exists := s.connPool[serviceName]; exists {
		return conn, nil
	}

	kacp := keepalive.ClientParameters{
		Time:                5 * time.Minute, // send pings every 5 minutes
		Timeout:             time.Second,     // wait 1 second for ping ack before considering the connection dead
		PermitWithoutStream: true,            // send pings even without active streams
	}

	conn, err := grpc.NewClient(
		fmt.Sprintf("%s:%d", serviceName, 50051),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(kacp),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", serviceName, err)
	}

	s.connPool[serviceName] = conn
	return conn, nil
}

func (s *Service) HandleHTTP(ctx *fasthttp.RequestCtx) {
	s.handle_http(ctx)
}
