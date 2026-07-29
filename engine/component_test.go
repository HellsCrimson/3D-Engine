package engine

import (
	"testing"

	"3d-engine/input"

	"gopkg.in/yaml.v3"
)

// probe records which lifecycle callbacks fired, in order.
type probe struct {
	Speed float32 `yaml:"speed"`
	Label string  `yaml:"label"`

	calls   []string
	deltas  []float32
	entity  *Entity
	hasSelf bool
}

func (p *probe) Start(ctx *Context) {
	p.calls = append(p.calls, "start")
	p.entity = ctx.Entity
	p.hasSelf = ctx.Entity != nil && ctx.World != nil
}

func (p *probe) Update(ctx *Context) {
	p.calls = append(p.calls, "update")
	p.deltas = append(p.deltas, ctx.DeltaTime)
}

func (p *probe) FixedUpdate(ctx *Context) {
	p.calls = append(p.calls, "fixed")
	p.deltas = append(p.deltas, ctx.DeltaTime)
}

func (p *probe) OnDestroy(ctx *Context) {
	p.calls = append(p.calls, "destroy")
}

// testApp builds an App with just enough wired up to drive components. It never
// touches GL, so it is safe in a unit test.
func testApp() *App {
	a := &App{
		World:            NewWorld(),
		Components:       NewComponentRegistry(),
		Input:            input.NewMap(),
		deltaTime:        0.016,
		physicsDeltaTime: 0.02,
	}
	a.World.onDespawn = a.destroyComponents
	return a
}

func TestComponentLifecycleOrder(t *testing.T) {
	a := testApp()

	p := &probe{}
	entity := NewEntity("subject")
	entity.AddComponent(p)
	a.World.Spawn(entity)

	a.startAndUpdateComponents()
	a.startAndUpdateComponents()

	a.World.Write(func(entities []*Entity) {
		a.fixedUpdateComponents(entities)
	})

	a.World.Despawn(entity.Handle())

	want := []string{"start", "update", "update", "fixed", "destroy"}
	if len(p.calls) != len(want) {
		t.Fatalf("callbacks: got %v, want %v", p.calls, want)
	}
	for i := range want {
		if p.calls[i] != want[i] {
			t.Fatalf("callbacks: got %v, want %v", p.calls, want)
		}
	}

	if !p.hasSelf || p.entity != entity {
		t.Fatal("Start did not receive a usable context")
	}
}

// TestFixedUpdateUsesFixedStep guards the distinction that makes physics
// components correct: Update sees frame time, FixedUpdate sees the fixed step.
func TestFixedUpdateUsesFixedStep(t *testing.T) {
	a := testApp()

	p := &probe{}
	entity := NewEntity("subject")
	entity.AddComponent(p)
	a.World.Spawn(entity)

	a.startAndUpdateComponents()
	a.World.Write(func(entities []*Entity) {
		a.fixedUpdateComponents(entities)
	})

	if len(p.deltas) != 2 {
		t.Fatalf("expected 2 deltas, got %v", p.deltas)
	}
	if p.deltas[0] != a.deltaTime {
		t.Fatalf("Update delta: got %v, want frame time %v", p.deltas[0], a.deltaTime)
	}
	if p.deltas[1] != a.physicsDeltaTime {
		t.Fatalf("FixedUpdate delta: got %v, want fixed step %v", p.deltas[1], a.physicsDeltaTime)
	}
}

// TestComponentAddedLaterStartsOnce checks that a component attached after the
// entity is already live still gets exactly one Start.
func TestComponentAddedLaterStartsOnce(t *testing.T) {
	a := testApp()

	entity := NewEntity("subject")
	a.World.Spawn(entity)
	a.startAndUpdateComponents()

	p := &probe{}
	entity.AddComponent(p)

	a.startAndUpdateComponents()
	a.startAndUpdateComponents()

	starts := 0
	for _, call := range p.calls {
		if call == "start" {
			starts++
		}
	}
	if starts != 1 {
		t.Fatalf("expected exactly one Start, got %d in %v", starts, p.calls)
	}
}

func TestRegistryRejectsDuplicateAndUnknown(t *testing.T) {
	r := NewComponentRegistry()

	if err := r.Register("Probe", func() Component { return &probe{} }); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	if err := r.Register("Probe", func() Component { return &probe{} }); err == nil {
		t.Fatal("duplicate registration should fail, not silently redefine the type")
	}
	if _, err := r.New("Nope"); err == nil {
		t.Fatal("unknown component type should fail")
	}

	built, err := r.New("Probe")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if _, ok := built.(*probe); !ok {
		t.Fatalf("New returned %T", built)
	}
}

// TestPropsDecodeIntoComponent covers the path scene files rely on: the props
// block is decoded into the registered Go type.
func TestPropsDecodeIntoComponent(t *testing.T) {
	r := NewComponentRegistry()
	r.MustRegister("Probe", func() Component { return &probe{} })

	var props yaml.Node
	if err := yaml.Unmarshal([]byte("speed: 90.5\nlabel: spin\n"), &props); err != nil {
		t.Fatalf("fixture parse failed: %v", err)
	}

	component, err := r.New("Probe")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if err := props.Decode(component); err != nil {
		t.Fatalf("props decode failed: %v", err)
	}

	got := component.(*probe)
	if got.Speed != 90.5 || got.Label != "spin" {
		t.Fatalf("props not applied: %+v", got)
	}
}

func TestGetComponent(t *testing.T) {
	entity := NewEntity("subject")
	p := &probe{Label: "target"}
	entity.AddComponent(p)

	found, ok := GetComponent[*probe](entity)
	if !ok || found != p {
		t.Fatal("GetComponent did not find the attached component")
	}

	if _, ok := GetComponent[*RigidBody](entity); ok {
		t.Fatal("GetComponent found a component that was never attached")
	}
}

// TestSceneReplaceDestroysComponents makes sure a scene switch tears down the
// outgoing scene's components rather than dropping them on the floor.
func TestSceneReplaceDestroysComponents(t *testing.T) {
	a := testApp()

	p := &probe{}
	outgoing := NewEntity("outgoing")
	outgoing.AddComponent(p)
	a.World.Spawn(outgoing)
	a.startAndUpdateComponents()

	a.World.Replace([]*Entity{NewEntity("incoming")})

	destroyed := false
	for _, call := range p.calls {
		if call == "destroy" {
			destroyed = true
		}
	}
	if !destroyed {
		t.Fatalf("scene replace did not run OnDestroy: %v", p.calls)
	}
}
