package object

import "github.com/go-gl/mathgl/mgl32"

func (m *Model) computeLocalBounds() {
	if len(m.Meshes) == 0 {
		m.localBoundsMin = mgl32.Vec3{0, 0, 0}
		m.localBoundsMax = mgl32.Vec3{0, 0, 0}
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
			if p.X() < minBounds.X() {
				minBounds[0] = p.X()
			}
			if p.Y() < minBounds.Y() {
				minBounds[1] = p.Y()
			}
			if p.Z() < minBounds.Z() {
				minBounds[2] = p.Z()
			}
			if p.X() > maxBounds.X() {
				maxBounds[0] = p.X()
			}
			if p.Y() > maxBounds.Y() {
				maxBounds[1] = p.Y()
			}
			if p.Z() > maxBounds.Z() {
				maxBounds[2] = p.Z()
			}
		}
	}

	if !hasVertices {
		minBounds = mgl32.Vec3{0, 0, 0}
		maxBounds = mgl32.Vec3{0, 0, 0}
	}

	m.localBoundsMin = minBounds
	m.localBoundsMax = maxBounds
	m.hasLocalBounds = hasVertices
}

func (m *Model) worldTransform() mgl32.Mat4 {
	modelMat := mgl32.Ident4()
	modelMat = modelMat.Mul4(mgl32.Translate3D(m.Coordinates.X(), m.Coordinates.Y(), m.Coordinates.Z()))
	modelMat = modelMat.Mul4(mgl32.HomogRotate3D(mgl32.DegToRad(m.Rotation.W()), m.Rotation.Vec3()))
	modelMat = modelMat.Mul4(mgl32.Scale3D(m.Scale.X(), m.Scale.Y(), m.Scale.Z()))
	return modelMat
}

func (m *Model) WorldAABB() (mgl32.Vec3, mgl32.Vec3) {
	if !m.hasLocalBounds {
		return m.Coordinates, m.Coordinates
	}

	minB := m.localBoundsMin
	maxB := m.localBoundsMax
	corners := [8]mgl32.Vec3{
		{minB.X(), minB.Y(), minB.Z()},
		{maxB.X(), minB.Y(), minB.Z()},
		{minB.X(), maxB.Y(), minB.Z()},
		{maxB.X(), maxB.Y(), minB.Z()},
		{minB.X(), minB.Y(), maxB.Z()},
		{maxB.X(), minB.Y(), maxB.Z()},
		{minB.X(), maxB.Y(), maxB.Z()},
		{maxB.X(), maxB.Y(), maxB.Z()},
	}

	modelMat := m.worldTransform()
	first := modelMat.Mul4x1(mgl32.Vec4{corners[0].X(), corners[0].Y(), corners[0].Z(), 1.0}).Vec3()
	worldMin := first
	worldMax := first

	for i := 1; i < len(corners); i++ {
		worldCorner := modelMat.Mul4x1(mgl32.Vec4{corners[i].X(), corners[i].Y(), corners[i].Z(), 1.0}).Vec3()
		if worldCorner.X() < worldMin.X() {
			worldMin[0] = worldCorner.X()
		}
		if worldCorner.Y() < worldMin.Y() {
			worldMin[1] = worldCorner.Y()
		}
		if worldCorner.Z() < worldMin.Z() {
			worldMin[2] = worldCorner.Z()
		}
		if worldCorner.X() > worldMax.X() {
			worldMax[0] = worldCorner.X()
		}
		if worldCorner.Y() > worldMax.Y() {
			worldMax[1] = worldCorner.Y()
		}
		if worldCorner.Z() > worldMax.Z() {
			worldMax[2] = worldCorner.Z()
		}
	}

	return worldMin, worldMax
}

func (m *Model) Intersects(other *Model) bool {
	aMin, aMax := m.WorldAABB()
	bMin, bMax := other.WorldAABB()

	return aMin.X() <= bMax.X() && aMax.X() >= bMin.X() &&
		aMin.Y() <= bMax.Y() && aMax.Y() >= bMin.Y() &&
		aMin.Z() <= bMax.Z() && aMax.Z() >= bMin.Z()
}

func (m *Model) CollisionSeparation(other *Model) mgl32.Vec3 {
	aMin, aMax := m.WorldAABB()
	bMin, bMax := other.WorldAABB()

	overlapX := minf(aMax.X(), bMax.X()) - maxf(aMin.X(), bMin.X())
	overlapY := minf(aMax.Y(), bMax.Y()) - maxf(aMin.Y(), bMin.Y())
	overlapZ := minf(aMax.Z(), bMax.Z()) - maxf(aMin.Z(), bMin.Z())
	if overlapX <= 0 || overlapY <= 0 || overlapZ <= 0 {
		return mgl32.Vec3{0, 0, 0}
	}

	centerA := aMin.Add(aMax).Mul(0.5)
	centerB := bMin.Add(bMax).Mul(0.5)

	if overlapX <= overlapY && overlapX <= overlapZ {
		if centerA.X() < centerB.X() {
			return mgl32.Vec3{-overlapX, 0, 0}
		}
		return mgl32.Vec3{overlapX, 0, 0}
	}

	if overlapY <= overlapX && overlapY <= overlapZ {
		if centerA.Y() < centerB.Y() {
			return mgl32.Vec3{0, -overlapY, 0}
		}
		return mgl32.Vec3{0, overlapY, 0}
	}

	if centerA.Z() < centerB.Z() {
		return mgl32.Vec3{0, 0, -overlapZ}
	}
	return mgl32.Vec3{0, 0, overlapZ}
}

func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
