package editor

import (
	"fmt"

	"3d-engine/engine"

	"github.com/AllenDang/cimgui-go/imgui"
)

func (e *Editor) draw() {
	imgui.BeginV("3D Engine", nil, imgui.WindowFlagsAlwaysAutoResize)

	e.drawStats()
	imgui.Separator()
	e.drawSceneModes()
	imgui.Separator()

	e.drawSave()
	imgui.Separator()

	rows := e.entityRows()
	e.drawEntityList(rows)
	imgui.Separator()
	e.drawInspector()

	imgui.End()
}

// drawSave writes the live world back to a scene file.
//
// The path defaults to the scene currently loaded, so the common case is Save
// over what you opened, but it stays editable so the world can be forked into a
// new file without leaving the editor. It is re-defaulted whenever the loaded
// scene changes, unless the field has been typed into.
func (e *Editor) drawSave() {
	current := e.app.Scenes.CurrentScenePath()
	if !e.savePathEdited && e.savePath != current {
		e.savePath = current
	}

	imgui.PushItemWidth(320)
	if imgui.InputTextWithHint("##save-path", "scene path", &e.savePath, 0, nil) {
		e.savePathEdited = true
	}
	imgui.PopItemWidth()

	imgui.SameLine()
	if imgui.Button("Save") {
		// Frame runs on the frame-loop goroutine, which is where SaveScene has
		// to be called: it reads the skybox and the camera, and both are only
		// written there.
		if err := e.app.SaveScene(e.savePath); err != nil {
			e.saveStatus = err.Error()
		} else {
			e.saveStatus = fmt.Sprintf("Saved to %s", e.savePath)
		}
	}

	if e.savePathEdited {
		imgui.SameLine()
		if imgui.Button("Reset path") {
			e.savePathEdited = false
			e.savePath = current
			e.saveStatus = ""
		}
	}

	if e.saveStatus != "" {
		imgui.Text(e.saveStatus)
	}
}

func (e *Editor) drawStats() {
	imgui.Text(fmt.Sprintf("Scene: %s", e.app.Scenes.CurrentScenePath()))
	imgui.Text(fmt.Sprintf("Mode:  %s", e.app.Scenes.CurrentSceneMode()))
	imgui.Text(fmt.Sprintf("Entities: %d", e.app.World.Len()))

	models, holds := e.app.Assets.Stats()
	imgui.Text(fmt.Sprintf("Models resident: %d (%d refs)", models, holds))

	imgui.Text("C releases the cursor to use this panel")

	if imgui.CollapsingHeaderTreeNodeFlagsV("Key bindings", 0) {
		if imgui.BeginTable("Bindings", 2) {
			imgui.TableSetupColumnV("Action", imgui.TableColumnFlagsWidthFixed, 180, 0)
			imgui.TableSetupColumnV("Keys", imgui.TableColumnFlagsWidthStretch, 0, 0)
			imgui.TableHeadersRow()

			for _, action := range e.app.Input.Actions() {
				imgui.TableNextColumn()
				imgui.Text(string(action))
				imgui.TableNextColumn()
				imgui.Text(e.app.Input.Describe(action))
			}

			imgui.EndTable()
		}
	}
}

// entityRows snapshots the world through the engine's object API, the same one
// the gRPC GET_OBJECTS handler uses.
func (e *Editor) entityRows() []engine.ObjectInfo {
	return e.app.ListObjects()
}

func (e *Editor) drawSceneModes() {
	if len(e.sceneModes) == 0 {
		e.sceneModes = e.app.Scenes.ListModeNames()
	}
	if len(e.sceneModes) == 0 {
		return
	}
	if e.selectedScene >= len(e.sceneModes) {
		e.selectedScene = 0
	}

	paths := e.app.Scenes.ListModes()

	if imgui.BeginTable("Scene Modes", 3) {
		imgui.TableSetupColumnV("Mode", imgui.TableColumnFlagsWidthFixed, 120, 0)
		imgui.TableSetupColumnV("Path", imgui.TableColumnFlagsWidthStretch, 0, 0)
		imgui.TableSetupColumnV("Action", imgui.TableColumnFlagsWidthFixed, 80, 0)
		imgui.TableHeadersRow()

		for i, mode := range e.sceneModes {
			imgui.PushIDStr(fmt.Sprintf("scene-mode-%d", i))
			imgui.TableNextColumn()
			imgui.Text(mode)
			imgui.TableNextColumn()
			imgui.Text(paths[mode])
			imgui.TableNextColumn()
			if imgui.Button("Load") {
				// Queued onto the frame loop by the scene manager, so this
				// returns immediately and the load happens on the GL thread.
				if err := e.app.Scenes.RequestSceneModeChange(mode); err != nil {
					imgui.Text(err.Error())
				}
				e.selected = engine.NoHandle
			}
			imgui.PopID()
		}

		imgui.EndTable()
	}
}

func (e *Editor) drawEntityList(rows []engine.ObjectInfo) {
	if imgui.BeginTable("Objects", 3) {
		imgui.TableSetupColumnV("Name", imgui.TableColumnFlagsWidthStretch, 0, 0)
		imgui.TableSetupColumnV("Handle", imgui.TableColumnFlagsWidthFixed, 80, 0)
		imgui.TableSetupColumnV("Action", imgui.TableColumnFlagsWidthFixed, 80, 0)
		imgui.TableHeadersRow()

		for i, row := range rows {
			imgui.PushIDStr(fmt.Sprintf("object-%d", i))

			imgui.TableNextColumn()
			imgui.Text(row.Name)

			imgui.TableNextColumn()
			imgui.Text(fmt.Sprintf("%d v%d", row.Handle.Index, row.Handle.Generation))

			imgui.TableNextColumn()
			if imgui.Button("Select") {
				e.selected = row.Handle
				e.status = ""
			}

			imgui.PopID()
		}

		imgui.EndTable()
	}
}

func (e *Editor) drawInspector() {
	if e.selected.IsZero() {
		imgui.Text("No object selected")
		return
	}

	// The handle stops resolving after a despawn or a scene switch, which is
	// exactly the stale-reference case the generation counter exists to catch.
	info, ok := e.app.ObjectInfo(e.selected)
	if !ok {
		imgui.Text(fmt.Sprintf("Object %s no longer exists", e.selected))
		if imgui.Button("Clear selection") {
			e.selected = engine.NoHandle
		}
		return
	}

	imgui.Text(fmt.Sprintf("Name: %s", info.Name))
	imgui.Text(fmt.Sprintf("Handle: %d v%d", e.selected.Index, e.selected.Generation))
	if info.Model != "" {
		imgui.Text(fmt.Sprintf("Model: %s", info.Model))
	}

	// Track the entity except while a widget is being dragged: otherwise the
	// physics step would overwrite the value under the user's cursor every
	// frame.
	if !imgui.IsAnyItemActive() {
		e.draft = info.Transform
	}

	imgui.Checkbox("Auto apply", &e.autoApply)

	changed := false
	imgui.PushItemWidth(90)

	imgui.Text("Position")
	imgui.PushIDStr("position")
	changed = imgui.DragFloatV("X", &e.draft.Position[0], 0.1, -1e6, 1e6, "%.2f", 0) || changed
	imgui.SameLine()
	changed = imgui.DragFloatV("Y", &e.draft.Position[1], 0.1, -1e6, 1e6, "%.2f", 0) || changed
	imgui.SameLine()
	changed = imgui.DragFloatV("Z", &e.draft.Position[2], 0.1, -1e6, 1e6, "%.2f", 0) || changed
	imgui.PopID()

	imgui.Text("Rotation (axis + degrees)")
	imgui.PushIDStr("rotation")
	changed = imgui.DragFloatV("X", &e.draft.Rotation[0], 0.01, -1, 1, "%.2f", 0) || changed
	imgui.SameLine()
	changed = imgui.DragFloatV("Y", &e.draft.Rotation[1], 0.01, -1, 1, "%.2f", 0) || changed
	imgui.SameLine()
	changed = imgui.DragFloatV("Z", &e.draft.Rotation[2], 0.01, -1, 1, "%.2f", 0) || changed
	imgui.SameLine()
	changed = imgui.DragFloatV("Angle", &e.draft.Rotation[3], 1.0, -360, 360, "%.0f", 0) || changed
	imgui.PopID()

	imgui.Text("Scale")
	imgui.PushIDStr("scale")
	changed = imgui.DragFloatV("X", &e.draft.Scale[0], 0.01, -1e6, 1e6, "%.3f", 0) || changed
	imgui.SameLine()
	changed = imgui.DragFloatV("Y", &e.draft.Scale[1], 0.01, -1e6, 1e6, "%.3f", 0) || changed
	imgui.SameLine()
	changed = imgui.DragFloatV("Z", &e.draft.Scale[2], 0.01, -1e6, 1e6, "%.3f", 0) || changed
	imgui.PopID()

	imgui.PopItemWidth()

	apply := imgui.Button("Apply")
	if (e.autoApply && changed) || apply {
		// The same call the gRPC handler makes, minus the serialization.
		draft := e.draft
		if err := e.app.UpdateTransform(e.selected, func(t *engine.Transform) {
			*t = draft
		}); err != nil {
			e.status = err.Error()
		}
	}

	imgui.SameLine()
	if imgui.Button("Despawn") {
		// Deferred, not direct: DespawnObject releases the model back to the
		// asset cache, which is GL work and belongs on the frame loop. Going
		// through World.Despawn instead would drop the entity and leak it.
		handle := e.selected
		e.app.Defer(func(a *engine.App) error {
			return a.DespawnObject(handle)
		})
		e.selected = engine.NoHandle
	}

	if e.status != "" {
		imgui.Text(e.status)
	}
}
