package main

import (
	"runtime"

	"3d-engine/editor"
	"3d-engine/engine"
	"3d-engine/utils"
)

func init() {
	// Every GL call must happen on this goroutine.
	runtime.LockOSThread()
}

func main() {
	args := utils.ParseArgs()

	app, err := engine.New(engine.Options{
		ConfigPath: args.ConfigPath,
		ScenePath:  args.ScenePath,
	})
	if err != nil {
		utils.Logger().Fatalln(err)
	}
	defer app.Close()

	// The editor draws into the engine's own window and context, so there is
	// one process to launch and debug. It needs a live GL context, which is why
	// it is built after engine.New rather than passed into it.
	if !args.NoEditor {
		ed, err := editor.New(app)
		if err != nil {
			utils.Logger().Fatalln(err)
		}
		app.SetOverlay(ed)
	}

	if err := app.Run(); err != nil {
		utils.Logger().Fatalln(err)
	}
}
