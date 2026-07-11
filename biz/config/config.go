package config

// Content-Type MIME of the most common data formats.
const (
	MIMEJSON     = "application/json"
	MIMEPROTOBUF = "application/x-protobuf"
)

// serizlizer method
const (
	SERIZLIZERPB   = "pb"
	SERIZLIZERJSON = "json"
)

var ApiSerializerMap = map[string]string{}
