package main

import (
	context "context"
	"fmt"
	"log"
	"net"

	egrpc "3d-engine/grpc"

	"github.com/go-gl/mathgl/mgl32"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	modelsMu.RLock()
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
	modelsMu.RUnlock()

	return objects, nil
}

func (eg *engineServer) MoveObject(_ context.Context, object *egrpc.Object) (*emptypb.Empty, error) {
	if object == nil || object.Location == nil || object.Location.Position == nil {
		return nil, status.Error(codes.InvalidArgument, "object.location.position is required")
	}

	modelsMu.Lock()
	defer modelsMu.Unlock()
	for _, model := range models {
		if model.Id == object.Id {
			model.Coordinates = mgl32.Vec3{object.Location.Position.X, object.Location.Position.Y, object.Location.Position.Z}
			return &emptypb.Empty{}, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "object %d not found", object.Id)
}

func (eg *engineServer) RotateObject(_ context.Context, object *egrpc.Object) (*emptypb.Empty, error) {
	if object == nil || object.Location == nil || object.Location.Rotation == nil {
		return nil, status.Error(codes.InvalidArgument, "object.location.rotation is required")
	}

	modelsMu.Lock()
	defer modelsMu.Unlock()
	for _, model := range models {
		if model.Id == object.Id {
			model.Rotation = mgl32.Vec4{object.Location.Rotation.X, object.Location.Rotation.Y, object.Location.Rotation.Z, object.Location.Rotation.W}
			return &emptypb.Empty{}, nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "object %d not found", object.Id)
}

func (eg *engineServer) ScaleObject(_ context.Context, object *egrpc.Object) (*emptypb.Empty, error) {
	if object == nil || object.Location == nil || object.Location.Scale == nil {
		return nil, status.Error(codes.InvalidArgument, "object.location.scale is required")
	}

	modelsMu.Lock()
	defer modelsMu.Unlock()
	for _, model := range models {
		if model.Id == object.Id {
			model.Scale = mgl32.Vec3{object.Location.Scale.X, object.Location.Scale.Y, object.Location.Scale.Z}
			return &emptypb.Empty{}, nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "object %d not found", object.Id)
}

func (eg *engineServer) UpdateObject(_ context.Context, object *egrpc.Object) (*emptypb.Empty, error) {
	if object == nil || object.Location == nil || object.Location.Position == nil || object.Location.Rotation == nil || object.Location.Scale == nil {
		return nil, status.Error(codes.InvalidArgument, "object.location with position/rotation/scale is required")
	}

	modelsMu.Lock()
	defer modelsMu.Unlock()
	for _, model := range models {
		if model.Id == object.Id {
			model.Coordinates = mgl32.Vec3{object.Location.Position.X, object.Location.Position.Y, object.Location.Position.Z}
			model.Rotation = mgl32.Vec4{object.Location.Rotation.X, object.Location.Rotation.Y, object.Location.Rotation.Z, object.Location.Rotation.W}
			model.Scale = mgl32.Vec3{object.Location.Scale.X, object.Location.Scale.Y, object.Location.Scale.Z}
			return &emptypb.Empty{}, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "object %d not found", object.Id)
}
