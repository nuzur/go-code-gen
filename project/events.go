package project

type Events struct {
	Enabled           bool            `json:"enabled"`
	Dir               string          `json:"dir"`
	Transport         EventTransport  `json:"transport"`
	TransportConfig   TransportConfig `json:"transportconfig"`
	AllEntities       bool            `json:"allentities"`
	EntityIdentifiers []string        `json:"entityidentifiers"`
}

type EventTransport string

const (
	InvalidEventTransport EventTransport = "invalid"
	KafkaEventTransport   EventTransport = "kafka"
)

type TransportConfig struct {
	Kafka *KafkaTransportConfig `json:"kafka"`
}

type KafkaTransportConfig struct {
	Version string   `json:"version"`
	Brokers []string `json:"brokers"`
	Topics  []string `json:"topics"`
}

func (p Project) KafkaEnabled() bool {
	if !p.Core.Events.Enabled {
		return false
	}

	if p.Core.Events.Transport == KafkaEventTransport && p.Core.Events.TransportConfig.Kafka != nil {
		return true
	}

	return false
}

func (p Project) KafkaConfig() *KafkaTransportConfig {
	if p.KafkaEnabled() {
		return p.Core.Events.TransportConfig.Kafka
	}
	return nil
}
