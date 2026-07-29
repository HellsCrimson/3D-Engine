package engine

import (
	"fmt"
	"io"
	"net"

	egrpc "3d-engine/grpc"
	"3d-engine/object"
	"3d-engine/utils"

	"github.com/go-gl/mathgl/mgl32"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type engineServer struct {
	egrpc.UnimplementedEngineServer
	app *App
}

// startRPCServer binds the listener synchronously so a port conflict surfaces
// as an error from New, then serves on its own goroutine. Handlers never touch
// GL: they either mutate transforms under the model lock or queue a scene
// change for the frame loop.
func (a *App) startRPCServer(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	server := grpc.NewServer()
	egrpc.RegisterEngineServer(server, &engineServer{app: a})
	a.rpc = server

	go func() {
		if err := server.Serve(lis); err != nil {
			utils.Logger().Println("rpc server stopped:", err)
		}
	}()

	return nil
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

	eg.app.Models(func(models []*object.Model) {
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
	})

	return objects
}

// mutate applies fn to the addressed model, translating a miss into a NotFound.
func (eg *engineServer) mutate(id uint32, fn func(model *object.Model)) error {
	if eg.app.MutateModel(id, fn) {
		return nil
	}
	return status.Errorf(codes.NotFound, "object %d not found", id)
}

func (eg *engineServer) moveObject(obj *egrpc.Object) error {
	if obj == nil || obj.Location == nil || obj.Location.Position == nil {
		return status.Error(codes.InvalidArgument, "object.location.position is required")
	}

	return eg.mutate(obj.Id, func(model *object.Model) {
		model.Coordinates = toVec3(obj.Location.Position)
	})
}

func (eg *engineServer) rotateObject(obj *egrpc.Object) error {
	if obj == nil || obj.Location == nil || obj.Location.Rotation == nil {
		return status.Error(codes.InvalidArgument, "object.location.rotation is required")
	}

	return eg.mutate(obj.Id, func(model *object.Model) {
		model.Rotation = toVec4(obj.Location.Rotation)
	})
}

func (eg *engineServer) scaleObject(obj *egrpc.Object) error {
	if obj == nil || obj.Location == nil || obj.Location.Scale == nil {
		return status.Error(codes.InvalidArgument, "object.location.scale is required")
	}

	return eg.mutate(obj.Id, func(model *object.Model) {
		model.Scale = toVec3(obj.Location.Scale)
	})
}

func (eg *engineServer) updateObject(obj *egrpc.Object) error {
	if obj == nil || obj.Location == nil || obj.Location.Position == nil || obj.Location.Rotation == nil || obj.Location.Scale == nil {
		return status.Error(codes.InvalidArgument, "object.location with position/rotation/scale is required")
	}

	return eg.mutate(obj.Id, func(model *object.Model) {
		model.Coordinates = toVec3(obj.Location.Position)
		model.Rotation = toVec4(obj.Location.Rotation)
		model.Scale = toVec3(obj.Location.Scale)
	})
}

func toVec3(v *egrpc.Vector3) mgl32.Vec3 {
	return mgl32.Vec3{v.X, v.Y, v.Z}
}

func toVec4(v *egrpc.Vector4) mgl32.Vec4 {
	return mgl32.Vec4{v.X, v.Y, v.Z, v.W}
}

func (eg *engineServer) loadScene(sceneRef *egrpc.SceneRef) error {
	if sceneRef == nil || sceneRef.GetPath() == "" {
		return status.Error(codes.InvalidArgument, "scene.path is required")
	}

	eg.app.Scenes.RequestSceneChange(sceneRef.GetPath())
	return nil
}

func (eg *engineServer) getSceneModes() *egrpc.SceneModes {
	modeNames := eg.app.Scenes.ListModeNames()
	modesMap := eg.app.Scenes.ListModes()

	result := &egrpc.SceneModes{
		Modes:            make([]*egrpc.SceneMode, 0, len(modeNames)),
		CurrentMode:      eg.app.Scenes.CurrentSceneMode(),
		CurrentScenePath: eg.app.Scenes.CurrentScenePath(),
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

	eg.app.Scenes.RequestSceneModeChange(sceneModeRef.GetMode())
	return nil
}
