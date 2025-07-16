# MicroBench

A comprehensive microservice benchmarking framework for evaluating Container Network Interface (CNI) performance in Kubernetes environments. This framework enables the deployment and testing of distributed service topologies try

## Features

- **Realistic Topologies**: Deploy service patterns based on production architectures.
- **Kubernetes-Native**: Full integration with Kubernetes networking and orchestration
- **CNI Performance Testing**: Specialized tooling for comparing networking solutions
- **Observability**: OpenTelemetry based tracing or log based.
- **HTTP and gRPC Support**: Frontend HTTP interface with internal gRPC communication

## Framework Components

### 1. Topology Generator (`scripts/generate-topologies/`)
Python script that generates YAML topology files for comprehensive CNI testing

### 2. Manifest Generator (`src/cmd/generate/`)
Reads YAML topology files and generates Kubernetes deployment manifests

### 3. Microservice Runtime (`src/cmd/service/`)
Lightweight Go services. Communicate with each other using gRPC, tracing can be enabled.

## Quick Start

### Topology Generation and Deployment

```bash
# 1. Generate topology YAML files
cd scripts/generate-topologies/
python3 generate_topologies.py --bulk --output-dir ./topologies --nodes "worker1,worker2" --replicas 4

# 2. Generate Kubernetes manifests from topology YAML
cd ../generate/
./generate -topology=../generate-topologies/topologies/fan_1_3_1_rr.yaml -cni=cilium > fan-deployment.yaml

# 3. Deploy to Kubernetes
kubectl apply -f fan-deployment.yaml

# Generate single topology
python3 generate_topologies.py --depth 4 --services "1,2,2,1" --replicas 2 --nodes "worker1,worker2" --scheduling rr
```

### Manual Service Usage

```bash
# Start a simple service
./microbench -name=service1 -delay=10ms

# Start a frontend service
./microbench -name=frontend -is-frontend=true -out=service1,service2

# Start with OpenTelemetry tracing
./microbench -name=service1 -delay=10ms -tracing=true
```

### Service Configuration

```bash
./microbench [OPTIONS]

Options:
  -name string        Service name (required)
  -out string         Comma-separated list of downstream services
  -delay duration     Processing delay simulation (e.g., 10ms, 100ms)
  -rps int           Target requests per second
  -port string       gRPC port to listen on (default: 50051)
  -is-frontend       Enable HTTP frontend interface
  -tracing           Enable OpenTelemetry tracing
  -pprof            Enable pprof profiling server
```

## Tracing Systems

MicroBench provides two complementary tracing systems that can be used independently or together:

### 1. Log-Based Tracing (`pkg/tracing/`)

A lightweight tracing system that outputs structured logs with timestamps for easy analysis.

**Operations Tracked:**
- `http_start/end` - HTTP request lifecycle
- `process_start/end` - Service processing boundaries
- `req_start/resp` - Downstream service calls

This was used initially to be used to be exported in Quickwit and analyzed later on, but used too much disk space in the end.

Most verbose output can be used with the `DEBUG` log level.

### 2. OpenTelemetry Tracing

Full distributed tracing with spans and exporters for production observability.

**Spans Created:**
- `http_request` - HTTP request handling
- `Process` - Service processing with metadata
- `processing_delay` - Isolated processing time measurement
- `call_service` - Downstream service calls

## Configuration

### Environment Variables

```bash
# Log level configuration
export LOG_LEVEL=DEBUG  # DEBUG, INFO, WARN, ERROR

# OpenTelemetry configuration
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
export OTEL_RESOURCE_ATTRIBUTES=service.version=1.0.0
export DEPLOYMENT_ENV=development

# Benchmark metadata
export BENCH_NAME=topology-test-001
export CNI=cilium
```

### Topology Types

The framework uses YAML files to define service topologies. Here's the structure:

#### Topology File Format

```yaml
# Example: fan-1-3-1.yaml
services:
  frontend:
    node: "worker-1"
    port: 50051
    processing_delay: "10ms"
    replicas: 1
    out_services: ["service-b1", "service-b2", "service-b3"]

  service-b1:
    node: "worker-2"
    port: 50051
    processing_delay: "5ms"
    replicas: 2
    out_services: ["service-c"]

  service-b2:
    node: "worker-3"
    port: 50051
    processing_delay: "5ms"
    replicas: 2
    out_services: ["service-c"]

  service-b3:
    node: "worker-4"
    port: 50051
    processing_delay: "5ms"
    replicas: 2
    out_services: ["service-c"]

  service-c:
    node: "worker-5"
    port: 50051
    processing_delay: "15ms"
    replicas: 1
    out_services: []
```

#### Topology Generation Process

```bash
# 1. Create or use existing topology YAML file
# 2. Generate Kubernetes manifests
./generate -topology=topologies/your-topology.yaml -cni=cilium > deployment.yaml

# 3. Deploy to Kubernetes
kubectl apply -f deployment.yaml

# 4. Test the deployed topology
kubectl get pods -l app.kubernetes.io/part-of=microbench
```

### Manual Service Topology Example

```bash
# Service A (frontend) -> Service B -> Service C
./microbench -name=serviceC -delay=5ms &
./microbench -name=serviceB -out=serviceC -delay=10ms &
./microbench -name=serviceA -is-frontend=true -out=serviceB -delay=15ms &

# Test the topology
curl http://localhost:8000/
```

## Building and Development

```bash
go build -o microbench ./cmd/service/
```

## Contributing

Contributions are welcome! This framework is designed to be extensible and can be adapted for various microservice testing scenarios.
