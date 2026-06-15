// Package csharp — Desktop native extractor for WPF, WinForms, and Uno C# source files.
//
// Covers the nine missing cells for:
//   - lang.csharp.framework.wpf
//   - lang.csharp.framework.winforms
//   - lang.csharp.framework.uno
//
// Capabilities:
//
//	Process/ipc_extraction:
//	  Named pipes (NamedPipeServerStream / NamedPipeClientStream),
//	  memory-mapped files (MemoryMappedFile), WCF service host
//	  (ServiceHost / ChannelFactory<T>), and
//	  Dispatcher.Invoke / Application.Current.Dispatcher usage
//	  emitted as SCOPE.Pattern/ipc_extraction.
//
//	Process/main_renderer_split:
//	  App.xaml.cs Application subclasses, Application.Run() calls,
//	  and static Main() / async Main() entry points in WinForms/WPF/Uno
//	  emitted as SCOPE.Component/main_renderer_split.
//
//	Native/native_module_imports:
//	  [DllImport("...")] P/Invoke declarations,
//	  unsafe blocks (pointer-level native interop),
//	  COM interop ([ComImport] / [Guid] / [InterfaceType]) markers,
//	  Windows.Win32 / PInvoke.* / CsWin32 usage patterns
//	  emitted as SCOPE.Pattern/native_module_imports.
//
// Registration key: "custom_csharp_desktop_native"
// Issue #3261.
package csharp

import (
	"context"
	"regexp"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_csharp_desktop_native", &desktopNativeExtractor{})
}

type desktopNativeExtractor struct{}

func (e *desktopNativeExtractor) Language() string { return "custom_csharp_desktop_native" }

// ---------------------------------------------------------------------------
// Regex catalog
// ---------------------------------------------------------------------------

var (
	// Process/ipc_extraction -------------------------------------------------

	// NamedPipeServerStream / NamedPipeClientStream — named pipe IPC
	reDNNamedPipe = regexp.MustCompile(
		`new\s+(NamedPipeServerStream|NamedPipeClientStream)\s*\(`,
	)

	// MemoryMappedFile.CreateNew / OpenExisting — shared-memory IPC
	reDNMemoryMapped = regexp.MustCompile(
		`MemoryMappedFile\.(CreateNew|CreateOrOpen|OpenExisting)\s*\(`,
	)

	// ServiceHost / ChannelFactory<T> — WCF IPC
	reDNWCFHost = regexp.MustCompile(
		`new\s+(ServiceHost|ChannelFactory\s*<[^>]+>)\s*\(`,
	)

	// Dispatcher.Invoke / InvokeAsync — UI-thread dispatch (cross-process/thread)
	reDNDispatcherInvoke = regexp.MustCompile(
		`(?:Dispatcher|Application\.Current\.Dispatcher)\.(?:Invoke|InvokeAsync|BeginInvoke)\s*\(`,
	)

	// Process.Start / Process.GetProcessById — child-process IPC
	reDNProcess = regexp.MustCompile(
		`System\.Diagnostics\.Process\.(Start|GetProcessById|GetProcessesByName)\s*\(`,
	)

	// EventWaitHandle / Mutex (named) — cross-process sync primitives
	reDNNamedSync = regexp.MustCompile(
		`new\s+(EventWaitHandle|Mutex|Semaphore)\s*\([^)]*,\s*["'][^"']+["']`,
	)

	// Process/main_renderer_split --------------------------------------------

	// class App : Application — WPF/Uno application class
	reDNAppClass = regexp.MustCompile(
		`(?m)class\s+(\w+)\s*:\s*(?:Application|PrismApplication|MvvmLightApplication)\b`,
	)

	// Application.Run(...) — WinForms entry point
	reDNApplicationRun = regexp.MustCompile(
		`Application\.Run\s*\(`,
	)

	// static Main() / static async Task Main() — program entry point
	reDNMainMethod = regexp.MustCompile(
		`(?m)static\s+(?:async\s+)?(?:void|Task|int)\s+Main\s*\(`,
	)

	// InitializeComponent() — XAML partial class wiring (renderer side)
	reDNInitializeComponent = regexp.MustCompile(
		`(?m)^\s*InitializeComponent\s*\(\s*\)\s*;`,
	)

	// CreateWindow / Window.Show — WinForms window creation
	reDNWindowShow = regexp.MustCompile(
		`new\s+(\w+Form|\w+Window)\s*\(\s*\)`,
	)

	// Uno: App.CreateWindow / CreateRootFrame
	reDNUnoCreate = regexp.MustCompile(
		`(?:CreateWindow|CreateRootFrame|InitializeComponent)\s*\(`,
	)

	// Native/native_module_imports -------------------------------------------

	// [DllImport("libname")] — P/Invoke
	reDNDllImport = regexp.MustCompile(
		`\[DllImport\s*\(\s*["']([^"']+)["']`,
	)

	// unsafe keyword — pointer-level native interop
	reDNUnsafe = regexp.MustCompile(
		`\bunsafe\s+(?:static\s+)?(?:void|class|struct|bool|int|byte\*|\w+\*|\w+)\b`,
	)

	// [ComImport] / [InterfaceType(ComInterfaceType.*)] — COM interop
	reDNComImport = regexp.MustCompile(
		`\[(?:ComImport|InterfaceType|Guid)\s*(?:\([^)]*\))?\s*\]`,
	)

	// using Windows.Win32 / using PInvoke. — CsWin32 / Community.Toolkit
	reDNWin32Import = regexp.MustCompile(
		`(?m)^\s*using\s+(Windows\.Win32\.|PInvoke\.|CsWin32\.)\w`,
	)

	// Marshal.GetDelegateForFunctionPointer / AllocHGlobal — manual interop
	reDNMarshalInterop = regexp.MustCompile(
		`Marshal\.(GetDelegateForFunctionPointer|AllocHGlobal|FreeHGlobal|PtrToStructure|StructureToPtr)\b`,
	)

	// NativeLibrary.Load("...") — .NET 5+ native library loading
	reDNNativeLibrary = regexp.MustCompile(
		`NativeLibrary\.(Load|TryLoad|GetExport)\s*\(`,
	)
)

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func (e *desktopNativeExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/csharp")
	_, span := tracer.Start(ctx, "indexer.desktop_native_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "desktop"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 {
		return nil, nil
	}
	if file.Language != "csharp" {
		return nil, nil
	}

	src := string(file.Content)
	var entities []types.EntityRecord
	seen := make(map[string]bool)

	add := func(ent types.EntityRecord) {
		key := ent.Kind + ":" + ent.Subtype + ":" + ent.Name
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	// -------------------------------------------------------------------------
	// Process/ipc_extraction
	// -------------------------------------------------------------------------

	for _, m := range reDNNamedPipe.FindAllStringSubmatchIndex(src, -1) {
		pipeType := src[m[2]:m[3]]
		ent := makeEntity("ipc:named_pipe:"+pipeType+":"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "ipc_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_NAMED_PIPE",
			"pipe_type", pipeType)
		add(ent)
	}

	for _, m := range reDNMemoryMapped.FindAllStringSubmatchIndex(src, -1) {
		method := src[m[2]:m[3]]
		ent := makeEntity("ipc:mmap:"+method+":"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "ipc_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_MEMORY_MAPPED",
			"method", method)
		add(ent)
	}

	for _, m := range reDNWCFHost.FindAllStringSubmatchIndex(src, -1) {
		hostType := src[m[2]:m[3]]
		ent := makeEntity("ipc:wcf:"+hostType+":"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "ipc_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_WCF_HOST",
			"host_type", hostType)
		add(ent)
	}

	for _, m := range reDNDispatcherInvoke.FindAllStringIndex(src, -1) {
		ent := makeEntity("ipc:dispatcher:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "ipc_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_DISPATCHER_INVOKE")
		add(ent)
	}

	for _, m := range reDNProcess.FindAllStringSubmatchIndex(src, -1) {
		method := src[m[2]:m[3]]
		ent := makeEntity("ipc:process:"+method+":"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "ipc_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_PROCESS_CALL",
			"method", method)
		add(ent)
	}

	for _, m := range reDNNamedSync.FindAllStringSubmatchIndex(src, -1) {
		syncType := src[m[2]:m[3]]
		ent := makeEntity("ipc:named_sync:"+syncType+":"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "ipc_extraction", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_NAMED_SYNC",
			"sync_type", syncType)
		add(ent)
	}

	// -------------------------------------------------------------------------
	// Process/main_renderer_split
	// -------------------------------------------------------------------------

	for _, m := range reDNAppClass.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity("app:"+name, "SCOPE.Component", "main_renderer_split",
			file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_APP_CLASS",
			"class_name", name)
		add(ent)
	}

	for _, m := range reDNApplicationRun.FindAllStringIndex(src, -1) {
		ent := makeEntity("app:Run:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Component", "main_renderer_split", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_APPLICATION_RUN")
		add(ent)
	}

	for _, m := range reDNMainMethod.FindAllStringIndex(src, -1) {
		ent := makeEntity("app:Main:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Component", "main_renderer_split", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_MAIN_METHOD")
		add(ent)
	}

	for _, m := range reDNInitializeComponent.FindAllStringIndex(src, -1) {
		ent := makeEntity("renderer:InitializeComponent:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Component", "main_renderer_split", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_INITIALIZE_COMPONENT")
		add(ent)
	}

	for _, m := range reDNWindowShow.FindAllStringSubmatchIndex(src, -1) {
		windowName := src[m[2]:m[3]]
		ent := makeEntity("renderer:"+windowName+":"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Component", "main_renderer_split", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_WINDOW_SHOW",
			"window_class", windowName)
		add(ent)
	}

	// -------------------------------------------------------------------------
	// Native/native_module_imports
	// -------------------------------------------------------------------------

	for _, m := range reDNDllImport.FindAllStringSubmatchIndex(src, -1) {
		libName := src[m[2]:m[3]]
		ent := makeEntity("native:dll:"+libName, "SCOPE.Pattern", "native_module_imports",
			file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_DLL_IMPORT",
			"library", libName)
		add(ent)
	}

	for _, m := range reDNUnsafe.FindAllStringIndex(src, -1) {
		ent := makeEntity("native:unsafe:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "native_module_imports", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_UNSAFE_BLOCK")
		add(ent)
	}

	for _, m := range reDNComImport.FindAllStringIndex(src, -1) {
		ent := makeEntity("native:com:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "native_module_imports", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_COM_IMPORT")
		add(ent)
	}

	for _, m := range reDNWin32Import.FindAllStringIndex(src, -1) {
		ent := makeEntity("native:win32:"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "native_module_imports", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_WIN32_IMPORT")
		add(ent)
	}

	for _, m := range reDNMarshalInterop.FindAllStringSubmatchIndex(src, -1) {
		method := src[m[2]:m[3]]
		ent := makeEntity("native:marshal:"+method+":"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "native_module_imports", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_MARSHAL_INTEROP",
			"method", method)
		add(ent)
	}

	for _, m := range reDNNativeLibrary.FindAllStringSubmatchIndex(src, -1) {
		method := src[m[2]:m[3]]
		ent := makeEntity("native:native_lib:"+method+":"+file.Path+":"+itoa(lineOf(src, m[0])),
			"SCOPE.Pattern", "native_module_imports", file.Path, "csharp", lineOf(src, m[0]))
		setProps(&ent, "framework", "desktop", "provenance", "INFERRED_FROM_NATIVE_LIBRARY",
			"method", method)
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}
