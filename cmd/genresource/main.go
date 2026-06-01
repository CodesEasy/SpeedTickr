// Command genresource regenerates SpeedTickr's icon assets from internal/icon:
//
//   - cmd/speedtickr/rsrc_windows_{amd64,arm64}.syso — COFF resource objects the Go
//     linker merges into the Windows executable so Explorer and the taskbar show the
//     app icon for the file itself.
//   - docs/logo.png — the project logo (the icon on its tile).
//
// The committed outputs are what ship; nobody needs to run this to build the app.
// Re-run it only after changing the icon drawing:
//
//	go generate ./cmd/genresource
package main

//go:generate go run .

import (
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/codeseasy/speedtickr/internal/icon"
)

func main() {
	log.SetFlags(0)
	root := repoRoot()

	// Standard Windows icon sizes, from the notification area up to the 256px tile
	// Explorer shows in its large-icon views.
	sizes := []int{16, 32, 48, 64, 128, 256}
	imgs := make([]*image.RGBA, len(sizes))
	for i, s := range sizes {
		imgs[i] = icon.App(s)
	}

	for _, arch := range []struct {
		name    string
		machine uint16
	}{
		{"amd64", icon.MachineAMD64},
		{"arm64", icon.MachineARM64},
	} {
		data, err := icon.SYSO(arch.machine, imgs)
		if err != nil {
			log.Fatalf("encode %s resource: %v", arch.name, err)
		}
		out := filepath.Join(root, "cmd", "speedtickr", "rsrc_windows_"+arch.name+".syso")
		if err := os.WriteFile(out, data, 0o644); err != nil {
			log.Fatal(err)
		}
		log.Printf("wrote %s (%d bytes)", rel(root, out), len(data))
	}

	logo := filepath.Join(root, "docs", "logo.png")
	if err := os.MkdirAll(filepath.Dir(logo), 0o755); err != nil {
		log.Fatal(err)
	}
	writePNG(logo, icon.App(512))
	log.Printf("wrote %s", rel(root, logo))
}

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}

// repoRoot resolves the module root from this file's location so the generator
// writes to the right place no matter the working directory.
func repoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("cannot determine source location")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return r
	}
	return path
}
