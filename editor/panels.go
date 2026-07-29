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
	byHandle := indexRows(rows)

	e.drawSpawn()
	imgui.Separator()
	e.drawEntityTree(rows, byHandle)
	imgui.Separator()
	e.drawInspector(rows, byHandle)

	imgui.End()
}

// indexRows makes the snapshot walkable as a tree. ObjectInfo carries child
// handles rather than nested structs, so drawing the hierarchy means resolving
// them against the rest of the snapshot.
func indexRows(rows []engine.ObjectInfo) map[engine.Handle]engine.ObjectInfo {
	byHandle := make(map[engine.Handle]engine.ObjectInfo, len(rows))
	for _, row := range rows {
		byHandle[row.Handle] = row
	}
	return byHandle
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

// drawSpawn is the create-entity form.
//
// It goes through App.SpawnObject, the same call the gRPC ADD_OBJECT handler and
// scene loading use, so there is nothing the editor can create that another
// front-end could not.
func (e *Editor) drawSpawn() {
	if !imgui.CollapsingHeaderTreeNodeFlagsV("Create entity", 0) {
		return
	}

	imgui.PushItemWidth(240)
	imgui.InputTextWithHint("Name", "entity name", &e.spawnName, 0, nil)
	imgui.InputTextWithHint("Model", "path/to.obj — leave empty for a light or group", &e.spawnModel, 0, nil)

	// "(none)" first, so the zero value of spawnComponent means no component.
	choices := append([]string{"(none)"}, e.app.Components.Names()...)
	imgui.ComboStrarr("Component", &e.spawnComponent, choices, int32(len(choices)))
	imgui.PopItemWidth()

	imgui.Checkbox("Rigid body", &e.spawnWithBody)
	if e.spawnWithBody {
		imgui.SameLine()
		imgui.Checkbox("Static", &e.spawnStatic)
	}

	if !e.selected.IsZero() {
		imgui.Checkbox("Child of selection", &e.spawnAsChild)
	} else {
		e.spawnAsChild = false
		imgui.TextDisabled("Select an entity to spawn a child of it")
	}

	if imgui.Button("Spawn") {
		e.spawn(int(e.spawnComponent), choices)
	}

	if e.spawnStatus != "" {
		imgui.Text(e.spawnStatus)
	}
}

// spawn builds the spec the form describes and creates it.
//
// Called from Frame, which the engine runs on the frame-loop goroutine, so
// SpawnObject can be called directly: importing the model is GL work, and this is
// where GL work is allowed.
func (e *Editor) spawn(componentChoice int, choices []string) {
	name := e.spawnName
	if name == "" {
		name = "entity"
	}

	spec := engine.ObjectSpec{
		Name:      name,
		Model:     e.spawnModel,
		Transform: engine.IdentityTransform(),
	}

	if e.spawnWithBody {
		spec.Body = &engine.RigidBody{Static: e.spawnStatic}
	}

	if componentChoice > 0 && componentChoice < len(choices) {
		component, err := e.app.Components.New(choices[componentChoice])
		if err != nil {
			e.spawnStatus = err.Error()
			return
		}
		spec.Components = append(spec.Components, component)
	}

	if e.spawnAsChild && !e.selected.IsZero() {
		// A child keeps the identity transform, which puts it exactly on its
		// parent. Dropping it in front of the camera instead would be wrong: a
		// child's position is in its parent's space, not the world's.
		spec.Parent = e.selected
	} else {
		// A root entity lands in front of the camera rather than at the origin,
		// which may be nowhere near what you are looking at.
		spec.Transform.Position = e.app.Camera.CameraPos.Add(e.app.Camera.CameraFront.Mul(5))
	}

	spawned, err := e.app.SpawnObject(spec)
	if err != nil {
		e.spawnStatus = err.Error()
		return
	}

	e.selected = spawned.Handle()
	e.reparentTarget = spec.Parent
	e.spawnStatus = fmt.Sprintf("Spawned %s", name)
	e.status = ""
}

// drawEntityTree draws the scene as a hierarchy. It replaced a flat table, which
// could list a parented entity but had no way to show what it was parented to.
//
// Only roots are walked from the top; children are reached through their parent,
// so each entity is drawn once even though the world stores them all in one flat
// slice.
func (e *Editor) drawEntityTree(rows []engine.ObjectInfo, byHandle map[engine.Handle]engine.ObjectInfo) {
	imgui.Text(fmt.Sprintf("Hierarchy (%d entities)", len(rows)))

	// A fixed-height scrolling region: the window is AlwaysAutoResize, so without
	// this a large scene would grow it past the bottom of the screen.
	if !imgui.BeginChildStrV("hierarchy", imgui.NewVec2(0, 220), 0, 0) {
		imgui.EndChild()
		return
	}

	for _, row := range rows {
		if row.Parent.IsZero() {
			e.drawEntityNode(row, byHandle)
		}
	}

	imgui.EndChild()
}

func (e *Editor) drawEntityNode(row engine.ObjectInfo, byHandle map[engine.Handle]engine.ObjectInfo) {
	flags := imgui.TreeNodeFlagsOpenOnArrow |
		imgui.TreeNodeFlagsDefaultOpen |
		imgui.TreeNodeFlagsSpanAvailWidth
	if len(row.Children) == 0 {
		flags |= imgui.TreeNodeFlagsLeaf
	}
	if row.Handle == e.selected {
		flags |= imgui.TreeNodeFlagsSelected
	}

	// The handle is in the ID, not just the label, so two entities sharing a name
	// stay distinct widgets.
	imgui.PushIDStr(fmt.Sprintf("entity-%d-%d", row.Handle.Index, row.Handle.Generation))
	open := imgui.TreeNodeExStrV(
		fmt.Sprintf("%s  [%d v%d]", row.Name, row.Handle.Index, row.Handle.Generation),
		flags)

	// OpenOnArrow above means a click on the label selects instead of collapsing.
	if imgui.IsItemClicked() {
		e.selected = row.Handle
		e.status = ""
		e.reparentTarget = row.Parent
	}

	if open {
		for _, childHandle := range row.Children {
			child, ok := byHandle[childHandle]
			if !ok {
				// A child that despawned between the snapshot and now. Nothing to
				// draw, and it will be gone from the next frame's snapshot.
				continue
			}
			e.drawEntityNode(child, byHandle)
		}
		imgui.TreePop()
	}

	imgui.PopID()
}

// drawReparent is the parent picker.
//
// Reparenting keeps the entity's local transform, so it jumps to sit at the same
// offset from its new parent rather than holding its world position. Holding the
// world position would mean decomposing a world matrix back into
// position/rotation/scale, which axis-angle rotations do not survive.
func (e *Editor) drawReparent(info engine.ObjectInfo, rows []engine.ObjectInfo, byHandle map[engine.Handle]engine.ObjectInfo) {
	current := "(scene root)"
	if !info.Parent.IsZero() {
		if parent, ok := byHandle[info.Parent]; ok {
			current = parent.Name
		} else {
			current = info.Parent.String()
		}
	}
	imgui.Text(fmt.Sprintf("Parent: %s", current))

	preview := "(scene root)"
	if !e.reparentTarget.IsZero() {
		if target, ok := byHandle[e.reparentTarget]; ok {
			preview = target.Name
		} else {
			// The chosen parent has been despawned since it was picked.
			preview = "(no longer exists)"
		}
	}

	imgui.PushItemWidth(200)
	if imgui.BeginCombo("##reparent", preview) {
		if imgui.SelectableBool("(scene root)") {
			e.reparentTarget = engine.NoHandle
		}

		for _, candidate := range rows {
			// Neither itself nor anything below it: those are the choices that
			// would make a cycle, and WorldMatrix would recurse until the stack
			// gave out.
			if candidate.Handle == info.Handle || isDescendantOf(candidate.Handle, info.Handle, byHandle) {
				continue
			}
			label := fmt.Sprintf("%s [%d v%d]", candidate.Name,
				candidate.Handle.Index, candidate.Handle.Generation)
			if imgui.SelectableBool(label) {
				e.reparentTarget = candidate.Handle
			}
		}

		imgui.EndCombo()
	}
	imgui.PopItemWidth()

	imgui.SameLine()
	if imgui.Button("Reparent") {
		if err := e.app.SetParent(e.selected, e.reparentTarget); err != nil {
			e.status = err.Error()
		} else {
			e.status = ""
		}
	}
}

// isDescendantOf reports whether candidate sits below root in the snapshot.
func isDescendantOf(candidate, root engine.Handle, byHandle map[engine.Handle]engine.ObjectInfo) bool {
	info, ok := byHandle[candidate]
	if !ok {
		return false
	}

	for !info.Parent.IsZero() {
		if info.Parent == root {
			return true
		}
		info, ok = byHandle[info.Parent]
		if !ok {
			return false
		}
	}
	return false
}

func (e *Editor) drawInspector(rows []engine.ObjectInfo, byHandle map[engine.Handle]engine.ObjectInfo) {
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
	if len(info.Children) > 0 {
		imgui.Text(fmt.Sprintf("Children: %d", len(info.Children)))
	}

	e.drawReparent(info, rows, byHandle)

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

	// Both go through the object API, which releases the entity's model back to
	// the asset cache. World.Despawn on its own would drop the entity and leak
	// its GPU memory. Frame runs on the frame-loop goroutine, which is where that
	// release is allowed, so these are direct calls rather than App.Defer.
	imgui.SameLine()
	if imgui.Button("Delete") {
		// The subtree, which is what deleting a thing means. Deleting the parent
		// alone would leave its children behind, lifted to the scene root.
		if err := e.app.DespawnTree(e.selected); err != nil {
			e.status = err.Error()
		} else {
			e.selected = engine.NoHandle
			e.reparentTarget = engine.NoHandle
		}
	}

	if len(info.Children) > 0 {
		imgui.SameLine()
		if imgui.Button("Delete, keep children") {
			if err := e.app.DespawnObject(e.selected); err != nil {
				e.status = err.Error()
			} else {
				e.selected = engine.NoHandle
				e.reparentTarget = engine.NoHandle
			}
		}
	}

	if e.status != "" {
		imgui.Text(e.status)
	}
}
