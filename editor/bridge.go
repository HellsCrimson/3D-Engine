package editor

// This file is the whole reason the editor can live in the engine's process.
//
// cimgui-go ships Dear ImGui's GLFW and OpenGL3 backends inside cimgui.a, but
// only exposes them through its own backend package — which calls glfwInit and
// glfwCreateWindow itself and runs its own event loop. Two owners of the window
// cannot be reconciled, which is what makes the obvious approach look
// impossible.
//
// The way out: cimgui.a does not bundle GLFW. Its glfw* symbols are undefined
// references resolved at final link, so they bind to the copy of GLFW that
// go-gl/glfw already compiled into the binary. There is exactly one GLFW in the
// process, and go-gl exposes the raw window as Window.Handle(). Declaring the
// backend entry points here lets us initialise ImGui against the engine's
// existing window and context.
//
// No LDFLAGS are needed: importing cimgui-go/imgui puts cimgui.a on the link
// line already, and these are just prototypes for symbols in it.

/*
#include <stdint.h>
#include <stdlib.h>
#include <stdbool.h>

// Dear ImGui backend entry points, defined in cimgui.a.
extern bool ImGui_ImplGlfw_InitForOpenGL(void* window, bool install_callbacks);
extern void ImGui_ImplGlfw_Shutdown(void);
extern void ImGui_ImplGlfw_NewFrame(void);

extern bool ImGui_ImplOpenGL3_Init(const char* glsl_version);
extern void ImGui_ImplOpenGL3_Shutdown(void);
extern void ImGui_ImplOpenGL3_NewFrame(void);
extern void ImGui_ImplOpenGL3_RenderDrawData(void* draw_data);

// cimgui core. Declared with void* instead of the real ImDrawData type so this
// file does not need cimgui's headers.
extern void  igRender(void);
extern void* igGetDrawData(void);

// engineImguiRender keeps igRender and RenderDrawData together on the C side,
// which avoids having to get at the draw data pointer from Go.
static void engineImguiRender(void) {
	igRender();
	ImGui_ImplOpenGL3_RenderDrawData(igGetDrawData());
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// initBackends attaches Dear ImGui to an existing GLFW window and GL context.
//
// installCallbacks lets ImGui chain onto the window's existing GLFW callbacks:
// it stores whatever is already registered and forwards to it, so the engine's
// own handlers keep firing. Register the engine's callbacks first, then call
// this.
func initBackends(window unsafe.Pointer, glslVersion string) error {
	if !C.ImGui_ImplGlfw_InitForOpenGL(window, C.bool(true)) {
		return fmt.Errorf("ImGui_ImplGlfw_InitForOpenGL failed")
	}

	version := C.CString(glslVersion)
	defer C.free(unsafe.Pointer(version))

	if !C.ImGui_ImplOpenGL3_Init(version) {
		C.ImGui_ImplGlfw_Shutdown()
		return fmt.Errorf("ImGui_ImplOpenGL3_Init failed")
	}

	return nil
}

// newFrame starts the platform and renderer frame. imgui.NewFrame must follow.
func newFrame() {
	C.ImGui_ImplOpenGL3_NewFrame()
	C.ImGui_ImplGlfw_NewFrame()
}

// render finishes the ImGui frame and issues its draw calls.
func render() {
	C.engineImguiRender()
}

func shutdownBackends() {
	C.ImGui_ImplOpenGL3_Shutdown()
	C.ImGui_ImplGlfw_Shutdown()
}
