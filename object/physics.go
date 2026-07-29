package object

import "github.com/go-gl/mathgl/mgl32"

func (m *Model) computeLocalBounds() {
	if len(m.Meshes) == 0 {
		m.localBounds = AABB{}
		m.hasLocalBounds = false
		return
	}

	minBounds := mgl32.Vec3{1e9, 1e9, 1e9}
	maxBounds := mgl32.Vec3{-1e9, -1e9, -1e9}
	hasVertices := false

	for _, mesh := range m.Meshes {
		for _, vertex := range mesh.Vertices {
			hasVertices = true
			p := vertex.Position
			for axis := 0; axis < 3; axis++ {
				if p[axis] < minBounds[axis] {
					minBounds[axis] = p[axis]
				}
				if p[axis] > maxBounds[axis] {
					maxBounds[axis] = p[axis]
				}
			}
		}
	}

	if !hasVertices {
		minBounds = mgl32.Vec3{0, 0, 0}
		maxBounds = mgl32.Vec3{0, 0, 0}
	}

	m.localBounds = AABB{Min: minBounds, Max: maxBounds}
	m.hasLocalBounds = hasVertices
}

// LocalBounds returns the model-space bounds and whether the model had any
// geometry to measure.
func (m *Model) LocalBounds() (AABB, bool) {
	return m.localBounds, m.hasLocalBounds
}

// WorldAABB fits an axis-aligned box around the model placed by modelMat. The
// transform now comes from the entity that owns this model rather than from
// fields on the model itself.
func (m *Model) WorldAABB(modelMat mgl32.Mat4) AABB {
	if !m.hasLocalBounds {
		return PointAABB(modelMat.Col(3).Vec3())
	}
	return m.localBounds.Transform(modelMat)
}
