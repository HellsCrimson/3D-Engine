build:
	go build -o 3DEngine .

run:
	go run .

run-all:
	go run . & \
	cd ui && go run . & \
	wait

build-rpc:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		grpc/engine.proto

clean:
	- ${RM} 3DEngine
