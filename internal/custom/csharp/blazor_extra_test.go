package csharp_test

// ---------------------------------------------------------------------------
// Blazor Extra — Structure / Navigation / Lifecycle
// ---------------------------------------------------------------------------

import "testing"

func TestBlazorExtraComponentExtraction(t *testing.T) {
	src := `
@page "/counter"
@page "/counter/{id:int}"

@code {
    private int count = 0;
}
`
	ents := extract(t, "custom_csharp_blazor_extra", fi("Counter.razor", "csharp", src))
	foundComp := false
	for _, e := range ents {
		if e.Subtype == "component_extraction" {
			foundComp = true
			break
		}
	}
	if !foundComp {
		t.Error("expected component_extraction entity from @page directive")
	}
}

func TestBlazorExtraComponentBaseClass(t *testing.T) {
	src := `
public class MyWidget : ComponentBase
{
    [Parameter]
    public string Title { get; set; }
}
`
	ents := extract(t, "custom_csharp_blazor_extra", fi("MyWidget.razor.cs", "csharp", src))
	if !containsEntity(ents, "SCOPE.UIComponent", "MyWidget") {
		t.Error("expected MyWidget UIComponent from ComponentBase subclass")
	}
}

func TestBlazorExtraContextExtraction(t *testing.T) {
	src := `
@inject IAuthService AuthService
@inject NavigationManager Nav
`
	ents := extract(t, "custom_csharp_blazor_extra", fi("Page.razor", "csharp", src))
	foundCtx := false
	for _, e := range ents {
		if e.Subtype == "context_extraction" {
			foundCtx = true
			break
		}
	}
	if !foundCtx {
		t.Error("expected context_extraction entity from @inject")
	}
}

func TestBlazorExtraCascadingContext(t *testing.T) {
	src := `
@code {
    [CascadingParameter]
    public AppState State { get; set; }
}
`
	ents := extract(t, "custom_csharp_blazor_extra", fi("Child.razor", "csharp", src))
	foundCtx := false
	for _, e := range ents {
		if e.Subtype == "context_extraction" {
			foundCtx = true
			break
		}
	}
	if !foundCtx {
		t.Error("expected context_extraction from [CascadingParameter]")
	}
}

func TestBlazorExtraRouterPattern(t *testing.T) {
	src := `
@page "/products"
@page "/products/{id}"

@code {
    private void GoToHome()
    {
        Nav.NavigateTo("/home");
    }
}
`
	ents := extract(t, "custom_csharp_blazor_extra", fi("Products.razor", "csharp", src))
	foundRoute := false
	foundNav := false
	for _, e := range ents {
		if e.Subtype == "router_pattern" && e.Kind == "SCOPE.Operation" {
			foundRoute = true
		}
		if e.Subtype == "router_pattern" && e.Kind == "SCOPE.Pattern" {
			foundNav = true
		}
	}
	if !foundRoute {
		t.Error("expected router_pattern operation from @page")
	}
	if !foundNav {
		t.Error("expected router_pattern pattern from NavigateTo")
	}
}

func TestBlazorExtraNavLink(t *testing.T) {
	src := `
<NavLink href="/orders" Match="NavLinkMatch.All">Orders</NavLink>
`
	ents := extract(t, "custom_csharp_blazor_extra", fi("Nav.razor", "csharp", src))
	foundNav := false
	for _, e := range ents {
		if e.Subtype == "router_pattern" && e.Kind == "SCOPE.Pattern" {
			foundNav = true
			break
		}
	}
	if !foundNav {
		t.Error("expected router_pattern from NavLink href")
	}
}

func TestBlazorExtraLifecycle(t *testing.T) {
	src := `
@code {
    protected override async Task OnInitializedAsync()
    {
        data = await Http.GetFromJsonAsync<List<Item>>("/api/items");
    }

    protected override void OnParametersSet()
    {
        StateHasChanged();
    }
}
`
	ents := extract(t, "custom_csharp_blazor_extra", fi("Page.razor", "csharp", src))
	foundLifecycle := false
	foundStateChange := false
	for _, e := range ents {
		if e.Subtype == "state_setter_emission" && e.Kind == "SCOPE.Operation" {
			foundLifecycle = true
		}
		if e.Subtype == "state_setter_emission" && e.Kind == "SCOPE.Pattern" {
			foundStateChange = true
		}
	}
	if !foundLifecycle {
		t.Error("expected state_setter_emission from OnInitializedAsync/OnParametersSet")
	}
	if !foundStateChange {
		t.Error("expected state_setter_emission from StateHasChanged call")
	}
}

func TestBlazorExtraLayoutComponent(t *testing.T) {
	src := `
public class MainLayout : LayoutComponentBase
{
    protected override void BuildRenderTree(RenderTreeBuilder builder) { }
}
`
	ents := extract(t, "custom_csharp_blazor_extra", fi("MainLayout.razor.cs", "csharp", src))
	if !containsEntity(ents, "SCOPE.UIComponent", "MainLayout") {
		t.Error("expected MainLayout UIComponent from LayoutComponentBase")
	}
}

func TestBlazorExtraNoMatch(t *testing.T) {
	src := `namespace MyApp { class Helper { } }`
	ents := extract(t, "custom_csharp_blazor_extra", fi("Helper.cs", "csharp", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities from plain class, got %d", len(ents))
	}
}
