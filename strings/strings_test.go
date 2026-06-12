package strings

import "testing"

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"id", "ID"},
		{"uuid", "UUID"},
		{"user_id", "UserId"},
		{"user_uuid", "UserUUID"},
		{"uuid_value", "UUIDValue"},
		{"json_data", "JSONData"},
		{"url_path", "URLPath"},
		{"https_url", "HTTPSURL"},
		{"http_port", "HTTPPort"},
		{"establecimiento", "Establecimiento"},
		{"normal_field", "NormalField"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := ToCamelCase(tt.input)
			if actual != tt.expected {
				t.Errorf("ToCamelCase(%q) = %q; expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ID", "id"},
		{"UUID", "uuid"},
		{"UserID", "user_id"},
		{"UserUUID", "user_uuid"},
		{"UUIDValue", "uuid_value"},
		{"JSONData", "json_data"},
		{"URLPath", "url_path"},
		{"HTTPSURL", "httpsurl"},
		{"HTTPPort", "http_port"},
		{"Establecimiento", "establecimiento"},
		{"NormalField", "normal_field"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := ToSnakeCase(tt.input)
			if actual != tt.expected {
				t.Errorf("ToSnakeCase(%q) = %q; expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestStringContains(t *testing.T) {
	if !StringContains("hello world", "world") {
		t.Error("expected StringContains to find 'world' in 'hello world'")
	}
	if StringContains("hello world", "earth") {
		t.Error("expected StringContains to not find 'earth' in 'hello world'")
	}
}
