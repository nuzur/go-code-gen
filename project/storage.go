package project

// StorageConfig controls the optional file-storage zone backed by S3-compatible
// object storage (AWS S3, Cloudflare R2): when enabled, the generated app
// exposes two generic, non-entity-scoped HTTP endpoints — POST /upload
// (multipart -> object store, returns {url,key}) and POST /sign (presigned URL
// for a key) — plus a small self-contained S3 client. File fields stay plain
// string columns; a caller uploads first and then puts the returned URL into
// normal create/update payloads.
//
// It is OFF by default. Generation only needs the Enabled flag — the
// credentials (region/bucket/key/secret) and the optional endpoint that selects
// a non-AWS S3-compatible store are runtime config read from the app's `aws:`
// config block, injected at deploy time from the team's ObjectStore. The
// endpoints are served over HTTP regardless of the API-surface choice
// (REST/gRPC/both): mounted on the default mux, forwarded by the REST router
// when REST is enabled, and served by the gRPC-only httpServer otherwise.
type StorageConfig struct {
	Enabled bool
}
