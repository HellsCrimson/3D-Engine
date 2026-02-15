build:
	go build -o 3DEngineGUI *.go

run:
	go run *.go

build-rpc:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		grpc/engine.proto

clean:
	- ${RM} 3DEngineGUI
