// C# taint-sites sniffer (#2773 Phase 2B T2).
//
// Recognises C# / .NET source / sink / sanitizer primitives across
// ASP.NET Core / MVC, Carter, FastEndpoints, ServiceStack, NancyFX,
// Blazor, Xamarin, .NET MAUI, and the gRPC-net stack.
//
// Sources:
//   - Request.Form / Query / Headers / Cookies / Body / Path /
//     RouteValues / Files
//   - [FromBody] / [FromQuery] / [FromForm] / [FromHeader] /
//     [FromRoute] parameter attributes on controller actions
//   - Environment.GetEnvironmentVariable / IConfiguration["..."]
//   - JsonSerializer.Deserialize / Newtonsoft.Json.JsonConvert.
//     DeserializeObject on untrusted bytes
//
// Sinks:
//   - SQL injection: SqlCommand with CommandText built from string
//     concatenation, IDbConnection.Execute / Query with a $"..." or
//     concatenated SQL; Entity Framework FromSqlRaw with an interpolated
//     string
//   - Command injection: Process.Start with a non-literal Arguments /
//     FileName, ProcessStartInfo with non-literal Arguments,
//     System.Diagnostics.Process.Start(string), CSharpScript.EvaluateAsync
//   - Path traversal: File.WriteAllText / ReadAllText / Open / Create /
//     Delete with a non-literal path, Directory.* equivalents
//   - XSS: Razor @Html.Raw of a non-literal, Response.Write of a
//     non-encoded string
//   - ReDoS: new Regex(<non-literal>) / Regex.Match with constructed
//     pattern
//   - Deserialisation: BinaryFormatter.Deserialize (RCE-capable),
//     JavaScriptSerializer with TypeResolver
//
// Sanitizers:
//   - Parameterised SQL: SqlCommand.Parameters.AddWithValue / Add,
//     IDbConnection.Execute(sql, new { p = v }) — Dapper named params,
//     EF Core FromSqlInterpolated (auto-parameterised) is also safe
//   - HTML escape: HtmlEncoder.Default.Encode, HttpUtility.HtmlEncode,
//     WebUtility.HtmlEncode
//   - Validation: DataAnnotations attributes (HARD RULE per #2772 —
//     [Required] / [StringLength] / [RegularExpression] on a class
//     property is the schema declaration; ModelState.IsValid by itself
//     is not a sanitizer), FluentValidation AbstractValidator<T>
//     subclass declarations
package substrate

import "regexp"

func init() { RegisterTaintSniffer("csharp", sniffTaintCSharp) }

// csSourceRequestRe matches ASP.NET Core HttpRequest accessors.
var csSourceRequestRe = regexp.MustCompile(
	`\b(?:Request|HttpContext\s*\.\s*Request)\s*\.\s*(?:Form|Query|Headers|Cookies|Body|Path|RouteValues|Files|QueryString)\b`,
)

// csSourceAttrRe matches [From*] action-method parameter attributes.
var csSourceAttrRe = regexp.MustCompile(
	`\[\s*From(?:Body|Query|Form|Header|Route|Services)\s*\]`,
)

// csSourceEnvRe matches Environment.GetEnvironmentVariable and
// IConfiguration indexer access.
var csSourceEnvRe = regexp.MustCompile(
	`\bEnvironment\s*\.\s*GetEnvironmentVariable\s*\(` +
		`|\b(?:_?[Cc]onfiguration|Configuration)\s*\[\s*"[A-Za-z_][\w:]*"\s*\]`,
)

// csSourceDeserializeRe matches deserialisation primitives. Newtonsoft
// JsonConvert.DeserializeObject and BinaryFormatter.Deserialize are
// the canonical RCE-capable surfaces.
var csSourceDeserializeRe = regexp.MustCompile(
	`\bJsonSerializer\s*\.\s*Deserialize\s*<` +
		`|\bJsonConvert\s*\.\s*DeserializeObject\s*(?:<|\()` +
		`|\bBinaryFormatter\s*\(\s*\)\s*\.\s*Deserialize\s*\(`,
)

// csSinkSQLRe matches SqlCommand.CommandText with $"..." or +,
// IDbConnection.Execute / Query with an interpolated/concat SQL.
// EF Core FromSqlRaw with a $"..." is the canonical raw-EF SQLi.
var csSinkSQLRe = regexp.MustCompile(
	`\bCommandText\s*=\s*\$?"[^"]*\{[^}]+\}` +
		`|\bCommandText\s*=\s*[A-Za-z_][\w]*\s*\+` +
		`|\.\s*(?:Execute|Query|QueryFirst|QueryFirstOrDefault|QuerySingle)\s*\(\s*\$"[^"]*\{` +
		`|\.\s*FromSqlRaw\s*\(\s*\$"[^"]*\{`,
)

// csSinkExecRe matches Process.Start with a non-literal FileName /
// Arguments, ProcessStartInfo with non-literal Arguments, dynamic-code
// CSharpScript.EvaluateAsync.
var csSinkExecRe = regexp.MustCompile(
	`\bProcess\s*\.\s*Start\s*\(\s*[A-Za-z_][\w]*\s*[,)]` +
		`|\bnew\s+ProcessStartInfo\s*\([^)]*[A-Za-z_][\w]*\s*\)` +
		`|\.\s*Arguments\s*=\s*[A-Za-z_][\w]*\s*;` +
		`|\bCSharpScript\s*\.\s*(?:EvaluateAsync|RunAsync)\s*\(`,
)

// csSinkFSRe matches File / Directory writes with a non-literal path.
var csSinkFSRe = regexp.MustCompile(
	`\bFile\s*\.\s*(?:WriteAllText|WriteAllBytes|WriteAllLines|AppendAllText|AppendAllLines|Open|OpenWrite|OpenRead|Create|CreateText|Delete|Move|Copy)\s*\(\s*[A-Za-z_][\w]*\s*[,)]` +
		`|\bDirectory\s*\.\s*(?:CreateDirectory|Delete|Move)\s*\(\s*[A-Za-z_][\w]*\s*[,)]`,
)

// csSinkXSSRe matches Razor @Html.Raw / Response.Write of a non-literal.
var csSinkXSSRe = regexp.MustCompile(
	`@Html\s*\.\s*Raw\s*\(\s*[A-Za-z_][\w]*\s*\)` +
		`|\bResponse\s*\.\s*Write\s*\(\s*[A-Za-z_][\w]*\s*\)` +
		`|\bMarkupString\s*\(\s*[A-Za-z_][\w]*\s*\)`,
)

// csSinkReDoSRe matches new Regex / Regex.Match with a non-literal
// pattern argument.
var csSinkReDoSRe = regexp.MustCompile(
	`\bnew\s+Regex\s*\(\s*[A-Za-z_][\w]*\s*[,)]` +
		`|\bRegex\s*\.\s*(?:Match|Matches|IsMatch|Replace|Split)\s*\([^,]*,\s*[A-Za-z_][\w]*\s*[,)]`,
)

// csSanitizerSQLRe matches SqlCommand parameter binding (the standard
// parameterised SQL flow), Dapper named-params, and EF Core's
// FromSqlInterpolated which auto-parameterises interpolation holes.
var csSanitizerSQLRe = regexp.MustCompile(
	`\.\s*Parameters\s*\.\s*Add(?:WithValue)?\s*\(` +
		`|\.\s*(?:Execute|Query|QueryFirst|QueryFirstOrDefault|QuerySingle)\s*\(\s*"[^"]*@[A-Za-z_][\w]*[^"]*"\s*,\s*new\b` +
		`|\.\s*FromSqlInterpolated\s*\(`,
)

// csSanitizerHTMLRe matches the canonical HTML-encode APIs.
var csSanitizerHTMLRe = regexp.MustCompile(
	`\bHtmlEncoder\s*\.\s*Default\s*\.\s*Encode\s*\(` +
		`|\bHttpUtility\s*\.\s*HtmlEncode\s*\(` +
		`|\bWebUtility\s*\.\s*HtmlEncode\s*\(`,
)

// csSanitizerValidateRe matches DataAnnotations schema declarations
// and FluentValidation subclass declarations. HARD RULE per #2772 —
// the [Required] / [StringLength] / [RegularExpression] attribute
// declaration or the AbstractValidator<T> subclass declaration is the
// signal; bare ModelState.IsValid does NOT count.
var csSanitizerValidateRe = regexp.MustCompile(
	`\[\s*(?:Required|StringLength|MaxLength|MinLength|Range|RegularExpression|EmailAddress|Url|CreditCard|Phone|DataType|Compare)\s*[\](]` +
		`|\bclass\s+[A-Za-z_]\w*\s*:\s*AbstractValidator\s*<`,
)

func sniffTaintCSharp(content string) []TaintMatch {
	if content == "" {
		return nil
	}
	headers := scanCSharpFuncHeaders(content)
	var out []TaintMatch
	out = appendTaintMatches(out, content, headers, csSourceRequestRe, TaintKindSource, TaintCategoryGeneric, "Request.Form/Query/Headers/Cookies", 1.0)
	out = appendTaintMatches(out, content, headers, csSourceAttrRe, TaintKindSource, TaintCategoryGeneric, "[FromBody]/[FromQuery]/[FromForm]", 0.95)
	out = appendTaintMatches(out, content, headers, csSourceEnvRe, TaintKindSource, TaintCategoryGeneric, "Environment.GetEnvironmentVariable/IConfiguration[...]", 0.85)
	out = appendTaintMatches(out, content, headers, csSourceDeserializeRe, TaintKindSource, TaintCategoryDeserialization, "JsonSerializer.Deserialize/BinaryFormatter", 0.9)
	// Sanitizers first.
	out = appendTaintMatches(out, content, headers, csSanitizerSQLRe, TaintKindSanitizer, TaintCategorySQL, "SqlCommand.Parameters.Add/Dapper named/FromSqlInterpolated", 1.0)
	out = appendTaintMatches(out, content, headers, csSanitizerHTMLRe, TaintKindSanitizer, TaintCategoryXSS, "HtmlEncoder/HttpUtility/WebUtility", 1.0)
	out = appendTaintMatches(out, content, headers, csSanitizerValidateRe, TaintKindSanitizer, TaintCategoryGeneric, "DataAnnotations/AbstractValidator<T>", 0.9)
	// Sinks.
	out = appendTaintMatches(out, content, headers, csSinkSQLRe, TaintKindSink, TaintCategorySQL, "CommandText=$/+/FromSqlRaw($)", 0.9)
	out = appendTaintMatches(out, content, headers, csSinkExecRe, TaintKindSink, TaintCategoryCommand, "Process.Start/ProcessStartInfo(non-literal)", 1.0)
	out = appendTaintMatches(out, content, headers, csSinkFSRe, TaintKindSink, TaintCategoryPath, "File.WriteAllText/Open(non-literal)", 0.85)
	out = appendTaintMatches(out, content, headers, csSinkXSSRe, TaintKindSink, TaintCategoryXSS, "@Html.Raw/Response.Write/MarkupString(non-literal)", 0.9)
	out = appendTaintMatches(out, content, headers, csSinkReDoSRe, TaintKindSink, TaintCategoryReDoS, "new Regex(non-literal)", 0.9)
	return out
}
