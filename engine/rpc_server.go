package engine

import (
	"fmt"
	"io"
	"net"

	egrpc "3d-engine/grpc"
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
		created, err := eg.addObject(req.GetObject())
		if err != nil {
			return errorResponse(egrpc.Operation_OPERATION_ADD_OBJECT, err)
		}
		return &egrpc.EngineResponse{
			Operation: egrpc.Operation_OPERATION_ADD_OBJECT,
			Success:   true,
			Body:      &egrpc.EngineResponse_Object{Object: created},
		}
	case egrpc.Operation_OPERATION_REMOVE_OBJECT:
		if err := eg.removeObject(req.GetObject()); err != nil {
			return errorResponse(egrpc.Operation_OPERATION_REMOVE_OBJECT, err)
		}
		return emptySuccessResponse(egrpc.Operation_OPERATION_REMOVE_OBJECT)
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
	case egrpc.Operation_OPERATION_REMOVE_TREE:
		if err := eg.removeTree(req.GetObject()); err != nil {
			return errorResponse(egrpc.Operation_OPERATION_REMOVE_TREE, err)
		}
		return emptySuccessResponse(egrpc.Operation_OPERATION_REMOVE_TREE)
	case egrpc.Operation_OPERATION_SET_PARENT:
		if err := eg.setParent(req.GetObject()); err != nil {
			return errorResponse(egrpc.Operation_OPERATION_SET_PARENT, err)
		}
		return emptySuccessResponse(egrpc.Operation_OPERATION_SET_PARENT)
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
	infos := eg.app.ListObjects()

	objects := &egrpc.Objects{Objects: make([]*egrpc.Object, 0, len(infos))}
	for _, info := range infos {
		objects.Objects = append(objects.Objects, toProtoObject(info))
	}
	return objects
}

// toProtoObject is the only place engine types become wire types.
func toProtoObject(info ObjectInfo) *egrpc.Object {
	// The wire still speaks axis-angle, so the quaternion is converted back here
	// rather than the proto growing a fourth rotation component with a different
	// meaning. Existing clients see exactly what they saw before.
	rotation := AxisAngleFromQuat(info.Transform.Rotation)

	return &egrpc.Object{
		Id: info.Handle.Encode(),
		// Zero for a root entity, which is what NoHandle encodes to, so a client
		// can rebuild the tree from the flat list.
		ParentId: info.Parent.Encode(),
		Name:     info.Name,
		Model:    info.Model,
		Location: &egrpc.Location{
			Position: &egrpc.Vector3{
				X: info.Transform.Position.X(),
				Y: info.Transform.Position.Y(),
				Z: info.Transform.Position.Z(),
			},
			Rotation: &egrpc.Vector4{
				X: rotation.X(),
				Y: rotation.Y(),
				Z: rotation.Z(),
				W: rotation.W(),
			},
			Scale: &egrpc.Vector3{
				X: info.Transform.Scale.X(),
				Y: info.Transform.Scale.Y(),
				Z: info.Transform.Scale.Z(),
			},
		},
	}
}

// addObject spawns through the same API a scene file and the editor use. The
// spawn is queued onto the frame loop because importing a model touches the
// GPU, so this blocks until the next frame picks it up.
func (eg *engineServer) addObject(obj *egrpc.Object) (*egrpc.Object, error) {
	if obj == nil {
		return nil, status.Error(codes.InvalidArgument, "object is required")
	}

	spec := ObjectSpec{
		Name:      obj.GetName(),
		Model:     obj.GetModel(),
		Transform: IdentityTransform(),
	}
	if obj.Location != nil {
		spec.Transform = transformFromProto(obj.Location, spec.Transform)
	}
	// Zero decodes to NoHandle, which SpawnObject reads as "no parent", so a
	// client that never sets the field behaves exactly as before.
	spec.Parent = DecodeHandle(obj.GetParentId())

	handle, err := eg.app.Spawn(spec)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	info, ok := eg.app.ObjectInfo(handle)
	if !ok {
		return nil, status.Errorf(codes.Internal, "object %s vanished immediately after spawn", handle)
	}
	return toProtoObject(info), nil
}

func (eg *engineServer) removeObject(obj *egrpc.Object) error {
	if obj == nil {
		return status.Error(codes.InvalidArgument, "object is required")
	}

	handle := DecodeHandle(obj.GetId())
	if err := eg.app.Despawn(handle); err != nil {
		return status.Errorf(codes.NotFound, "%v", err)
	}
	return nil
}

// removeTree is REMOVE_OBJECT's cascading twin, mirroring the engine's
// DespawnObject/DespawnTree pair: the former lifts the children to the scene
// root, this one takes them with it.
func (eg *engineServer) removeTree(obj *egrpc.Object) error {
	if obj == nil {
		return status.Error(codes.InvalidArgument, "object is required")
	}

	handle := DecodeHandle(obj.GetId())
	if err := eg.app.DespawnSubtree(handle); err != nil {
		return status.Errorf(codes.NotFound, "%v", err)
	}
	return nil
}

// setParent reparents an existing object. A parent_id of zero decodes to
// NoHandle and detaches the object to the scene root.
func (eg *engineServer) setParent(obj *egrpc.Object) error {
	if obj == nil {
		return status.Error(codes.InvalidArgument, "object is required")
	}

	child := DecodeHandle(obj.GetId())
	parent := DecodeHandle(obj.GetParentId())

	if err := eg.app.SetParent(child, parent); err != nil {
		// A cycle is a bad request, not a missing object: both handles resolved.
		return status.Errorf(codes.InvalidArgument, "%v", err)
	}
	return nil
}

// transformFromProto fills in only the components the request supplied, leaving
// the rest at the fallback.
func transformFromProto(location *egrpc.Location, fallback Transform) Transform {
	transform := fallback
	if location.Position != nil {
		transform.Position = toVec3(location.Position)
	}
	if location.Rotation != nil {
		transform.Rotation = QuatFromAxisAngle(toVec4(location.Rotation))
	}
	if location.Scale != nil {
		transform.Scale = toVec3(location.Scale)
	}
	return transform
}

func (eg *engineServer) moveObject(obj *egrpc.Object) error {
	if obj == nil || obj.Location == nil || obj.Location.Position == nil {
		return status.Error(codes.InvalidArgument, "object.location.position is required")
	}

	return eg.update(obj.Id, func(t *Transform) {
		t.Position = toVec3(obj.Location.Position)
	})
}

func (eg *engineServer) rotateObject(obj *egrpc.Object) error {
	if obj == nil || obj.Location == nil || obj.Location.Rotation == nil {
		return status.Error(codes.InvalidArgument, "object.location.rotation is required")
	}

	return eg.update(obj.Id, func(t *Transform) {
		t.Rotation = QuatFromAxisAngle(toVec4(obj.Location.Rotation))
	})
}

func (eg *engineServer) scaleObject(obj *egrpc.Object) error {
	if obj == nil || obj.Location == nil || obj.Location.Scale == nil {
		return status.Error(codes.InvalidArgument, "object.location.scale is required")
	}

	return eg.update(obj.Id, func(t *Transform) {
		t.Scale = toVec3(obj.Location.Scale)
	})
}

func (eg *engineServer) updateObject(obj *egrpc.Object) error {
	if obj == nil || obj.Location == nil || obj.Location.Position == nil || obj.Location.Rotation == nil || obj.Location.Scale == nil {
		return status.Error(codes.InvalidArgument, "object.location with position/rotation/scale is required")
	}

	return eg.update(obj.Id, func(t *Transform) {
		*t = transformFromProto(obj.Location, *t)
	})
}

// update translates a wire id into a handle and a miss into a NotFound. A
// handle that no longer resolves — because the entity was despawned or the
// scene reloaded — fails rather than writing to whatever now occupies the slot.
func (eg *engineServer) update(id uint64, fn func(t *Transform)) error {
	handle := DecodeHandle(id)
	if err := eg.app.UpdateTransform(handle, fn); err != nil {
		return status.Errorf(codes.NotFound, "%v", err)
	}
	return nil
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

	if err := eg.app.Scenes.RequestSceneModeChange(sceneModeRef.GetMode()); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	return nil
}
