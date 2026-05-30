package csharp_test

// ---------------------------------------------------------------------------
// Desktop Native — WPF / WinForms / Uno — Process / Native
// ---------------------------------------------------------------------------

import "testing"

func TestDesktopNativeIPCNamedPipe(t *testing.T) {
	src := `
using System.IO.Pipes;

var server = new NamedPipeServerStream("mypipe", PipeDirection.InOut);
await server.WaitForConnectionAsync();
`
	ents := extract(t, "custom_csharp_desktop_native", fi("PipeServer.cs", "csharp", src))
	foundIPC := false
	for _, e := range ents {
		if e.Subtype == "ipc_extraction" {
			foundIPC = true
			break
		}
	}
	if !foundIPC {
		t.Error("expected ipc_extraction from NamedPipeServerStream")
	}
}

func TestDesktopNativeIPCMemoryMapped(t *testing.T) {
	src := `
using System.IO.MemoryMappedFiles;

var mmf = MemoryMappedFile.CreateOrOpen("shared_mem", 1024);
var accessor = mmf.CreateViewAccessor();
`
	ents := extract(t, "custom_csharp_desktop_native", fi("SharedMem.cs", "csharp", src))
	foundIPC := false
	for _, e := range ents {
		if e.Subtype == "ipc_extraction" {
			foundIPC = true
			break
		}
	}
	if !foundIPC {
		t.Error("expected ipc_extraction from MemoryMappedFile")
	}
}

func TestDesktopNativeIPCDispatcher(t *testing.T) {
	src := `
Application.Current.Dispatcher.Invoke(() =>
{
    StatusLabel.Text = "Updated from background thread";
});
`
	ents := extract(t, "custom_csharp_desktop_native", fi("MainWindow.xaml.cs", "csharp", src))
	foundIPC := false
	for _, e := range ents {
		if e.Subtype == "ipc_extraction" {
			foundIPC = true
			break
		}
	}
	if !foundIPC {
		t.Error("expected ipc_extraction from Dispatcher.Invoke")
	}
}

func TestDesktopNativeMainRendererAppClass(t *testing.T) {
	src := `
public partial class App : Application
{
    protected override void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);
        new MainWindow().Show();
    }
}
`
	ents := extract(t, "custom_csharp_desktop_native", fi("App.xaml.cs", "csharp", src))
	if !containsEntity(ents, "SCOPE.Component", "app:App") {
		t.Error("expected app:App from Application subclass")
	}
}

func TestDesktopNativeMainRendererWinForms(t *testing.T) {
	src := `
static class Program
{
    [STAThread]
    static void Main()
    {
        ApplicationConfiguration.Initialize();
        Application.Run(new MainForm());
    }
}
`
	ents := extract(t, "custom_csharp_desktop_native", fi("Program.cs", "csharp", src))
	foundMain := false
	foundRun := false
	for _, e := range ents {
		if e.Subtype == "main_renderer_split" {
			if e.Kind == "SCOPE.Component" {
				foundMain = true
			}
			foundRun = true
		}
	}
	if !foundMain {
		t.Error("expected main_renderer_split from static Main()")
	}
	if !foundRun {
		t.Error("expected main_renderer_split from Application.Run()")
	}
}

func TestDesktopNativeMainRendererInitializeComponent(t *testing.T) {
	src := `
public partial class MainWindow : Window
{
    public MainWindow()
    {
        InitializeComponent();
    }
}
`
	ents := extract(t, "custom_csharp_desktop_native", fi("MainWindow.xaml.cs", "csharp", src))
	foundRenderer := false
	for _, e := range ents {
		if e.Subtype == "main_renderer_split" {
			foundRenderer = true
			break
		}
	}
	if !foundRenderer {
		t.Error("expected main_renderer_split from InitializeComponent()")
	}
}

func TestDesktopNativeNativeDllImport(t *testing.T) {
	src := `
using System.Runtime.InteropServices;

public static class Win32
{
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    public static extern int MessageBox(IntPtr hWnd, string text, string caption, int type);

    [DllImport("kernel32.dll")]
    public static extern bool CloseHandle(IntPtr hObject);
}
`
	ents := extract(t, "custom_csharp_desktop_native", fi("Win32.cs", "csharp", src))
	foundUser32 := false
	foundKernel32 := false
	for _, e := range ents {
		if e.Subtype == "native_module_imports" && e.Name == "native:dll:user32.dll" {
			foundUser32 = true
		}
		if e.Subtype == "native_module_imports" && e.Name == "native:dll:kernel32.dll" {
			foundKernel32 = true
		}
	}
	if !foundUser32 {
		t.Error("expected native:dll:user32.dll from DllImport")
	}
	if !foundKernel32 {
		t.Error("expected native:dll:kernel32.dll from DllImport")
	}
}

func TestDesktopNativeNativeComInterop(t *testing.T) {
	src := `
using System.Runtime.InteropServices;

[ComImport]
[Guid("0000010C-0000-0000-C000-000000000046")]
[InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
public interface IPersist
{
    void GetClassID(out Guid classId);
}
`
	ents := extract(t, "custom_csharp_desktop_native", fi("IPersist.cs", "csharp", src))
	foundCom := false
	for _, e := range ents {
		if e.Subtype == "native_module_imports" {
			foundCom = true
			break
		}
	}
	if !foundCom {
		t.Error("expected native_module_imports from COM interop attributes")
	}
}

func TestDesktopNativeNativeUnsafe(t *testing.T) {
	src := `
public static unsafe int Sum(int* arr, int length)
{
    int sum = 0;
    for (int i = 0; i < length; i++)
        sum += arr[i];
    return sum;
}
`
	ents := extract(t, "custom_csharp_desktop_native", fi("NativeOps.cs", "csharp", src))
	foundNative := false
	for _, e := range ents {
		if e.Subtype == "native_module_imports" {
			foundNative = true
			break
		}
	}
	if !foundNative {
		t.Error("expected native_module_imports from unsafe keyword")
	}
}

func TestDesktopNativeNativeLibrary(t *testing.T) {
	src := `
using System.Runtime.InteropServices;

var lib = NativeLibrary.Load("mylibrary.so");
var funcPtr = NativeLibrary.GetExport(lib, "my_function");
`
	ents := extract(t, "custom_csharp_desktop_native", fi("NativeLib.cs", "csharp", src))
	foundNative := false
	for _, e := range ents {
		if e.Subtype == "native_module_imports" {
			foundNative = true
			break
		}
	}
	if !foundNative {
		t.Error("expected native_module_imports from NativeLibrary.Load/GetExport")
	}
}

func TestDesktopNativeNoMatch(t *testing.T) {
	src := `namespace MyApp { class Helper { string GetName() { return "test"; } } }`
	ents := extract(t, "custom_csharp_desktop_native", fi("Helper.cs", "csharp", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}
