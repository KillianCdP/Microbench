package tracing

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
