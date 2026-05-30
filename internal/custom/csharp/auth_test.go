package csharp_test

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Auth — auth_coverage
// ---------------------------------------------------------------------------

func TestAuthAuthorizeAttribute(t *testing.T) {
	src := `
[ApiController]
[Authorize]
public class SecureController : ControllerBase
{
    [HttpGet]
    public IActionResult Get() => Ok();
}
`
	ents := extract(t, "custom_csharp_auth", fi("SecureController.cs", "csharp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "auth_coverage" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth_coverage SCOPE.Pattern from [Authorize]")
	}
}

func TestAuthAuthorizeRoles(t *testing.T) {
	src := `[Authorize(Roles = "Admin,Manager")]`
	ents := extract(t, "custom_csharp_auth", fi("AdminController.cs", "csharp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "auth_coverage" && e.Name == "auth:Authorize:roles:Admin,Manager" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth:Authorize:roles:Admin,Manager entity")
	}
}

func TestAuthAuthorizePolicy(t *testing.T) {
	src := `[Authorize(Policy = "RequireAdminRole")]`
	ents := extract(t, "custom_csharp_auth", fi("Controller.cs", "csharp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "auth_coverage" && e.Name == "auth:Authorize:policy:RequireAdminRole" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth:Authorize:policy:RequireAdminRole entity")
	}
}

func TestAuthAuthorizePositional(t *testing.T) {
	src := `[Authorize("AdminOnly")]`
	ents := extract(t, "custom_csharp_auth", fi("Controller.cs", "csharp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "auth_coverage" && e.Name == "auth:Authorize:policy:AdminOnly" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth:Authorize:policy:AdminOnly from positional [Authorize(\"AdminOnly\")]")
	}
}

func TestAuthAllowAnonymous(t *testing.T) {
	src := `
[AllowAnonymous]
public IActionResult Login() => Ok();
`
	ents := extract(t, "custom_csharp_auth", fi("AuthController.cs", "csharp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "auth_coverage" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth_coverage entity from [AllowAnonymous]")
	}
}

func TestAuthRequireAuthorization(t *testing.T) {
	src := `
app.MapGet("/secure", () => "secret").RequireAuthorization();
app.MapPost("/admin", Handler).RequireAuthorization("AdminPolicy");
`
	ents := extract(t, "custom_csharp_auth", fi("Program.cs", "csharp", src))
	count := 0
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "auth_coverage" {
			count++
		}
	}
	if count < 2 {
		t.Errorf("expected at least 2 auth_coverage entities from RequireAuthorization, got %d", count)
	}
}

func TestAuthAddAuthorization(t *testing.T) {
	src := `
builder.Services.AddAuthorization(options =>
{
    options.AddPolicy("AdminOnly", policy => policy.RequireRole("Admin"));
    options.AddPolicy("PremiumUser", policy => policy.RequireClaim("subscription", "premium"));
});
`
	ents := extract(t, "custom_csharp_auth", fi("Program.cs", "csharp", src))
	addAuthFound := false
	policyFound := 0
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "auth_coverage" {
			if e.Name == "auth:AddAuthorization:Program.cs" {
				addAuthFound = true
			}
			if e.Name == "auth:AddPolicy:AdminOnly" || e.Name == "auth:AddPolicy:PremiumUser" {
				policyFound++
			}
		}
	}
	if !addAuthFound {
		t.Error("expected auth:AddAuthorization:Program.cs entity")
	}
	if policyFound < 2 {
		t.Errorf("expected 2 AddPolicy entities, got %d", policyFound)
	}
}

func TestAuthNoMatch(t *testing.T) {
	src := `namespace App { class Helper { public void DoWork() {} } }`
	ents := extract(t, "custom_csharp_auth", fi("Helper.cs", "csharp", src))
	if len(ents) != 0 {
		t.Errorf("expected 0 auth entities, got %d", len(ents))
	}
}

func TestAuthWrongLanguageSkipped(t *testing.T) {
	src := `[Authorize]`
	ents := extract(t, "custom_csharp_auth", fi("file.java", "java", src))
	if len(ents) != 0 {
		t.Errorf("expected 0 entities for non-csharp language, got %d", len(ents))
	}
}

// ---------------------------------------------------------------------------
// csAuth: JWT Bearer (issue #3380)
// ---------------------------------------------------------------------------

func TestAuthJwtBearerAddAuthentication(t *testing.T) {
	src := `
builder.Services.AddAuthentication(JwtBearerDefaults.AuthenticationScheme)
    .AddJwtBearer(options =>
    {
        options.TokenValidationParameters = new TokenValidationParameters
        {
            ValidIssuer = "https://auth.example.com",
            ValidAudience = "my-api",
            IssuerSigningKey = new SymmetricSecurityKey(Encoding.UTF8.GetBytes(key)),
        };
    });
`
	ents := extract(t, "custom_csharp_auth", fi("Program.cs", "csharp", src))

	addAuthFound := false
	addJwtFound := false
	tokenValFound := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "auth_coverage" {
			switch e.Name {
			case "auth:AddAuthentication:Program.cs":
				addAuthFound = true
			case "auth:AddJwtBearer:Program.cs":
				addJwtFound = true
			case "auth:TokenValidationParameters:Program.cs":
				tokenValFound = true
			}
		}
	}
	if !addAuthFound {
		t.Error("expected auth:AddAuthentication:Program.cs entity")
	}
	if !addJwtFound {
		t.Error("expected auth:AddJwtBearer:Program.cs entity")
	}
	if !tokenValFound {
		t.Error("expected auth:TokenValidationParameters:Program.cs entity")
	}
}

func TestAuthJwtBearerIssuerAudience(t *testing.T) {
	src := `
options.TokenValidationParameters = new TokenValidationParameters
{
    ValidIssuer = "https://issuer.example.com",
    ValidAudience = "target-audience",
};
`
	ents := extract(t, "custom_csharp_auth", fi("JwtConfig.cs", "csharp", src))
	// AddJwtBearer is not in file, but TokenValidationParameters IS
	tokenValFound := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "auth_coverage" && e.Name == "auth:TokenValidationParameters:JwtConfig.cs" {
			tokenValFound = true
		}
	}
	if !tokenValFound {
		t.Error("expected auth:TokenValidationParameters entity from TokenValidationParameters block")
	}
}

func TestAuthJwtBearerIssuerCapture(t *testing.T) {
	src := `
builder.Services.AddAuthentication().AddJwtBearer(o =>
{
    o.TokenValidationParameters = new TokenValidationParameters
    {
        ValidIssuer = "https://auth.myapp.io",
        ValidAudience = "myapp-api",
    };
});
`
	ents := extract(t, "custom_csharp_auth", fi("Auth.cs", "csharp", src))
	jwtEnt := entitySummary{}
	for _, e := range ents {
		if e.Name == "auth:AddJwtBearer:Auth.cs" {
			jwtEnt = e
			break
		}
	}
	if jwtEnt.Name == "" {
		t.Error("expected auth:AddJwtBearer:Auth.cs entity")
	}
}

func TestAuthAuthorizeScheme(t *testing.T) {
	src := `
[Authorize(AuthenticationSchemes = "Bearer")]
public IActionResult SecureEndpoint() => Ok();
`
	ents := extract(t, "custom_csharp_auth", fi("Controller.cs", "csharp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "auth_coverage" && e.Name == "auth:Authorize:scheme:Bearer" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth:Authorize:scheme:Bearer from [Authorize(AuthenticationSchemes=\"Bearer\")]")
	}
}

// ---------------------------------------------------------------------------
// csAuth: ASP.NET Identity (issue #3380)
// ---------------------------------------------------------------------------

func TestAuthAddIdentityFullRoles(t *testing.T) {
	src := `
builder.Services.AddIdentity<ApplicationUser, IdentityRole>(options =>
{
    options.Password.RequireDigit = true;
    options.Password.RequiredLength = 8;
})
.AddEntityFrameworkStores<AppDbContext>()
.AddDefaultTokenProviders();
`
	ents := extract(t, "custom_csharp_auth", fi("Program.cs", "csharp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "auth_coverage" && e.Name == "auth:AddIdentity:ApplicationUser:IdentityRole" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth:AddIdentity:ApplicationUser:IdentityRole from AddIdentity<TUser,TRole>")
	}
}

func TestAuthAddDefaultIdentity(t *testing.T) {
	src := `
builder.Services.AddDefaultIdentity<IdentityUser>(options =>
{
    options.SignIn.RequireConfirmedAccount = true;
})
.AddEntityFrameworkStores<AppDbContext>();
`
	ents := extract(t, "custom_csharp_auth", fi("Program.cs", "csharp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "auth_coverage" && e.Name == "auth:AddDefaultIdentity:IdentityUser" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth:AddDefaultIdentity:IdentityUser from AddDefaultIdentity<T>")
	}
}

func TestAuthAddIdentityCore(t *testing.T) {
	src := `services.AddIdentityCore<AppUser>().AddRoles<AppRole>();`
	ents := extract(t, "custom_csharp_auth", fi("Startup.cs", "csharp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "auth_coverage" && e.Name == "auth:AddIdentityCore:AppUser" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected auth:AddIdentityCore:AppUser from AddIdentityCore<T>")
	}
}

// ---------------------------------------------------------------------------
// csAuth: Pipeline registration (issue #3380)
// ---------------------------------------------------------------------------

func TestAuthUseAuthenticationPipeline(t *testing.T) {
	src := `
app.UseRouting();
app.UseAuthentication();
app.UseAuthorization();
app.MapControllers();
`
	ents := extract(t, "custom_csharp_auth", fi("Program.cs", "csharp", src))
	useAuthNFound := false
	useAuthZFound := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "auth_coverage" {
			if e.Name == "auth:UseAuthentication:Program.cs" {
				useAuthNFound = true
			}
			if e.Name == "auth:UseAuthorization:Program.cs" {
				useAuthZFound = true
			}
		}
	}
	if !useAuthNFound {
		t.Error("expected auth:UseAuthentication:Program.cs from app.UseAuthentication()")
	}
	if !useAuthZFound {
		t.Error("expected auth:UseAuthorization:Program.cs from app.UseAuthorization()")
	}
}

func TestAuthFullProgram(t *testing.T) {
	// Full realistic Program.cs with JWT + Identity + Authorization + Policies
	src := `
using Microsoft.AspNetCore.Authentication.JwtBearer;

var builder = WebApplication.CreateBuilder(args);

builder.Services.AddAuthentication(JwtBearerDefaults.AuthenticationScheme)
    .AddJwtBearer(options =>
    {
        options.TokenValidationParameters = new TokenValidationParameters
        {
            ValidIssuer = "https://auth.example.com",
            ValidAudience = "my-api",
        };
    });

builder.Services.AddDefaultIdentity<IdentityUser>()
    .AddEntityFrameworkStores<AppDbContext>();

builder.Services.AddAuthorization(options =>
{
    options.AddPolicy("AdminOnly", policy => policy.RequireRole("Admin"));
    options.AddPolicy("ApiAccess", policy => policy.RequireClaim("scope", "api.read"));
});

var app = builder.Build();

app.UseAuthentication();
app.UseAuthorization();
app.MapControllers();
`
	ents := extract(t, "custom_csharp_auth", fi("Program.cs", "csharp", src))

	expected := map[string]bool{
		"auth:AddAuthentication:Program.cs":         false,
		"auth:AddJwtBearer:Program.cs":              false,
		"auth:TokenValidationParameters:Program.cs": false,
		"auth:AddDefaultIdentity:IdentityUser":      false,
		"auth:AddAuthorization:Program.cs":          false,
		"auth:AddPolicy:AdminOnly":                  false,
		"auth:AddPolicy:ApiAccess":                  false,
		"auth:UseAuthentication:Program.cs":         false,
		"auth:UseAuthorization:Program.cs":          false,
	}

	for _, e := range ents {
		if _, ok := expected[e.Name]; ok {
			expected[e.Name] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("expected entity %q not found", name)
		}
	}
}
