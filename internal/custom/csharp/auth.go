// Package csharp — auth extractor for C# source files.
//
// Detects ASP.NET Core / Microsoft.AspNetCore.Authorization patterns:
//   - [Authorize] / [Authorize(Roles="...")] / [Authorize(Policy="...")] attributes
//   - [AllowAnonymous] attribute
//   - RequireAuthorization() / RequireAuthorization("policy") minimal-API calls
//   - AddAuthorization() / AddAuthorization(options => ...) service registration
//   - AddPolicy("name", ...) policy definition
//
// csAuth — JWT bearer + ASP.NET Identity deepening (issue #3380):
//   - AddAuthentication(...).AddJwtBearer(...) JWT bearer configuration
//   - services.AddAuthentication(JwtBearerDefaults.AuthenticationScheme) scheme registration
//   - AddJwtBearer(options => { options.TokenValidationParameters = ... }) token params
//   - services.AddIdentity<TUser,TRole>() / AddDefaultIdentity<T>() / AddIdentityCore<T>()
//   - app.UseAuthentication() / app.UseAuthorization() pipeline registration
//   - [Authorize(AuthenticationSchemes="...")] scheme-specific authorization
//
// Emits SCOPE.Pattern entities with subtype "auth_coverage" so the coverage
// cells light up for the C# backend frameworks.
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
	extractor.Register("custom_csharp_auth", &csharpAuthExtractor{})
}

type csharpAuthExtractor struct{}

func (e *csharpAuthExtractor) Language() string { return "custom_csharp_auth" }

// ---------------------------------------------------------------------------
// Regexes
// ---------------------------------------------------------------------------

var (
	// [Authorize] — plain, no arguments
	reAuthorize = regexp.MustCompile(
		`\[Authorize\s*\]`,
	)
	// [Authorize(Roles = "...")] or [Authorize(Roles="...")]
	reAuthorizeRoles = regexp.MustCompile(
		`\[Authorize\s*\(\s*Roles\s*=\s*"([^"]+)"`,
	)
	// [Authorize(Policy = "...")] or [Authorize(Policy="...")]
	reAuthorizePolicy = regexp.MustCompile(
		`\[Authorize\s*\(\s*Policy\s*=\s*"([^"]+)"`,
	)
	// [Authorize("policyName")] — positional first arg as policy name
	reAuthorizePositional = regexp.MustCompile(
		`\[Authorize\s*\(\s*"([^"]+)"\s*\)`,
	)
	// [AllowAnonymous]
	reAllowAnonymous = regexp.MustCompile(
		`\[AllowAnonymous\s*\]`,
	)
	// .RequireAuthorization() / .RequireAuthorization("policyName")
	reRequireAuthorization = regexp.MustCompile(
		`\.RequireAuthorization\s*\(\s*(?:"([^"]+)")?\s*\)`,
	)
	// services.AddAuthorization(...) — DI registration
	reAddAuthorization = regexp.MustCompile(
		`\bAddAuthorization\s*\(`,
	)
	// options.AddPolicy("name", ...) — policy builder
	reAddPolicy = regexp.MustCompile(
		`\.AddPolicy\s*\(\s*"([^"]+)"`,
	)

	// csAuth: JWT bearer -------------------------------------------------------

	// .AddAuthentication(...) — authentication service registration
	csAuthAddAuthentication = regexp.MustCompile(
		`\bAddAuthentication\s*\(`,
	)

	// .AddJwtBearer(...) — JWT bearer scheme registration (chained after AddAuthentication)
	csAuthAddJwtBearer = regexp.MustCompile(
		`\.AddJwtBearer\s*\(`,
	)

	// options.TokenValidationParameters = new TokenValidationParameters { ... }
	csAuthTokenValidation = regexp.MustCompile(
		`\bTokenValidationParameters\s*=`,
	)

	// ValidIssuer = "..." inside token validation params
	csAuthIssuer = regexp.MustCompile(
		`ValidIssuer\s*=\s*"([^"]+)"`,
	)

	// ValidAudience = "..." inside token validation params
	csAuthAudience = regexp.MustCompile(
		`ValidAudience\s*=\s*"([^"]+)"`,
	)

	// [Authorize(AuthenticationSchemes="...")] — scheme-specific authorization
	csAuthSchemeAttr = regexp.MustCompile(
		`\[Authorize\s*\(\s*(?:[^)]*,\s*)?AuthenticationSchemes\s*=\s*"([^"]+)"`,
	)

	// csAuth: ASP.NET Identity ------------------------------------------------

	// services.AddIdentity<TUser, TRole>() — full Identity with roles
	csAuthAddIdentity = regexp.MustCompile(
		`\bAddIdentity\s*<\s*(\w+)\s*,\s*(\w+)\s*>`,
	)

	// services.AddDefaultIdentity<TUser>() — Identity without roles
	csAuthAddDefaultIdentity = regexp.MustCompile(
		`\bAddDefaultIdentity\s*<\s*(\w+)\s*>`,
	)

	// services.AddIdentityCore<TUser>() — minimal Identity
	csAuthAddIdentityCore = regexp.MustCompile(
		`\bAddIdentityCore\s*<\s*(\w+)\s*>`,
	)

	// app.UseAuthentication() — pipeline middleware registration
	csAuthUseAuthentication = regexp.MustCompile(
		`\bapp\.UseAuthentication\s*\(`,
	)

	// app.UseAuthorization() — pipeline middleware registration
	csAuthUseAuthorization = regexp.MustCompile(
		`\bapp\.UseAuthorization\s*\(`,
	)
)

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func (e *csharpAuthExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/csharp")
	_, span := tracer.Start(ctx, "indexer.csharp_auth_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
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

	// [Authorize] — plain attribute
	for _, m := range reAuthorize.FindAllStringIndex(src, -1) {
		line := lineOf(src, m[0])
		name := "auth:Authorize:plain:" + file.Path + ":" + itoa(line)
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "Authorize", "detail", "plain")
		add(ent)
	}

	// [Authorize(Roles="...")]
	for _, m := range reAuthorizeRoles.FindAllStringSubmatchIndex(src, -1) {
		roles := src[m[2]:m[3]]
		line := lineOf(src, m[0])
		name := "auth:Authorize:roles:" + roles
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "Authorize.Roles", "roles", roles)
		add(ent)
	}

	// [Authorize(Policy="...")]
	for _, m := range reAuthorizePolicy.FindAllStringSubmatchIndex(src, -1) {
		policy := src[m[2]:m[3]]
		line := lineOf(src, m[0])
		name := "auth:Authorize:policy:" + policy
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "Authorize.Policy", "policy_name", policy)
		add(ent)
	}

	// [Authorize("policyName")] positional
	for _, m := range reAuthorizePositional.FindAllStringSubmatchIndex(src, -1) {
		policy := src[m[2]:m[3]]
		line := lineOf(src, m[0])
		name := "auth:Authorize:policy:" + policy
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "Authorize.Policy", "policy_name", policy)
		add(ent)
	}

	// [AllowAnonymous]
	for _, m := range reAllowAnonymous.FindAllStringIndex(src, -1) {
		line := lineOf(src, m[0])
		name := "auth:AllowAnonymous:" + file.Path + ":" + itoa(line)
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "AllowAnonymous")
		add(ent)
	}

	// .RequireAuthorization(...)
	for _, m := range reRequireAuthorization.FindAllStringSubmatchIndex(src, -1) {
		line := lineOf(src, m[0])
		policy := ""
		if m[2] >= 0 {
			policy = src[m[2]:m[3]]
		}
		name := "auth:RequireAuthorization:" + policy + ":" + file.Path + ":" + itoa(line)
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "RequireAuthorization", "policy_name", policy)
		add(ent)
	}

	// AddAuthorization() — service registration (emit once per file)
	if reAddAuthorization.MatchString(src) {
		line := 1
		if idx := reAddAuthorization.FindStringIndex(src); idx != nil {
			line = lineOf(src, idx[0])
		}
		name := "auth:AddAuthorization:" + file.Path
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "AddAuthorization")
		add(ent)
	}

	// .AddPolicy("name", ...)
	for _, m := range reAddPolicy.FindAllStringSubmatchIndex(src, -1) {
		policy := src[m[2]:m[3]]
		line := lineOf(src, m[0])
		name := "auth:AddPolicy:" + policy
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "AddPolicy", "policy_name", policy)
		add(ent)
	}

	// -----------------------------------------------------------------------
	// csAuth: JWT Bearer (issue #3380)
	// -----------------------------------------------------------------------

	// AddAuthentication(...) — authentication service registration (emit once per file)
	if csAuthAddAuthentication.MatchString(src) {
		idx := csAuthAddAuthentication.FindStringIndex(src)
		line := 1
		if idx != nil {
			line = lineOf(src, idx[0])
		}
		name := "auth:AddAuthentication:" + file.Path
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "AddAuthentication", "mechanism", "jwt_bearer")
		add(ent)
	}

	// .AddJwtBearer(...) — JWT bearer scheme (emit once per file)
	if csAuthAddJwtBearer.MatchString(src) {
		idx := csAuthAddJwtBearer.FindStringIndex(src)
		line := 1
		if idx != nil {
			line = lineOf(src, idx[0])
		}
		name := "auth:AddJwtBearer:" + file.Path
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "AddJwtBearer", "mechanism", "jwt_bearer")
		// Capture issuer + audience if present in file
		if m := csAuthIssuer.FindStringSubmatch(src); len(m) > 1 {
			ent.Properties["issuer"] = m[1]
		}
		if m := csAuthAudience.FindStringSubmatch(src); len(m) > 1 {
			ent.Properties["audience"] = m[1]
		}
		add(ent)
	}

	// TokenValidationParameters — JWT validation config (emit once per file)
	if csAuthTokenValidation.MatchString(src) {
		idx := csAuthTokenValidation.FindStringIndex(src)
		line := 1
		if idx != nil {
			line = lineOf(src, idx[0])
		}
		name := "auth:TokenValidationParameters:" + file.Path
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "TokenValidationParameters", "mechanism", "jwt_bearer")
		add(ent)
	}

	// [Authorize(AuthenticationSchemes="...")] — scheme-specific
	for _, m := range csAuthSchemeAttr.FindAllStringSubmatchIndex(src, -1) {
		scheme := src[m[2]:m[3]]
		line := lineOf(src, m[0])
		name := "auth:Authorize:scheme:" + scheme
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "Authorize.AuthScheme", "scheme", scheme, "mechanism", "jwt_bearer")
		add(ent)
	}

	// -----------------------------------------------------------------------
	// csAuth: ASP.NET Identity (issue #3380)
	// -----------------------------------------------------------------------

	// services.AddIdentity<TUser, TRole>()
	for _, m := range csAuthAddIdentity.FindAllStringSubmatchIndex(src, -1) {
		userType := src[m[2]:m[3]]
		roleType := src[m[4]:m[5]]
		line := lineOf(src, m[0])
		name := "auth:AddIdentity:" + userType + ":" + roleType
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "AddIdentity", "mechanism", "aspnet_identity",
			"user_type", userType, "role_type", roleType)
		add(ent)
	}

	// services.AddDefaultIdentity<TUser>()
	for _, m := range csAuthAddDefaultIdentity.FindAllStringSubmatchIndex(src, -1) {
		userType := src[m[2]:m[3]]
		line := lineOf(src, m[0])
		name := "auth:AddDefaultIdentity:" + userType
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "AddDefaultIdentity", "mechanism", "aspnet_identity",
			"user_type", userType)
		add(ent)
	}

	// services.AddIdentityCore<TUser>()
	for _, m := range csAuthAddIdentityCore.FindAllStringSubmatchIndex(src, -1) {
		userType := src[m[2]:m[3]]
		line := lineOf(src, m[0])
		name := "auth:AddIdentityCore:" + userType
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "AddIdentityCore", "mechanism", "aspnet_identity",
			"user_type", userType)
		add(ent)
	}

	// -----------------------------------------------------------------------
	// csAuth: Pipeline registration (issue #3380)
	// -----------------------------------------------------------------------

	// app.UseAuthentication()
	if csAuthUseAuthentication.MatchString(src) {
		idx := csAuthUseAuthentication.FindStringIndex(src)
		line := 1
		if idx != nil {
			line = lineOf(src, idx[0])
		}
		name := "auth:UseAuthentication:" + file.Path
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "UseAuthentication", "mechanism", "pipeline")
		add(ent)
	}

	// app.UseAuthorization()
	if csAuthUseAuthorization.MatchString(src) {
		idx := csAuthUseAuthorization.FindStringIndex(src)
		line := 1
		if idx != nil {
			line = lineOf(src, idx[0])
		}
		name := "auth:UseAuthorization:" + file.Path
		ent := makeEntity(name, "SCOPE.Pattern", "auth_coverage", file.Path, "csharp", line)
		setProps(&ent, "auth_pattern", "UseAuthorization", "mechanism", "pipeline")
		add(ent)
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}

// itoa converts an int to its decimal string representation without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
