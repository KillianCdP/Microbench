package topology

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Service struct {
	Name            string   `yaml:"-"`
	Node            string   `yaml:"node"`
	Replicas        int      `yaml:"replicas"`
	Port            int      `yaml:"port"`
	ProcessingDelay string   `yaml:"processing_delay"`
	OutServices     []string `yaml:"out_services"`
}

type Topology struct {
	Services map[string]Service `yaml:"services"`
}

func ReadFile(filename string) (*Topology, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var topology Topology
	err = yaml.Unmarshal(data, &topology)
	if err != nil {
		return nil, err
	}

	for name, service := range topology.Services {
		service.Name = name
		topology.Services[name] = service
	}

	return &topology, nil
}
