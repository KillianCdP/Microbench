package tracing

import (
    "log/slog"
    "time"
)

type Tracer struct {
    logger    *slog.Logger
    topology  string
    cni       string
    serviceName string
}

func NewTracer(logger *slog.Logger, topology, cni, serviceName string) *Tracer {
    return &Tracer{
        logger:      logger,
        topology:    topology,
        cni:        cni,
        serviceName: serviceName,
    }
}

func (t *Tracer) LogTrace(operation, benchID, traceID, relatedService string) {
    traceLog := TraceLog{
        Topology:       t.topology,
        BenchID:        benchID,
        CNI:            t.cni,
        Operation:      operation,
        Timestamp:      time.Now().UnixNano(),
        TraceID:        traceID,
        ServiceName:    t.serviceName,
        RelatedService: relatedService,
    }

    t.logger.Debug("trace_log",
        slog.Any("trace", traceLog),
    )
}
