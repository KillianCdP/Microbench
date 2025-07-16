package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

type Topology struct {
	Services map[string]Service `yaml:"services"`
}

type Service struct {
	Node            string   `yaml:"node"`
	Port            int      `yaml:"port"`
	ProcessingDelay string   `yaml:"processing_delay"`
	Replicas        int      `yaml:"replicas"`
	OutServices     []string `yaml:"out_services"`
}

type TemplateData struct {
	Name      string
	Service   Service
	BenchName string
	CNI       string
	LogLevel  string
	Args      []string
}

const manifestTemplate = `
{{- define "statefulset" }}
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ .Name }}
  labels:
    app.kubernetes.io/part-of: microbench
spec:
  serviceName: {{ .Name }}
  replicas: {{ .Service.Replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ .Name }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {{ .Name }}
        app.kubernetes.io/part-of: microbench
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        runAsGroup: 3000  # matches the appgroup GID
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: {{ .Name }}
        image: 134.59.129.87:4430/microbench:v2
        securityContext:
          allowPrivilegeEscalation: false
          runAsNonRoot: true
          runAsUser: 1000
          runAsGroup: 3000
          capabilities:
            drop:
            - ALL
        env:
        - name: BENCH_NAME
          value: {{ .BenchName }}
        - name: CNI
          value: {{ .CNI }}
        - name: LOG_LEVEL
          value: {{ .LogLevel }}
        args:
        {{- range .Args }}
        - "{{ . }}"
        {{- end }}
        ports:
        - containerPort: {{ .Service.Port }}
        - containerPort: 8000
        - containerPort: 8080
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: kubernetes.io/hostname
                operator: In
                values:
                - {{ .Service.Node }}
{{ end }}

{{- define "service" }}
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}
  labels:
    app.kubernetes.io/part-of: microbench
spec:
  selector:
    app.kubernetes.io/name: {{ .Name }}
  ports:
  - port: {{ .Service.Port }}
    targetPort: {{ .Service.Port }}
{{ end }}

{{- define "external-service" }}
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .Name }}-external
  labels:
    app.kubernetes.io/part-of: microbench
spec:
  selector:
    app.kubernetes.io/name: {{ .Name }}
  type: LoadBalancer
  ports:
  - port: 8000
    targetPort: 8000
    name: http
{{ end }}
`

func generateManifests(file, cniName, logLevel string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	var topo Topology
	err = yaml.Unmarshal(data, &topo)
	if err != nil {
		return err
	}

	benchName := strings.Split(strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)), "/")[0]

	tmpl, err := template.New("manifest").Parse(manifestTemplate)
	if err != nil {
		return err
	}

	for name, service := range topo.Services {
		data := TemplateData{
			Name:      name,
			Service:   service,
			BenchName: benchName,
			CNI:       cniName,
			LogLevel:  logLevel,
			Args: []string{
				fmt.Sprintf("--name=%s", name),
				fmt.Sprintf("--out=%s", strings.Join(service.OutServices, ",")),
				fmt.Sprintf("--delay=%s", service.ProcessingDelay),
				fmt.Sprintf("--port=%d", service.Port),
			},
		}

		if name == "frontend" {
			data.Args = append(data.Args, "--is-frontend")
		}

		if err := tmpl.ExecuteTemplate(os.Stdout, "statefulset", data); err != nil {
			return err
		}
		fmt.Println("---")

		if err := tmpl.ExecuteTemplate(os.Stdout, "service", data); err != nil {
			return err
		}
		fmt.Println("---")

		if name == "frontend" {
			if err := tmpl.ExecuteTemplate(os.Stdout, "external-service", data); err != nil {
				return err
			}
			fmt.Println("---")
		}
	}

	return nil
}

func main() {
	topologyFile := flag.String("topology", "", "Path to the topology file")
	cni := flag.String("cni", "", "CNI name")
	logLevel := flag.String("log-level", "info", "Log level")
	flag.Parse()

	if *topologyFile == "" || *cni == "" {
		log.Fatal("Usage: generate -topology <topology_file> -cni <cni_name> [-log-level <log_level>]")
	}

	err := generateManifests(*topologyFile, *cni, *logLevel)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}
