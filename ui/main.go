package main

import (
	"fmt"
	"log"
	"runtime"

	pb "3d-engine-ui/grpc"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/backend/glfwbackend"
	"github.com/AllenDang/cimgui-go/examples/common"
	"github.com/AllenDang/cimgui-go/imgui"
	_ "github.com/AllenDang/cimgui-go/immarkdown"
	_ "github.com/AllenDang/cimgui-go/imnodes"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	currentBackend backend.Backend[glfwbackend.GLFWWindowFlags]

	client pb.EngineClient

	objects             []*pb.Object
	selectedObjectIndex int
	autoUpdate          bool = false
)

func init() {
	runtime.LockOSThread()
}

func main() {
	opts := []grpc.DialOption{}
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.NewClient("localhost:8080", opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client = pb.NewEngineClient(conn)

	common.Initialize()
	currentBackend, _ = backend.CreateBackend(glfwbackend.NewGLFWBackend())
	currentBackend.SetAfterCreateContextHook(common.AfterCreateContext)
	currentBackend.SetBeforeDestroyContextHook(common.BeforeDestroyContext)

	currentBackend.SetBgColor(imgui.NewVec4(0.0, 0.0, 0.0, 0.0))

	currentBackend.CreateWindow("3D Engine UI", 1200, 900)

	currentBackend.SetCloseCallback(func(b backend.Backend[glfwbackend.GLFWWindowFlags]) {
		fmt.Println("window is closing")
	})

	currentBackend.SetIcons(common.Image())

	currentBackend.Run(Loop)
}

func Loop() {
	// imgui.ShowDemoWindow()
	imgui.BeginV("3D Engine UI", nil, imgui.WindowFlagsAlwaysAutoResize)

	if imgui.Button("Get Objects") {
		getObjects()
	}

	imgui.Dummy(imgui.Vec2{X: 500, Y: 10})
	imgui.Separator()

	imgui.BeginTable("Objects", 2)
	imgui.TableSetupColumnV("Name", imgui.TableColumnFlagsNoHide|imgui.TableColumnFlagsWidthFixed, 100, 0)
	imgui.TableSetupColumnV("Select Object", imgui.TableColumnFlagsWidthFixed, 100, 0)
	imgui.TableHeadersRow()

	for index, object := range objects {
		imgui.PushIDStr(fmt.Sprintf("object#%d", index))

		imgui.TableNextColumn()
		imgui.Text(object.Name)

		imgui.TableNextColumn()
		if imgui.Button("Select") {
			selectedObjectIndex = index
		}

		imgui.PopID()
	}

	imgui.EndTable()

	imgui.Separator()

	if len(objects) > 0 {
		makeObjectTable()
	}

	imgui.End()
}

func makeObjectTable() {
	imgui.Checkbox("Auto Update", &autoUpdate)

	imgui.BeginTable("Object Properties", 2)
	imgui.TableSetupColumnV("Property", imgui.TableColumnFlagsNoHide|imgui.TableColumnFlagsWidthFixed, 100, 0)
	imgui.TableSetupColumnV("Value", imgui.TableColumnFlagsWidthStretch, 0, 0)
	imgui.TableHeadersRow()

	selectedObject := objects[selectedObjectIndex]

	imgui.PushItemWidth(100)

	imgui.TableNextColumn()
	imgui.Text("Id")

	imgui.TableNextColumn()
	imgui.Text(fmt.Sprintf("%d", selectedObject.Id))

	imgui.TableNextColumn()
	imgui.Text("Name")

	imgui.TableNextColumn()
	imgui.Text(selectedObject.Name)

	imgui.TableNextColumn()
	imgui.Text("Position")

	imgui.TableNextColumn()
	imgui.PushIDStr("Position")
	imgui.BeginGroup()
	imgui.DragFloatV("X", &selectedObject.Location.Position.X, 0.1, -1000000, 1000000, "%.2f", 0.0)
	imgui.SameLine()
	imgui.DragFloatV("Y", &selectedObject.Location.Position.Y, 0.1, -1000000, 1000000, "%.2f", 0.0)
	imgui.SameLine()
	imgui.DragFloatV("Z", &selectedObject.Location.Position.Z, 0.1, -1000000, 1000000, "%.2f", 0.0)
	imgui.EndGroup()
	imgui.PopID()

	imgui.TableNextColumn()
	imgui.Text("Rotation")

	imgui.TableNextColumn()
	imgui.PushIDStr("Rotation")
	imgui.BeginGroup()
	imgui.DragFloatV("X", &selectedObject.Location.Rotation.X, 0.1, -1, 1, "%.2f", 0.0)
	imgui.SameLine()
	imgui.DragFloatV("Y", &selectedObject.Location.Rotation.Y, 0.1, -1, 1, "%.2f", 0.0)
	imgui.SameLine()
	imgui.DragFloatV("Z", &selectedObject.Location.Rotation.Z, 0.1, -1, 1, "%.2f", 0.0)
	imgui.SameLine()
	imgui.DragFloatV("Angle", &selectedObject.Location.Rotation.W, 1.0, -360, 360, "%.0f", 0.0)
	imgui.EndGroup()
	imgui.PopID()

	imgui.TableNextColumn()
	imgui.Text("Scale")

	imgui.TableNextColumn()
	imgui.PushIDStr("Scale")
	imgui.BeginGroup()
	imgui.DragFloatV("X", &selectedObject.Location.Scale.X, 0.1, -1000000, 1000000, "%.2f", 0.0)
	imgui.SameLine()
	imgui.DragFloatV("Y", &selectedObject.Location.Scale.Y, 0.1, -1000000, 1000000, "%.2f", 0.0)
	imgui.SameLine()
	imgui.DragFloatV("Z", &selectedObject.Location.Scale.Z, 0.1, -1000000, 1000000, "%.2f", 0.0)
	imgui.EndGroup()
	imgui.PopID()

	imgui.PopItemWidth()

	imgui.EndTable()

	if autoUpdate || imgui.Button("Update Object") {
		updateObject(selectedObject)
	}
}
