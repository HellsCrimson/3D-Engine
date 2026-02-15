package main

import (
	"fmt"
	"io"
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

func (eg *engineServer) Stream(stream egrpc.Engine_StreamServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		resp := eg.handleRequest(req)
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func (eg *engineServer) handleRequest(req *egrpc.EngineRequest) *egrpc.EngineResponse {
	if req == nil {
		return errorResponse(egrpc.Operation_OPERATION_UNSPECIFIED, status.Error(codes.InvalidArgument, "request is required"))
	}

	switch req.GetOperation() {
	case egrpc.Operation_OPERATION_GET_OBJECTS:
		objects := eg.getObjects()
		return &egrpc.EngineResponse{
			Operation: egrpc.Operation_OPERATION_GET_OBJECTS,
			Success:   true,
			Body: &egrpc.EngineResponse_Objects{
				Objects: objects,
			},
		}
	case egrpc.Operation_OPERATION_ADD_OBJECT:
		return errorResponse(egrpc.Operation_OPERATION_ADD_OBJECT, status.Error(codes.Unimplemented, "add object is not implemented"))
	case egrpc.Operation_OPERATION_REMOVE_OBJECT:
		return errorResponse(egrpc.Operation_OPERATION_REMOVE_OBJECT, status.Error(codes.Unimplemented, "remove object is not implemented"))
	case egrpc.Operation_OPERATION_MOVE_OBJECT:
		if err := eg.moveObject(req.GetObject()); err != nil {
			return errorResponse(egrpc.Operation_OPERATION_MOVE_OBJECT, err)
		}
		return emptySuccessResponse(egrpc.Operation_OPERATION_MOVE_OBJECT)
	case egrpc.Operation_OPERATION_ROTATE_OBJECT:
		if err := eg.rotateObject(req.GetObject()); err != nil {
			return errorResponse(egrpc.Operation_OPERATION_ROTATE_OBJECT, err)
		}
		return emptySuccessResponse(egrpc.Operation_OPERATION_ROTATE_OBJECT)
	case egrpc.Operation_OPERATION_SCALE_OBJECT:
		if err := eg.scaleObject(req.GetObject()); err != nil {
			return errorResponse(egrpc.Operation_OPERATION_SCALE_OBJECT, err)
		}
		return emptySuccessResponse(egrpc.Operation_OPERATION_SCALE_OBJECT)
	case egrpc.Operation_OPERATION_UPDATE_OBJECT:
		if err := eg.updateObject(req.GetObject()); err != nil {
			return errorResponse(egrpc.Operation_OPERATION_UPDATE_OBJECT, err)
		}
		return emptySuccessResponse(egrpc.Operation_OPERATION_UPDATE_OBJECT)
	case egrpc.Operation_OPERATION_LOAD_SCENE:
		if err := eg.loadScene(req.GetScene()); err != nil {
			return errorResponse(egrpc.Operation_OPERATION_LOAD_SCENE, err)
		}
		return emptySuccessResponse(egrpc.Operation_OPERATION_LOAD_SCENE)
	case egrpc.Operation_OPERATION_GET_SCENE_MODES:
		return &egrpc.EngineResponse{
			Operation: egrpc.Operation_OPERATION_GET_SCENE_MODES,
			Success:   true,
			Body: &egrpc.EngineResponse_SceneModes{
				SceneModes: eg.getSceneModes(),
			},
		}
	case egrpc.Operation_OPERATION_LOAD_SCENE_MODE:
		if err := eg.loadSceneMode(req.GetSceneMode()); err != nil {
			return errorResponse(egrpc.Operation_OPERATION_LOAD_SCENE_MODE, err)
		}
		return emptySuccessResponse(egrpc.Operation_OPERATION_LOAD_SCENE_MODE)
	default:
		return errorResponse(req.GetOperation(), status.Error(codes.InvalidArgument, "unsupported operation"))
	}
}

func emptySuccessResponse(op egrpc.Operation) *egrpc.EngineResponse {
	return &egrpc.EngineResponse{
		Operation: op,
		Success:   true,
		Body: &egrpc.EngineResponse_Empty{
			Empty: &emptypb.Empty{},
		},
	}
}

func errorResponse(op egrpc.Operation, err error) *egrpc.EngineResponse {
	return &egrpc.EngineResponse{
		Operation: op,
		Success:   false,
		Error:     err.Error(),
	}
}

func (eg *engineServer) getObjects() *egrpc.Objects {
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

	return objects
}

func (eg *engineServer) moveObject(object *egrpc.Object) error {
	if object == nil || object.Location == nil || object.Location.Position == nil {
		return status.Error(codes.InvalidArgument, "object.location.position is required")
	}

	modelsMu.Lock()
	defer modelsMu.Unlock()
	for _, model := range models {
		if model.Id == object.Id {
			model.Coordinates = mgl32.Vec3{object.Location.Position.X, object.Location.Position.Y, object.Location.Position.Z}
			return nil
		}
	}

	return status.Errorf(codes.NotFound, "object %d not found", object.Id)
}

func (eg *engineServer) rotateObject(object *egrpc.Object) error {
	if object == nil || object.Location == nil || object.Location.Rotation == nil {
		return status.Error(codes.InvalidArgument, "object.location.rotation is required")
	}

	modelsMu.Lock()
	defer modelsMu.Unlock()
	for _, model := range models {
		if model.Id == object.Id {
			model.Rotation = mgl32.Vec4{object.Location.Rotation.X, object.Location.Rotation.Y, object.Location.Rotation.Z, object.Location.Rotation.W}
			return nil
		}
	}
	return status.Errorf(codes.NotFound, "object %d not found", object.Id)
}

func (eg *engineServer) scaleObject(object *egrpc.Object) error {
	if object == nil || object.Location == nil || object.Location.Scale == nil {
		return status.Error(codes.InvalidArgument, "object.location.scale is required")
	}

	modelsMu.Lock()
	defer modelsMu.Unlock()
	for _, model := range models {
		if model.Id == object.Id {
			model.Scale = mgl32.Vec3{object.Location.Scale.X, object.Location.Scale.Y, object.Location.Scale.Z}
			return nil
		}
	}
	return status.Errorf(codes.NotFound, "object %d not found", object.Id)
}

func (eg *engineServer) updateObject(object *egrpc.Object) error {
	if object == nil || object.Location == nil || object.Location.Position == nil || object.Location.Rotation == nil || object.Location.Scale == nil {
		return status.Error(codes.InvalidArgument, "object.location with position/rotation/scale is required")
	}

	modelsMu.Lock()
	defer modelsMu.Unlock()
	for _, model := range models {
		if model.Id == object.Id {
			model.Coordinates = mgl32.Vec3{object.Location.Position.X, object.Location.Position.Y, object.Location.Position.Z}
			model.Rotation = mgl32.Vec4{object.Location.Rotation.X, object.Location.Rotation.Y, object.Location.Rotation.Z, object.Location.Rotation.W}
			model.Scale = mgl32.Vec3{object.Location.Scale.X, object.Location.Scale.Y, object.Location.Scale.Z}
			return nil
		}
	}

	return status.Errorf(codes.NotFound, "object %d not found", object.Id)
}

func (eg *engineServer) loadScene(sceneRef *egrpc.SceneRef) error {
	if sceneRef == nil || sceneRef.GetPath() == "" {
		return status.Error(codes.InvalidArgument, "scene.path is required")
	}

	sceneMgr.RequestSceneChange(sceneRef.GetPath())
	return nil
}

func (eg *engineServer) getSceneModes() *egrpc.SceneModes {
	modeNames := sceneMgr.ListModeNames()
	modesMap := sceneMgr.ListModes()

	result := &egrpc.SceneModes{
		Modes:            make([]*egrpc.SceneMode, 0, len(modeNames)),
		CurrentMode:      sceneMgr.CurrentSceneMode(),
		CurrentScenePath: sceneMgr.CurrentScenePath(),
	}

	for _, mode := range modeNames {
		result.Modes = append(result.Modes, &egrpc.SceneMode{
			Name: mode,
			Path: modesMap[mode],
		})
	}
	return result
}

func (eg *engineServer) loadSceneMode(sceneModeRef *egrpc.SceneModeRef) error {
	if sceneModeRef == nil || sceneModeRef.GetMode() == "" {
		return status.Error(codes.InvalidArgument, "scene_mode.mode is required")
	}

	sceneMgr.RequestSceneModeChange(sceneModeRef.GetMode())
	return nil
}
