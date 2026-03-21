package project

type EventsConfig struct {
	Enabled           bool           `json:"enabled"`
	Dir               string         `json:"dir"`
	Transport         EventTransport `json:"transport"`
	AllEntities       bool           `json:"all_entities"`
	EntityIdentifiers []string       `json:"entity_identifiers"`
}

type EventTransport string

const (
	InvalidEventTransport EventTransport = "invalid"
	KafkaEventTransport   EventTransport = "kafka"
)

func (p Project) KafkaEnabled() bool {
	if !p.CoreConfig.EventsConfig.Enabled {
		return false
	}

	if p.CoreConfig.EventsConfig.Transport == KafkaEventTransport {
		return true
	}

	return false
}
