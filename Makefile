build:
	go build -o 3DEngine .

run:
	go run .

# The editor is drawn inside the engine window now, so `make run` is the whole
# app. This target additionally starts the standalone gRPC client in ui/, which
# is only needed to exercise the RPC surface from another process.
run-with-external-ui:
	go run . & \
	cd ui && go run . & \
	wait

build-rpc:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		grpc/engine.proto

clean:
	- ${RM} 3DEngine
