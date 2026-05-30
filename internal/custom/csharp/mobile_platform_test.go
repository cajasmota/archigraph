package csharp_test

// ---------------------------------------------------------------------------
// Mobile Platform — MAUI / Xamarin — Structure / Navigation / Lifecycle / Platform / Native
// ---------------------------------------------------------------------------

import "testing"

func TestMobilePlatformContextExtraction(t *testing.T) {
	src := `
using Xamarin.Forms;

public class ProductListPage : ContentPage
{
    public ProductListPage()
    {
        var service = DependencyService.Get<IProductService>();
        var ctx = Application.Current;
    }
}
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("ProductListPage.cs", "csharp", src))
	foundCtx := false
	for _, e := range ents {
		if e.Subtype == "context_extraction" {
			foundCtx = true
			break
		}
	}
	if !foundCtx {
		t.Error("expected context_extraction from DependencyService.Get or Application.Current")
	}
}

func TestMobilePlatformDeepLinkShell(t *testing.T) {
	src := `
await Shell.Current.GoToAsync("//products/detail");
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("AppShell.xaml.cs", "csharp", src))
	foundDeep := false
	for _, e := range ents {
		if e.Subtype == "deep_link_extraction" {
			foundDeep = true
			break
		}
	}
	if !foundDeep {
		t.Error("expected deep_link_extraction from Shell.GoToAsync")
	}
}

func TestMobilePlatformDeepLinkQueryProperty(t *testing.T) {
	src := `
[QueryProperty("ProductId", "id")]
public partial class ProductDetailPage : ContentPage
{
}
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("ProductDetailPage.cs", "csharp", src))
	foundDeep := false
	for _, e := range ents {
		if e.Subtype == "deep_link_extraction" {
			foundDeep = true
			break
		}
	}
	if !foundDeep {
		t.Error("expected deep_link_extraction from [QueryProperty]")
	}
}

func TestMobilePlatformNavigationExtraction(t *testing.T) {
	src := `
public async Task NavigateToDetail(int id)
{
    await Navigation.PushAsync(new ProductDetailPage(id));
}

public async Task GoBack()
{
    await Navigation.PopAsync();
}
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("ProductList.xaml.cs", "csharp", src))
	foundNav := false
	for _, e := range ents {
		if e.Subtype == "navigation_extraction" {
			foundNav = true
			break
		}
	}
	if !foundNav {
		t.Error("expected navigation_extraction from Navigation.PushAsync/PopAsync")
	}
}

func TestMobilePlatformScreenDetection(t *testing.T) {
	src := `
public partial class MainPage : ContentPage
{
    public MainPage() { InitializeComponent(); }
}
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("MainPage.xaml.cs", "csharp", src))
	if !containsEntity(ents, "SCOPE.UIComponent", "screen:MainPage") {
		t.Error("expected screen:MainPage from ContentPage subclass")
	}
}

func TestMobilePlatformScreenDetectionShell(t *testing.T) {
	src := `
public partial class AppShell : Shell
{
    public AppShell() { InitializeComponent(); }
}
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("AppShell.xaml.cs", "csharp", src))
	if !containsEntity(ents, "SCOPE.UIComponent", "screen:shell:AppShell") {
		t.Error("expected screen:shell:AppShell from Shell subclass")
	}
}

func TestMobilePlatformPlatformBranchingDeviceInfo(t *testing.T) {
	src := `
if (DeviceInfo.Platform == DevicePlatform.Android)
{
    // Android-specific code
}
else if (DeviceInfo.Platform == DevicePlatform.iOS)
{
    // iOS-specific code
}
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("PlatformHelper.cs", "csharp", src))
	foundBranch := false
	for _, e := range ents {
		if e.Subtype == "platform_branching" {
			foundBranch = true
			break
		}
	}
	if !foundBranch {
		t.Error("expected platform_branching from DeviceInfo.Platform comparison")
	}
}

func TestMobilePlatformPlatformBranchingPreprocessor(t *testing.T) {
	src := `
#if ANDROID
using Android.Widget;
#elif IOS
using UIKit;
#endif
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("PlatformSpecific.cs", "csharp", src))
	foundBranch := false
	for _, e := range ents {
		if e.Subtype == "platform_branching" {
			foundBranch = true
			break
		}
	}
	if !foundBranch {
		t.Error("expected platform_branching from #if ANDROID/#if IOS")
	}
}

func TestMobilePlatformNativeModuleDllImport(t *testing.T) {
	src := `
public static class NativeMethods
{
    [DllImport("user32.dll")]
    public static extern int MessageBox(IntPtr hWnd, string text, string caption, int type);
}
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("NativeMethods.cs", "csharp", src))
	if !containsEntity(ents, "SCOPE.Pattern", "native:dll:user32.dll") {
		t.Error("expected native:dll:user32.dll from [DllImport]")
	}
}

func TestMobilePlatformNativeModuleDependency(t *testing.T) {
	src := `
[assembly: Dependency(typeof(CameraService))]
namespace MyApp.Droid
{
    public class CameraService : ICameraService
    {
    }
}
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("CameraService.cs", "csharp", src))
	foundNative := false
	for _, e := range ents {
		if e.Subtype == "native_module_imports" {
			foundNative = true
			break
		}
	}
	if !foundNative {
		t.Error("expected native_module_imports from [assembly: Dependency]")
	}
}

func TestMobilePlatformEnumExtraction(t *testing.T) {
	src := `
public enum OrderStatus
{
    Pending,
    Processing,
    Shipped,
    Delivered,
    Cancelled
}
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("OrderStatus.cs", "csharp", src))
	if !containsEntity(ents, "SCOPE.Schema", "enum:OrderStatus") {
		t.Error("expected enum:OrderStatus from enum declaration")
	}
}

func TestMobilePlatformInterfaceExtraction(t *testing.T) {
	src := `
public interface IProductService
{
    Task<List<Product>> GetProductsAsync();
    Task<Product> GetProductByIdAsync(int id);
}
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("IProductService.cs", "csharp", src))
	if !containsEntity(ents, "SCOPE.Component", "interface:IProductService") {
		t.Error("expected interface:IProductService from interface declaration")
	}
}

func TestMobilePlatformLifecycleMAUI(t *testing.T) {
	src := `
public static class MauiProgram
{
    public static MauiApp CreateMauiApp()
    {
        var builder = MauiApp.CreateBuilder();
        builder.UseMauiApp<App>();
        return builder.Build();
    }
}
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("MauiProgram.cs", "csharp", src))
	foundLifecycle := false
	for _, e := range ents {
		if e.Subtype == "state_setter_emission" {
			foundLifecycle = true
			break
		}
	}
	if !foundLifecycle {
		t.Error("expected state_setter_emission from CreateMauiApp")
	}
}

func TestMobilePlatformLifecycleXamarin(t *testing.T) {
	src := `
public partial class App : Application
{
    protected override void OnStart() { }
    protected override void OnSleep() { }
    protected override void OnResume() { }
}
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("App.xaml.cs", "csharp", src))
	foundCount := 0
	for _, e := range ents {
		if e.Subtype == "state_setter_emission" {
			foundCount++
		}
	}
	if foundCount < 3 {
		t.Errorf("expected at least 3 state_setter_emission entities (OnStart/OnSleep/OnResume), got %d", foundCount)
	}
}

func TestMobilePlatformINPC(t *testing.T) {
	src := `
public class ProductViewModel : INotifyPropertyChanged
{
    private string _name;
    public string Name
    {
        get => _name;
        set
        {
            _name = value;
            PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(nameof(Name)));
        }
    }
}
`
	ents := extract(t, "custom_csharp_mobile_platform", fi("ProductViewModel.cs", "csharp", src))
	foundLifecycle := false
	for _, e := range ents {
		if e.Subtype == "state_setter_emission" {
			foundLifecycle = true
			break
		}
	}
	if !foundLifecycle {
		t.Error("expected state_setter_emission from PropertyChanged invocation")
	}
}

func TestMobilePlatformNoMatch(t *testing.T) {
	src := `namespace MyApp { class Helper { } }`
	ents := extract(t, "custom_csharp_mobile_platform", fi("Helper.cs", "csharp", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}
