package editor

import (
	"fmt"

	"3d-engine/engine"

	"github.com/AllenDang/cimgui-go/imgui"
)

// entityRow is one line of the object list, snapshotted under the world lock so
// the widgets never touch a live entity pointer.
type entityRow struct {
	handle engine.Handle
	name   string
}

func (e *Editor) draw() {
	imgui.BeginV("3D Engine", nil, imgui.WindowFlagsAlwaysAutoResize)

	e.drawStats()
	imgui.Separator()
	e.drawSceneModes()
	imgui.Separator()

	rows := e.entityRows()
	e.drawEntityList(rows)
	imgui.Separator()
	e.drawInspector()

	imgui.End()
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

// entityRows snapshots the world. Names and handles are copied out so the rest
// of the frame can run without holding the lock.
func (e *Editor) entityRows() []entityRow {
	var rows []entityRow

	e.app.World.Read(func(entities []*engine.Entity) {
		rows = make([]entityRow, 0, len(entities))
		for _, entity := range entities {
			rows = append(rows, entityRow{handle: entity.Handle(), name: entity.Name})
		}
	})

	return rows
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

func (e *Editor) drawEntityList(rows []entityRow) {
	if imgui.BeginTable("Objects", 3) {
		imgui.TableSetupColumnV("Name", imgui.TableColumnFlagsWidthStretch, 0, 0)
		imgui.TableSetupColumnV("Handle", imgui.TableColumnFlagsWidthFixed, 80, 0)
		imgui.TableSetupColumnV("Action", imgui.TableColumnFlagsWidthFixed, 80, 0)
		imgui.TableHeadersRow()

		for i, row := range rows {
			imgui.PushIDStr(fmt.Sprintf("object-%d", i))

			imgui.TableNextColumn()
			imgui.Text(row.name)

			imgui.TableNextColumn()
			imgui.Text(fmt.Sprintf("%d v%d", row.handle.Index, row.handle.Generation))

			imgui.TableNextColumn()
			if imgui.Button("Select") {
				e.selected = row.handle
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
	entity := e.app.World.Get(e.selected)
	if entity == nil {
		imgui.Text(fmt.Sprintf("Object %s no longer exists", e.selected))
		if imgui.Button("Clear selection") {
			e.selected = engine.NoHandle
		}
		return
	}

	imgui.Text(fmt.Sprintf("Name: %s", entity.Name))
	imgui.Text(fmt.Sprintf("Handle: %d v%d", e.selected.Index, e.selected.Generation))

	// Track the entity except while a widget is being dragged: otherwise the
	// physics step would overwrite the value under the user's cursor every
	// frame.
	if !imgui.IsAnyItemActive() {
		e.draft = entity.Transform()
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
		// Straight into the World API — the same call the gRPC handler makes,
		// with no serialization in between.
		draft := e.draft
		e.app.World.Mutate(e.selected, func(target *engine.Entity) {
			target.SetTransform(draft)
		})
	}

	imgui.SameLine()
	if imgui.Button("Despawn") {
		handle := e.selected
		e.app.Defer(func(a *engine.App) error {
			a.World.Despawn(handle)
			return nil
		})
		e.selected = engine.NoHandle
	}
}
