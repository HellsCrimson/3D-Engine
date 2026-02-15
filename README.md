# 3D-Engine

Written in Golang, using OpenGL

## Usage

First do
```
go mod tidy
```

Then you can launch it with
```
go run main.go
```

You will need to adjust the scene.yml to add your own models

## Scene Modes

Configure named scene modes in `config.yml`:

- `defaultSceneMode`: mode loaded at startup (if present)
- `sceneModes`: map of mode name to scene YAML path

You can switch scene modes at runtime from the UI (Get Scene Modes -> Load Selected Scene Mode).

## Controls

- `G`: toggle gravity on/off
- `H`: switch gravity axis (`-Y` / `-Z`) for world-space testing
- `P`: toggle player gravity mode (camera uses collider + gravity)
- `Space`: jump when player gravity mode is enabled and grounded
- `B`: toggle collision debug boxes (red=model, yellow=mesh, green=player)
- `Z`: toggle wireframe mode
- `F`: toggle flashlight

## References

[Learn OpenGL❤️](https://learnopengl.com)  
[Go-GL](https://github.com/go-gl/gl)  
[Go-GLFW](https://github.com/go-gl/glfw)  
[MathGL](https://github.com/go-gl/mathgl)  
