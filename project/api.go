package project

type APIConfig struct {
	Domain   string `json:"domain"`
	GRPCPort string `json:"grpcport"`
	HTTPPort string `json:"httpport"`
}
