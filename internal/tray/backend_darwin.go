//go:build darwin

package tray

// macOS tray backend, pure Go via purego. fyne.io/systray draws the macOS menu bar
// through Objective-C/cgo, which forces a Mac (with the SDK) to build. purego instead
// talks to the Objective-C runtime at runtime — dlopen AppKit on the Mac and call
// objc_msgSend — so this file builds with CGO disabled and the whole app
// cross-compiles to darwin from any host, while still drawing the same native
// NSStatusItem + NSMenu.
//
// AppKit is main-thread-only and [NSApp run] blocks, so the main goroutine is pinned
// to thread 0 (see init) and Run owns it; any update from another goroutine hops to
// the main thread via runOnMain. Menu construction happens inside onReady, which Run
// calls on the main thread, so those calls touch AppKit directly.

import (
	"log/slog"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// AppKit/Foundation enum values (there are no headers without cgo).
const (
	nsApplicationActivationPolicyAccessory = 1 // menu-bar app, no Dock icon
	nsControlStateValueOff                 = 0
	nsControlStateValueOn                  = 1
	nsImageLeft                            = 2 // NSCellImagePosition: image left of title
)

// nsVariableStatusItemLength (-1.0) is a CGFloat, so it must be a float64 to land in
// a floating-point register when passed to statusItemWithLength:.
const nsVariableStatusItemLength float64 = -1.0

// sel caches every selector once (RegisterName takes the global Obj-C lock).
var sel struct {
	alloc, init, new                             objc.SEL
	sharedApplication, setActivationPolicy       objc.SEL
	activateIgnoringOtherApps, run, terminate    objc.SEL
	systemStatusBar, statusItemWithLength        objc.SEL
	button, setMenu                              objc.SEL
	setTitle, setToolTip, setImage               objc.SEL
	setImagePosition, setSize                    objc.SEL
	stringWithUTF8, dataWithBytesLength          objc.SEL
	initWithData                                 objc.SEL
	addItem, separatorItem, initItem             objc.SEL
	setState, setSubmenu, setTarget, handleClick objc.SEL
}

var (
	classNSApplication objc.Class
	classNSStatusBar   objc.Class
	classNSMenu        objc.Class
	classNSMenuItem    objc.Class
	classNSString      objc.Class
	classNSData        objc.Class
	classNSImage       objc.Class

	clickTarget objc.ID // shared target for every menu item's action
)

// init pins the main goroutine to OS thread 0. AppKit's run loop and UI calls are
// only valid on the process's main thread, and the Go scheduler may migrate the main
// goroutine off it at the first blocking call in main() (e.g. reading the config).
// Locking here — at package init, before main() runs — binds it for the whole run, so
// by the time Run reaches [NSApp run] we are guaranteed to be on thread 0.
func init() {
	runtime.LockOSThread()
}

// appkitBackend is the singleton macOS tray. It implements backend.
type appkitBackend struct {
	onExit func()
	app    objc.ID // NSApplication
	item   objc.ID // NSStatusItem
	button objc.ID // NSStatusBarButton (hosts the title + image)
	menu   objc.ID // root NSMenu
}

func newBackend() backend { return &appkitBackend{} }

func (b *appkitBackend) Run(onReady, onExit func()) {
	b.onExit = onExit
	// The main goroutine is already pinned to thread 0 (see init); AppKit requires it.

	if _, err := purego.Dlopen(
		"/System/Library/Frameworks/Cocoa.framework/Cocoa",
		purego.RTLD_GLOBAL|purego.RTLD_LAZY,
	); err != nil {
		slog.Error("macOS: cannot load the Cocoa framework", "err", err)
		os.Exit(1)
	}
	initDispatch()
	initObjC()

	b.app = objc.ID(classNSApplication).Send(sel.sharedApplication)
	b.app.Send(sel.setActivationPolicy, nsApplicationActivationPolicyAccessory)

	bar := objc.ID(classNSStatusBar).Send(sel.systemStatusBar)
	b.item = bar.Send(sel.statusItemWithLength, nsVariableStatusItemLength)
	b.button = b.item.Send(sel.button)

	b.menu = objc.ID(classNSMenu).Send(sel.alloc).Send(sel.init)
	b.item.Send(sel.setMenu, b.menu)

	if onReady != nil {
		onReady() // builds the menu + starts background work, on the main thread
	}

	b.app.Send(sel.activateIgnoringOtherApps, true)
	b.app.Send(sel.run) // blocks until terminate:
}

func (b *appkitBackend) Quit() {
	if b.onExit != nil {
		b.onExit()
	}
	app := b.app
	runOnMain(func() { app.Send(sel.terminate, objc.ID(0)) })
}

func (b *appkitBackend) SetTitle(s string) {
	button := b.button
	runOnMain(func() { button.Send(sel.setTitle, nsString(s)) })
}

func (b *appkitBackend) SetTooltip(s string) {
	button := b.button
	runOnMain(func() { button.Send(sel.setToolTip, nsString(s)) })
}

func (b *appkitBackend) SetIcon(data []byte) {
	button := b.button
	png := append([]byte(nil), data...) // own the bytes for the async closure
	runOnMain(func() {
		if img := nsImageFromPNG(png); img != 0 {
			button.Send(sel.setImage, img)
			button.Send(sel.setImagePosition, nsImageLeft)
		}
	})
}

func (b *appkitBackend) AddMenuItem(title, tooltip string) menuItem {
	it := newAppkitItem(title, false)
	b.menu.Send(sel.addItem, it.nsItem)
	return it
}

func (b *appkitBackend) AddMenuItemCheckbox(title, tooltip string, checked bool) menuItem {
	it := newAppkitItem(title, checked)
	b.menu.Send(sel.addItem, it.nsItem)
	return it
}

func (b *appkitBackend) AddSeparator() {
	b.menu.Send(sel.addItem, objc.ID(classNSMenuItem).Send(sel.separatorItem))
}

// appkitItem is one NSMenuItem. checked is a Go-side shadow so Checked() never has to
// read AppKit off the main thread; Check/Uncheck update it and dispatch the redraw.
type appkitItem struct {
	nsItem  objc.ID
	submenu objc.ID // 0 until the first child is added
	clicked chan struct{}
	checked bool
}

// newAppkitItem builds an NSMenuItem wired to fire our click handler. It must run on
// the main thread (menu construction does).
func newAppkitItem(title string, checked bool) *appkitItem {
	ns := objc.ID(classNSMenuItem).Send(sel.alloc)
	ns = ns.Send(sel.initItem, nsString(title), sel.handleClick, nsString(""))
	ns.Send(sel.setTarget, clickTarget)
	if checked {
		ns.Send(sel.setState, nsControlStateValueOn)
	}
	it := &appkitItem{nsItem: ns, clicked: make(chan struct{}, 1), checked: checked}
	handlersMu.Lock()
	handlers[ns] = it
	handlersMu.Unlock()
	return it
}

func (it *appkitItem) ClickedCh() <-chan struct{} { return it.clicked }
func (it *appkitItem) Checked() bool              { return it.checked }
func (it *appkitItem) Check()                     { it.setState(true) }
func (it *appkitItem) Uncheck()                   { it.setState(false) }

func (it *appkitItem) setState(on bool) {
	it.checked = on
	state := nsControlStateValueOff
	if on {
		state = nsControlStateValueOn
	}
	ns := it.nsItem
	runOnMain(func() { ns.Send(sel.setState, state) })
}

func (it *appkitItem) AddSubMenuItemCheckbox(title, tooltip string, checked bool) menuItem {
	if it.submenu == 0 {
		it.submenu = objc.ID(classNSMenu).Send(sel.alloc).Send(sel.init)
		it.nsItem.Send(sel.setSubmenu, it.submenu)
	}
	child := newAppkitItem(title, checked)
	it.submenu.Send(sel.addItem, child.nsItem)
	return child
}

// handlers maps an NSMenuItem to its Go item so the shared click handler knows which
// channel to fire. Built on the main thread during construction, read on the main
// thread during a click — the mutex guards against any future cross-thread use.
var (
	handlersMu sync.Mutex
	handlers   = map[objc.ID]*appkitItem{}
)

// handleClick is the Obj-C action method (-(void)handleClick:(id)sender) bridged to
// Go. It runs on the main thread and only does a non-blocking channel send, so the
// real work happens off the UI thread in the item's handler goroutine.
func handleClick(self objc.ID, _cmd objc.SEL, sender objc.ID) {
	handlersMu.Lock()
	it := handlers[sender]
	handlersMu.Unlock()
	if it == nil {
		return
	}
	select {
	case it.clicked <- struct{}{}:
	default: // a click is already queued; dropping a duplicate is harmless
	}
}

func initObjC() {
	classNSApplication = objc.GetClass("NSApplication")
	classNSStatusBar = objc.GetClass("NSStatusBar")
	classNSMenu = objc.GetClass("NSMenu")
	classNSMenuItem = objc.GetClass("NSMenuItem")
	classNSString = objc.GetClass("NSString")
	classNSData = objc.GetClass("NSData")
	classNSImage = objc.GetClass("NSImage")

	sel.alloc = objc.RegisterName("alloc")
	sel.init = objc.RegisterName("init")
	sel.new = objc.RegisterName("new")
	sel.sharedApplication = objc.RegisterName("sharedApplication")
	sel.setActivationPolicy = objc.RegisterName("setActivationPolicy:")
	sel.activateIgnoringOtherApps = objc.RegisterName("activateIgnoringOtherApps:")
	sel.run = objc.RegisterName("run")
	sel.terminate = objc.RegisterName("terminate:")
	sel.systemStatusBar = objc.RegisterName("systemStatusBar")
	sel.statusItemWithLength = objc.RegisterName("statusItemWithLength:")
	sel.button = objc.RegisterName("button")
	sel.setMenu = objc.RegisterName("setMenu:")
	sel.setTitle = objc.RegisterName("setTitle:")
	sel.setToolTip = objc.RegisterName("setToolTip:")
	sel.setImage = objc.RegisterName("setImage:")
	sel.setImagePosition = objc.RegisterName("setImagePosition:")
	sel.setSize = objc.RegisterName("setSize:")
	sel.stringWithUTF8 = objc.RegisterName("stringWithUTF8String:")
	sel.dataWithBytesLength = objc.RegisterName("dataWithBytes:length:")
	sel.initWithData = objc.RegisterName("initWithData:")
	sel.addItem = objc.RegisterName("addItem:")
	sel.separatorItem = objc.RegisterName("separatorItem")
	sel.initItem = objc.RegisterName("initWithTitle:action:keyEquivalent:")
	sel.setState = objc.RegisterName("setState:")
	sel.setSubmenu = objc.RegisterName("setSubmenu:")
	sel.setTarget = objc.RegisterName("setTarget:")
	sel.handleClick = objc.RegisterName("handleClick:")

	cls, err := objc.RegisterClass(
		"SpeedTickrMenuTarget",
		objc.GetClass("NSObject"),
		nil, nil,
		[]objc.MethodDef{{Cmd: sel.handleClick, Fn: handleClick}},
	)
	if err != nil {
		slog.Error("macOS: cannot register the menu click handler", "err", err)
		os.Exit(1)
	}
	clickTarget = objc.ID(cls).Send(sel.new)
}

// nsString makes an autoreleased NSString from a Go string. stringWithUTF8String:
// copies the bytes, so the Go string need not outlive the call.
func nsString(s string) objc.ID {
	return objc.ID(classNSString).Send(sel.stringWithUTF8, s+"\x00")
}

// nsSize mirrors Foundation's NSSize (two CGFloats), for setSize:.
type nsSize struct{ width, height float64 }

// nsImageFromPNG builds an NSImage from in-memory PNG bytes (colour preserved — not a
// template image — to match the green/blue glyph shown elsewhere). It is sized to
// 16×16 points, the menu-bar icon size systray uses, so a larger source PNG doesn't
// render oversized next to the speed text.
func nsImageFromPNG(png []byte) objc.ID {
	if len(png) == 0 {
		return 0
	}
	data := objc.ID(classNSData).Send(sel.dataWithBytesLength, unsafe.Pointer(&png[0]), len(png))
	runtime.KeepAlive(png) // hold the slice until dataWithBytes:length: has copied it
	img := objc.ID(classNSImage).Send(sel.alloc)
	img = img.Send(sel.initWithData, data)
	img.Send(sel.setSize, nsSize{16, 16})
	return img
}

// --- main-thread dispatch ---------------------------------------------------
//
// AppKit calls must happen on the main thread. From any goroutine, runOnMain hands a
// closure to the main run loop via libdispatch. dispatch_get_main_queue() is a macro,
// so we Dlsym the underlying _dispatch_main_q data symbol (its address is the queue).

type funcHolder struct{ fn func() }

var (
	dispatchMainQueue  uintptr
	dispatchAsyncF     func(queue uintptr, context unsafe.Pointer, work uintptr)
	dispatchTrampoline uintptr

	mainFuncsMu sync.Mutex
	mainFuncs   = map[*funcHolder]struct{}{} // keeps holders alive (and GC-pinned) until run
)

func initDispatch() {
	lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		slog.Error("macOS: cannot load libSystem", "err", err)
		os.Exit(1)
	}
	q, err := purego.Dlsym(lib, "_dispatch_main_q")
	if err != nil {
		slog.Error("macOS: cannot resolve the main dispatch queue", "err", err)
		os.Exit(1)
	}
	dispatchMainQueue = q
	purego.RegisterLibFunc(&dispatchAsyncF, lib, "dispatch_async_f")

	// One reused trampoline matching dispatch_function_t: void (*)(void *context).
	dispatchTrampoline = purego.NewCallback(func(ctx *funcHolder) {
		mainFuncsMu.Lock()
		_, ok := mainFuncs[ctx]
		delete(mainFuncs, ctx)
		mainFuncsMu.Unlock()
		if ok && ctx.fn != nil {
			ctx.fn()
		}
	})
}

// runOnMain schedules fn on the main thread. Safe from any goroutine; runs async.
func runOnMain(fn func()) {
	h := &funcHolder{fn: fn}
	mainFuncsMu.Lock()
	mainFuncs[h] = struct{}{}
	mainFuncsMu.Unlock()
	dispatchAsyncF(dispatchMainQueue, unsafe.Pointer(h), dispatchTrampoline)
}
