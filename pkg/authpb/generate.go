package authpb

// Regenerate stubs from repo root (protoc-gen-go and protoc-gen-go-grpc on PATH):
//
//	protoc -I proto --go_out=pkg/authpb --go_opt=paths=source_relative \
//	  --go-grpc_out=pkg/authpb --go-grpc_opt=paths=source_relative proto/auth.proto
//
//go:generate protoc -I ../../proto --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative auth.proto
