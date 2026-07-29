package main

import (
	"3d-engine/engine"
	"3d-engine/utils"
	"runtime"
)

func init() {
	// Every GL call must happen on this goroutine.
	runtime.LockOSThread()
}

func main() {
	utils.ParseArgs()

	app, err := engine.New(engine.Options{
		ConfigPath: utils.GetContext().ConfigPath,
		ScenePath:  utils.GetContext().ScenePath,
	})
	if err != nil {
		utils.Logger().Fatalln(err)
	}
	defer app.Close()

	if err := app.Run(); err != nil {
		utils.Logger().Fatalln(err)
	}
}
