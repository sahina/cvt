module github.com/sahina/cvt/examples/go/consumer

go 1.25.0

require github.com/sahina/cvt/sdks/go v0.0.0

require (
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/sahina/cvt/sdks/go => ../../../sdks/go
