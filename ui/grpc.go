package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "3d-engine-ui/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

func getObjects() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := streamRequest(ctx, &pb.EngineRequest{
		Operation: pb.Operation_OPERATION_GET_OBJECTS,
		Body: &pb.EngineRequest_Empty{
			Empty: &emptypb.Empty{},
		},
	})
	if err != nil {
		log.Fatalf("client get objects failed: %v", err)
	}
	objects = resp.GetObjects().GetObjects()
}

func updateObject(obj *pb.Object) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := streamRequest(ctx, &pb.EngineRequest{
		Operation: pb.Operation_OPERATION_UPDATE_OBJECT,
		Body: &pb.EngineRequest_Object{
			Object: obj,
		},
	})
	if err != nil {
		log.Fatalf("client update object failed: %v", err)
	}
}

func loadScene(scenePath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := streamRequest(ctx, &pb.EngineRequest{
		Operation: pb.Operation_OPERATION_LOAD_SCENE,
		Body: &pb.EngineRequest_Scene{
			Scene: &pb.SceneRef{
				Path: scenePath,
			},
		},
	})
	if err != nil {
		log.Fatalf("client load scene failed: %v", err)
	}
}

func getSceneModes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := streamRequest(ctx, &pb.EngineRequest{
		Operation: pb.Operation_OPERATION_GET_SCENE_MODES,
		Body: &pb.EngineRequest_Empty{
			Empty: &emptypb.Empty{},
		},
	})
	if err != nil {
		log.Fatalf("client get scene modes failed: %v", err)
	}

	sceneModes = resp.GetSceneModes().GetModes()
	currentSceneMode = resp.GetSceneModes().GetCurrentMode()
	currentScenePath = resp.GetSceneModes().GetCurrentScenePath()
}

func loadSceneMode(mode string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := streamRequest(ctx, &pb.EngineRequest{
		Operation: pb.Operation_OPERATION_LOAD_SCENE_MODE,
		Body: &pb.EngineRequest_SceneMode{
			SceneMode: &pb.SceneModeRef{
				Mode: mode,
			},
		},
	})
	if err != nil {
		log.Fatalf("client load scene mode failed: %v", err)
	}
}

func streamRequest(ctx context.Context, req *pb.EngineRequest) (*pb.EngineResponse, error) {
	stream, err := client.Stream(ctx)
	if err != nil {
		return nil, err
	}

	if err := stream.Send(req); err != nil {
		return nil, err
	}
	if err := stream.CloseSend(); err != nil {
		return nil, err
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, err
	}

	if !resp.GetSuccess() {
		return nil, fmt.Errorf("operation %s failed: %s", req.GetOperation().String(), resp.GetError())
	}

	return resp, nil
}
