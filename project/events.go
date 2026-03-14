package project

type EventsConfig struct {
	Enabled           bool            `json:"enabled"`
	Dir               string          `json:"dir"`
	Transport         EventTransport  `json:"transport"`
	TransportConfig   TransportConfig `json:"transport_config"`
	AllEntities       bool            `json:"all_entities"`
	EntityIdentifiers []string        `json:"entity_identifiers"`
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
	if !p.CoreConfig.EventsConfig.Enabled {
		return false
	}

	if p.CoreConfig.EventsConfig.Transport == KafkaEventTransport && p.CoreConfig.EventsConfig.TransportConfig.Kafka != nil {
		return true
	}

	return false
}

func (p Project) KafkaConfig() *KafkaTransportConfig {
	if p.KafkaEnabled() {
		return p.CoreConfig.EventsConfig.TransportConfig.Kafka
	}
	return nil
}
