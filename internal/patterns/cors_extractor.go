package patterns

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cajasmota/grafel/internal/types"
)

// corsExtractor detects CORS middleware configuration across frameworks and
// stamps a security-relevant policy node (SCOPE.Config / cors_policy) carrying
// the allowed origins, methods, credentials flag, and a wildcard flag. The
// dangerous posture — a wildcard origin ("*") combined with credentials — is
// captured explicitly so the graph can answer "what origins can call this API?"
// and flag the permissive `*` + credentials combination.
//
// CORS configuration is usually GLOBAL (app/middleware level), in which case
// the policy applies to every endpoint in that app; per-route configuration
// (Spring @CrossOrigin, ASP.NET [EnableCors]) is stamped on just that route's
// scope. We emit one cors_policy node per configuration site (matching the
// auth_policy posture model) rather than mutating every endpoint op.
type corsExtractor struct{}

var (
	corsExpressRE        = regexp.MustCompile(`(?:app|router)\s*\.\s*use\s*\(\s*cors\s*\(\s*(\{[^}]*\})?\s*\)`)
	corsExpressOriginRE  = regexp.MustCompile(`origin\s*:\s*["']([^"']+)["']`)
	corsExpressMethodsRE = regexp.MustCompile(`methods\s*:\s*["']([^"']+)["']`)
	corsExpressHeadersRE = regexp.MustCompile(`allowedHeaders\s*:\s*["']([^"']+)["']`)
	corsExpressOriginTRE = regexp.MustCompile(`origin\s*:\s*true\b`)

	corsSpringCrossOriRE = regexp.MustCompile(`@CrossOrigin\s*(?:\([^)]*\))?`)
	corsSpringOriginsRE  = regexp.MustCompile(`origins\s*=\s*["']([^"']+)["']`)
	corsSpringMethodsRE  = regexp.MustCompile(`allowedMethods\s*=\s*["']([^"']+)["']`)
	corsSpringRegistryRE = regexp.MustCompile(`addMapping\s*\(\s*["']([^"']+)["']\s*\)`)
	corsSpringAllowedRE  = regexp.MustCompile(`allowedOrigins(?:Patterns)?\s*\(\s*["']([^"']+)["']`)
	corsSpringCredRE     = regexp.MustCompile(`allowCredentials\s*(?:\(\s*true\s*\)|=\s*["']?true)`)

	corsASPNetAddCorsRE    = regexp.MustCompile(`(?:services|builder\s*\.\s*Services)\s*\.\s*AddCors\s*\(`)
	corsASPNetPolicyNameRE = regexp.MustCompile(`AddPolicy\s*\(\s*["']([^"']+)["']`)
	corsASPNetEnableCorsRE = regexp.MustCompile(`\[EnableCors\s*\(\s*["']([^"']+)["']\s*\)\]`)

	corsFastAPIRE        = regexp.MustCompile(`(?s)add_middleware\s*\(\s*CORSMiddleware\s*,([^)]+)\)`)
	corsFastAPIOriginsRE = regexp.MustCompile(`(?s)allow_origins\s*=\s*\[([^\]]*)\]`)
	corsFastAPIMethodsRE = regexp.MustCompile(`(?s)allow_methods\s*=\s*\[([^\]]*)\]`)
	corsFastAPICredRE    = regexp.MustCompile(`allow_credentials\s*=\s*True\b`)

	// django-cors-headers settings.
	corsDjangoAllowAllRE = regexp.MustCompile(`CORS_ALLOW_ALL_ORIGINS\s*=\s*True\b`)
	corsDjangoOriginsRE  = regexp.MustCompile(`(?s)CORS_ALLOWED_ORIGINS\s*=\s*\[([^\]]*)\]`)
	corsDjangoCredRE     = regexp.MustCompile(`CORS_ALLOW_CREDENTIALS\s*=\s*True\b`)

	// Rails rack-cors: `origins '*'` / `origins 'example.com'` inside a
	// Rack::Cors block. credentials true sets the credentials flag.
	corsRackBlockRE   = regexp.MustCompile(`Rack::Cors`)
	corsRackOriginsRE = regexp.MustCompile(`origins\s+([^\n]+)`)
	corsRackCredRE    = regexp.MustCompile(`credentials\s+true\b`)

	corsStringLiteralRE = regexp.MustCompile(`["']([^"']+)["']`)
)

var corsImportTokens = []string{
	"CrossOrigin", "CorsRegistry", "CorsConfiguration",
	"AddCors", "EnableCors", "CorsPolicy",
	"CORSMiddleware", "starlette.middleware.cors", "fastapi.middleware.cors",
	"require('cors')", `require("cors")`, "import cors",
	"CORS_ALLOWED_ORIGINS", "CORS_ALLOW_ALL_ORIGINS", "corsheaders",
	"Rack::Cors", "rack-cors",
}

var corsSourceTokens = []string{
	"cors(", "cors({", "CORSMiddleware", "@CrossOrigin",
	"AddCors(", "EnableCors(", "addMapping(", "allowedOrigins(",
	"CORS_ALLOW_ALL_ORIGINS", "CORS_ALLOWED_ORIGINS", "Rack::Cors",
}

func (c *corsExtractor) Category() string { return "cors" }

func (c *corsExtractor) AppliesTo(src string) bool {
	for _, tok := range corsImportTokens {
		if strings.Contains(src, tok) {
			return true
		}
	}
	for _, tok := range corsSourceTokens {
		if strings.Contains(src, tok) {
			return true
		}
	}
	return false
}

// isWildcardOrigin reports whether a resolved origin pattern is permissive
// (reflects/allows any origin). Express `cors()` with no options defaults to
// reflecting the request origin, which is treated as wildcard for posture.
func isWildcardOrigin(origin string) bool {
	return origin == "*" || strings.Contains(origin, "*")
}

// corsPolicyProps builds the property bag for a cors_policy node, always
// stamping cors_enabled and deriving cors_wildcard from the origin. credentials
// is stamped only when explicitly enabled. Origins are omitted (not fabricated)
// when dynamic.
func corsPolicyProps(framework, origin string, credentials bool) map[string]string {
	props := map[string]string{
		"kind":           "cors_policy",
		"framework":      framework,
		"cors_enabled":   "true",
		"origin_pattern": origin,
	}
	if origin != "dynamic" {
		props["cors_origins"] = origin
	}
	if isWildcardOrigin(origin) {
		props["cors_wildcard"] = "true"
	}
	if credentials {
		props["cors_credentials"] = "true"
	}
	return props
}

func (c *corsExtractor) Detect(filePath, language, src string) []types.EntityRecord {
	var results []types.EntityRecord
	seen := map[string]bool{}

	// Express: app.use(cors({...})) — global middleware.
	for idx, m := range corsExpressRE.FindAllStringSubmatchIndex(src, -1) {
		opts := ""
		if m[2] >= 0 {
			opts = src[m[2]:m[3]]
		}
		// Default cors() with no options reflects any origin → wildcard.
		origin := "*"
		if opts != "" {
			if om := corsExpressOriginRE.FindStringSubmatch(opts); om != nil {
				origin = om[1]
			} else if corsExpressOriginTRE.MatchString(opts) {
				// origin: true reflects the request origin → wildcard posture.
				origin = "*"
			} else {
				origin = "dynamic"
			}
		}
		key := fmt.Sprintf("express:%d", idx)
		if seen[key] {
			continue
		}
		seen[key] = true
		cred := strings.Contains(opts, "credentials") && regexp.MustCompile(`credentials\s*:\s*true`).MatchString(opts)
		props := corsPolicyProps("express", origin, cred)
		if mm := corsExpressMethodsRE.FindStringSubmatch(opts); mm != nil {
			props["methods"] = mm[1]
		}
		if hm := corsExpressHeadersRE.FindStringSubmatch(opts); hm != nil {
			props["allowed_headers"] = hm[1]
		}
		results = append(results, makeEntity(filePath,
			fmt.Sprintf("cors_policy_express_%d", idx), "SCOPE.Config", "cors_policy", language,
			lineOf(src, m[0]), props))
	}

	// Spring: @CrossOrigin — per-route (or per-controller) configuration.
	for idx, m := range corsSpringCrossOriRE.FindAllStringSubmatchIndex(src, -1) {
		ann := src[m[0]:m[1]]
		origin := "dynamic"
		if om := corsSpringOriginsRE.FindStringSubmatch(ann); om != nil {
			origin = om[1]
		} else if strings.Contains(ann, "@CrossOrigin") && !strings.Contains(ann, "(") {
			// Bare @CrossOrigin defaults to allowing all origins.
			origin = "*"
		}
		key := fmt.Sprintf("spring_crossorigin:%d", idx)
		if seen[key] {
			continue
		}
		seen[key] = true
		cred := corsSpringCredRE.MatchString(ann)
		props := corsPolicyProps("spring", origin, cred)
		if mm := corsSpringMethodsRE.FindStringSubmatch(ann); mm != nil {
			props["methods"] = mm[1]
		}
		results = append(results, makeEntity(filePath,
			fmt.Sprintf("cors_policy_spring_crossorigin_%d", idx), "SCOPE.Config", "cors_policy", language,
			lineOf(src, m[0]), props))
	}

	// Spring: addMapping("/**").allowedOrigins("...") — global CorsRegistry.
	for idx, m := range corsSpringRegistryRE.FindAllStringSubmatchIndex(src, -1) {
		mapping := src[m[2]:m[3]]
		origin := "dynamic"
		rest := src[m[1]:]
		if om := corsSpringAllowedRE.FindStringSubmatch(rest); om != nil {
			origin = om[1]
		}
		key := fmt.Sprintf("spring_registry:%d", idx)
		if seen[key] {
			continue
		}
		seen[key] = true
		cred := corsSpringCredRE.MatchString(rest)
		props := corsPolicyProps("spring", origin, cred)
		props["mapping"] = mapping
		results = append(results, makeEntity(filePath,
			fmt.Sprintf("cors_policy_spring_registry_%d", idx), "SCOPE.Config", "cors_policy", language,
			lineOf(src, m[0]), props))
	}

	// ASP.NET: builder.Services.AddCors(...) — global policy registration.
	for idx, m := range corsASPNetAddCorsRE.FindAllStringSubmatchIndex(src, -1) {
		policyName := "default"
		rest := src[m[1]:]
		if pm := corsASPNetPolicyNameRE.FindStringSubmatch(rest); pm != nil {
			policyName = pm[1]
		}
		key := fmt.Sprintf("aspnet_addcors:%d", idx)
		if seen[key] {
			continue
		}
		seen[key] = true
		cred := regexp.MustCompile(`AllowCredentials\s*\(`).MatchString(rest)
		props := corsPolicyProps("aspnet", "dynamic", cred)
		props["policy_name"] = policyName
		// AllowAnyOrigin() → wildcard posture.
		if regexp.MustCompile(`AllowAnyOrigin\s*\(`).MatchString(rest) {
			props["origin_pattern"] = "*"
			props["cors_origins"] = "*"
			props["cors_wildcard"] = "true"
		}
		results = append(results, makeEntity(filePath,
			"cors_policy_aspnet_"+policyName, "SCOPE.Config", "cors_policy", language,
			lineOf(src, m[0]), props))
	}

	// ASP.NET: [EnableCors("PolicyName")] — per-route attribute.
	for _, m := range corsASPNetEnableCorsRE.FindAllStringSubmatchIndex(src, -1) {
		policyName := src[m[2]:m[3]]
		key := "aspnet_enablecors:" + policyName
		if seen[key] {
			continue
		}
		seen[key] = true
		props := corsPolicyProps("aspnet", "dynamic", false)
		props["policy_name"] = policyName
		results = append(results, makeEntity(filePath,
			"cors_policy_aspnet_enable_"+policyName, "SCOPE.Config", "cors_policy", language,
			lineOf(src, m[0]), props))
	}

	// FastAPI/Starlette: add_middleware(CORSMiddleware, ...) — global middleware.
	for idx, m := range corsFastAPIRE.FindAllStringSubmatchIndex(src, -1) {
		args := src[m[2]:m[3]]
		origin := "dynamic"
		if om := corsFastAPIOriginsRE.FindStringSubmatch(args); om != nil {
			// Collect all string-literal origins; mark dynamic if none.
			lits := corsStringLiteralRE.FindAllStringSubmatch(om[1], -1)
			if len(lits) > 0 {
				var origins []string
				for _, lm := range lits {
					origins = append(origins, lm[1])
				}
				origin = strings.Join(origins, ",")
			}
		}
		key := fmt.Sprintf("fastapi:%d", idx)
		if seen[key] {
			continue
		}
		seen[key] = true
		cred := corsFastAPICredRE.MatchString(args)
		props := corsPolicyProps("fastapi", origin, cred)
		if mm := corsFastAPIMethodsRE.FindStringSubmatch(args); mm != nil {
			var methods []string
			for _, mv := range corsStringLiteralRE.FindAllStringSubmatch(mm[1], -1) {
				methods = append(methods, mv[1])
			}
			if len(methods) > 0 {
				props["methods"] = strings.Join(methods, ",")
			}
		}
		results = append(results, makeEntity(filePath,
			fmt.Sprintf("cors_policy_fastapi_%d", idx), "SCOPE.Config", "cors_policy", language,
			lineOf(src, m[0]), props))
	}

	// django-cors-headers: settings.py module-level config — global.
	results = append(results, c.detectDjango(filePath, language, src, seen)...)

	// Rails rack-cors: Rack::Cors middleware block — global.
	results = append(results, c.detectRack(filePath, language, src, seen)...)

	return results
}

// detectDjango handles django-cors-headers settings. CORS_ALLOW_ALL_ORIGINS=True
// is a wildcard; CORS_ALLOWED_ORIGINS=[...] is an explicit allowlist.
func (c *corsExtractor) detectDjango(filePath, language, src string, seen map[string]bool) []types.EntityRecord {
	allowAll := corsDjangoAllowAllRE.MatchString(src)
	originsMatch := corsDjangoOriginsRE.FindStringSubmatch(src)
	if !allowAll && originsMatch == nil {
		return nil
	}
	if seen["django"] {
		return nil
	}
	seen["django"] = true

	origin := "dynamic"
	if allowAll {
		origin = "*"
	} else if originsMatch != nil {
		var origins []string
		for _, lm := range corsStringLiteralRE.FindAllStringSubmatch(originsMatch[1], -1) {
			origins = append(origins, lm[1])
		}
		if len(origins) > 0 {
			origin = strings.Join(origins, ",")
		}
	}
	cred := corsDjangoCredRE.MatchString(src)
	props := corsPolicyProps("django-cors-headers", origin, cred)

	line := 1
	if loc := corsDjangoAllowAllRE.FindStringIndex(src); loc != nil {
		line = lineOf(src, loc[0])
	} else if loc := corsDjangoOriginsRE.FindStringIndex(src); loc != nil {
		line = lineOf(src, loc[0])
	}
	return []types.EntityRecord{makeEntity(filePath,
		"cors_policy_django", "SCOPE.Config", "cors_policy", language, line, props)}
}

// detectRack handles Rails rack-cors blocks. `origins '*'` is a wildcard;
// `origins 'example.com'` is an explicit allowlist; `credentials true` sets
// the credentials flag.
func (c *corsExtractor) detectRack(filePath, language, src string, seen map[string]bool) []types.EntityRecord {
	if !corsRackBlockRE.MatchString(src) {
		return nil
	}
	if seen["rack"] {
		return nil
	}
	seen["rack"] = true

	origin := "dynamic"
	if om := corsRackOriginsRE.FindStringSubmatch(src); om != nil {
		var origins []string
		for _, lm := range corsStringLiteralRE.FindAllStringSubmatch(om[1], -1) {
			origins = append(origins, lm[1])
		}
		if len(origins) > 0 {
			origin = strings.Join(origins, ",")
		}
	}
	cred := corsRackCredRE.MatchString(src)
	props := corsPolicyProps("rack-cors", origin, cred)

	line := 1
	if loc := corsRackBlockRE.FindStringIndex(src); loc != nil {
		line = lineOf(src, loc[0])
	}
	return []types.EntityRecord{makeEntity(filePath,
		"cors_policy_rack", "SCOPE.Config", "cors_policy", language, line, props)}
}

func init() {
	Register(&corsExtractor{})
}
