package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	pb "github.com/KillianCdP/MicroBench/pkg/proto"
	"google.golang.org/grpc"

	"github.com/KillianCdP/MicroBench/pkg/service"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/pprofhandler"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func initTracer(serviceName string) (*tracesdk.TracerProvider, error) {
	ctx := context.Background()

	exp, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironment(os.Getenv("DEPLOYMENT_ENV")),
		),
		resource.WithProcessRuntimeDescription(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp,
			tracesdk.WithBatchTimeout(5*time.Second),
			tracesdk.WithMaxExportBatchSize(512),
			tracesdk.WithMaxQueueSize(2048),
		),
		tracesdk.WithResource(res),
		tracesdk.WithSampler(tracesdk.ParentBased(
			tracesdk.TraceIDRatioBased(0.1), // Sample 10% of traces, could be set as an option?
		)),
	)

	otel.SetTracerProvider(tp)
	return tp, nil
}

func getLogLevelFromEnv() slog.Level {
	levelStr := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	switch levelStr {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func main() {
	name := flag.String("name", "", "Service name")
	outServices := flag.String("out", "", "Comma-separated list of service")
	processingDelay := flag.Duration("delay", 0, "Processing delay")
	rps := flag.Int("rps", 0, "Target requests per second")
	listenPort := flag.String("port", "50051", "gRPC port to listen to")
	frontend := flag.Bool("is-frontend", false, "whether the service is the frontend")
	pprof := flag.Bool("pprof", false, "Enable pprof server")
	enableTracing := flag.Bool("tracing", false, "Enable OpenTelemetry tracing")
	flag.Parse()

	level := getLogLevelFromEnv()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	topology := os.Getenv("BENCH_NAME")
	if topology == "" {
		topology = "unknown"
	}
	cni := os.Getenv("CNI")
	if cni == "" {
		cni = "unknown"
	}

	slog.SetDefault(logger)

	if *name == "" {
		log.Fatal("Service name is required")
	}

	var tp *tracesdk.TracerProvider
	if *enableTracing {
		var err error
		tp, err = initTracer(*name)
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			if err := tp.Shutdown(context.Background()); err != nil {
				log.Printf("Error shutting down tracer provider: %v", err)
			}
		}()
		slog.Info("Tracing enabled")
	} else {
		slog.Info("Tracing disabled")
	}

	processedOutServices := []string{}
	if split := strings.Split(*outServices, ","); len(split) > 1 {
		processedOutServices = split
	}

	config := service.ServiceConfig{
		Name:            *name,
		OutServices:     processedOutServices,
		ProcessingDelay: *processingDelay,
		RPS:             *rps,
		Topology:        topology,
		CNI:             cni,
		Logger:          logger,
		TracerProvider:  tp,
	}

	svc := service.NewService(config)
	svc.Preconnect()

	if *pprof {
		go func() {
			pprofRouter := router.New()
			pprofRouter.GET("/debug/pprof/{profile?}", pprofhandler.PprofHandler)

			log.Println("Starting pprof server on :6060")
			if err := fasthttp.ListenAndServe(":6060", pprofRouter.Handler); err != nil {
				log.Fatalf("Failed to start pprof server: %v", err)
			}
		}()
	}

	if *frontend {
		go func() {
			handler := func(ctx *fasthttp.RequestCtx) {
				switch string(ctx.Path()) {
				case "/":
					svc.HandleHTTP(ctx)
				default:
					ctx.Error("Not found", fasthttp.StatusNotFound)
				}
			}

			slog.Info("Serving frontend on :8000")
			if err := fasthttp.ListenAndServe(":8000", handler); err != nil {
				log.Fatalf("Failed to serve frontend: %v", err)
			}
		}()
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%s", "0.0.0.0", *listenPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	var grpcServer *grpc.Server
	if *enableTracing {
		grpcServer = grpc.NewServer(
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
		)
	} else {
		grpcServer = grpc.NewServer()
	}
	pb.RegisterBenchmarkServiceServer(grpcServer, svc)

	log.Printf("Starting service %s, out services: %v, processing delay: %v, target RPS: %d\n",
		*name, *outServices, *processingDelay, *rps)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
