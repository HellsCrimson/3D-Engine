package main

import (
	context "context"
	"fmt"
	"log"
	"net"

	egrpc "3d-engine/grpc"

	"github.com/go-gl/mathgl/mgl32"
	"google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type engineServer struct {
	egrpc.UnimplementedEngineServer
}

func StartRPCServer() {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", 8080))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	s := engineServer{}
	egrpc.RegisterEngineServer(grpcServer, &s)
	grpcServer.Serve(lis)
}

func (eg *engineServer) GetObjects(_ context.Context, _ *emptypb.Empty) (*egrpc.Objects, error) {
	objects := &egrpc.Objects{
		Objects: []*egrpc.Object{},
	}

	for _, model := range models {
		objects.Objects = append(objects.Objects, &egrpc.Object{
			Id:   model.Id,
			Name: model.Name,
			Location: &egrpc.Location{
				Position: &egrpc.Vector3{
					X: model.Coordinates.X(),
					Y: model.Coordinates.Y(),
					Z: model.Coordinates.Z(),
				},
				Rotation: &egrpc.Vector4{
					X: model.Rotation.X(),
					Y: model.Rotation.Y(),
					Z: model.Rotation.Z(),
					W: model.Rotation.W(),
				},
				Scale: &egrpc.Vector3{
					X: model.Scale.X(),
					Y: model.Scale.Y(),
					Z: model.Scale.Z(),
				},
			},
		})

	}

	return objects, nil
}

func (eg *engineServer) MoveObject(_ context.Context, object *egrpc.Object) (*emptypb.Empty, error) {
	for _, model := range models {
		if model.Id == object.Id {
			model.Coordinates = mgl32.Vec3{object.Location.Position.X, object.Location.Position.Y, object.Location.Position.Z}
			break
		}
	}

	return &emptypb.Empty{}, nil
}

func (eg *engineServer) RotateObject(_ context.Context, object *egrpc.Object) (*emptypb.Empty, error) {
	for _, model := range models {
		if model.Id == object.Id {
			model.Rotation = mgl32.Vec4{object.Location.Rotation.X, object.Location.Rotation.Y, object.Location.Rotation.Z, object.Location.Rotation.W}
			break
		}
	}
	return &emptypb.Empty{}, nil
}

func (eg *engineServer) ScaleObject(_ context.Context, object *egrpc.Object) (*emptypb.Empty, error) {
	for _, model := range models {
		if model.Id == object.Id {
			model.Scale = mgl32.Vec3{object.Location.Scale.X, object.Location.Scale.Y, object.Location.Scale.Z}
			break
		}
	}
	return &emptypb.Empty{}, nil
}

func (eg *engineServer) UpdateObject(_ context.Context, object *egrpc.Object) (*emptypb.Empty, error) {
	for _, model := range models {
		if model.Id == object.Id {
			model.Coordinates = mgl32.Vec3{object.Location.Position.X, object.Location.Position.Y, object.Location.Position.Z}
			model.Rotation = mgl32.Vec4{object.Location.Rotation.X, object.Location.Rotation.Y, object.Location.Rotation.Z, object.Location.Rotation.W}
			model.Scale = mgl32.Vec3{object.Location.Scale.X, object.Location.Scale.Y, object.Location.Scale.Z}
			break
		}
	}

	return &emptypb.Empty{}, nil
}
