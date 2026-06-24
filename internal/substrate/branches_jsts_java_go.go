// Branch inventory analyzers for brace-delimited languages — JS/TS, Java, Go
// (#4434, extends #4423/#4435, epic #4419 capability 4).
//
// branches.go added the language-neutral BranchFacet schema, the
// BranchAnalyzerFn registry, and the flagship Python analyzer (an
// indentation-scoped CFG walk). This file registers analyzers for the three
// brace-delimited languages the acme stack and the wider corpus lean on.
//
// Where Python keys block scope on indentation, these languages delimit blocks
// with `{`/`}`. The shared helper braceBlockBody walks forward from a header
// line tracking brace depth to collect exactly the statements of the block the
// header opens — the brace-language analogue of pyBlockBody. Each analyzer then
// classifies the FIRST control-altering statement (throw/return/redirect/
// status-write) in that block, exactly mirroring the Python outcome lattice.
//
// Same opt-in contract: these run only when the effects MCP tool is called with
// include="branches", so the default effects payload is byte-for-byte
// unchanged. Classification is deliberately conservative — a branch is only
// surfaced when it provably alters control flow (returns / throws / redirects /
// writes an HTTP error status / panics); plain branching `if`s are skipped so
// the facet does not drown a porting agent in every conditional.
package substrate

import (
	"regexp"
	"strings"
)

func init() {
	RegisterBranchAnalyzer("jsts", analyzeBranchesJSTS)
	RegisterBranchAnalyzer("java", analyzeBranchesJava)
	RegisterBranchAnalyzer("go", analyzeBranchesGo)
}

// --- method-boundary clamp (shared by all brace-language analyzers) -------

// braceBodyEnd returns the exclusive line index at which the function/method
// whose header is the first non-blank line of `lines` ends — the line just
// AFTER the `}` that closes the body block the header opens. It is the
// brace-language analogue of bodyEndPython: it stops a branch walk from
// bleeding into the sibling methods that follow when the effects tool padded
// the source window because the entity's EndLine was missing (#4488/#4666).
//
// Walk model: find the first `{` at or after the header line (K&R or Allman),
// then track brace depth — skipping braces inside string/char literals and
// comments via stripBraceNoise — until depth returns to 0. The line holding
// that closing `}` is the last line of the body; the boundary is the next
// index. When no opening brace is found (a brace-less arrow body, or a
// truncated window), the whole window is returned (len(lines)) so we never
// drop real branches — honest over-inclusion beats silent truncation.
func braceBodyEnd(lines []string) int {
	headerIdx := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) != "" {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return len(lines)
	}
	depth := 0
	opened := false
	for j := headerIdx; j < len(lines); j++ {
		for _, r := range stripBraceNoise(lines[j]) {
			switch r {
			case '{':
				depth++
				opened = true
			case '}':
				depth--
			}
		}
		if opened && depth <= 0 {
			return j + 1 // body closes on line j; boundary is the next line
		}
	}
	return len(lines)
}

// --- shared brace-block helpers -----------------------------------------

// braceBlockBody returns the source lines of the block opened on or after the
// header line lines[headerIdx]. It scans forward from the header, finds the
// first `{` that opens the block, and returns every line until the matching
// `}` (exclusive of the closing-brace line's trailing content). The opening
// brace may be on the header line (K&R) or the next line (Allman). When no
// brace is found within a small lookahead (e.g. a single-statement
// brace-less `if (x) return;`), the remainder of the header line after the
// condition plus the next line are returned so the classifier can still see a
// trailing `return`/`throw`.
//
// String/char/line-comment contents are skipped when counting braces so a `{`
// inside a literal does not corrupt depth tracking — a cheap scanner, not a
// full lexer, which is sufficient for the well-formed function windows the
// effects pipeline feeds in.
func braceBlockBody(lines []string, headerIdx int, afterCond string) []string {
	// Brace-less single-statement form: `if (x) return y;` — afterCond holds
	// the text after the `)`. If it already contains a statement, that IS the
	// body.
	if s := strings.TrimSpace(afterCond); s != "" && !strings.HasPrefix(s, "{") {
		return []string{s}
	}

	depth := 0
	opened := false
	var body []string
	// Look a few lines ahead for the opening brace (Allman style), then collect
	// until depth returns to zero. The header line may carry a LEADING `}` that
	// closes the PRECEDING block (`} catch (e) {`, `} else if (x) {`); skip up
	// to the header keyword so that closer is not counted as our depth.
	for j := headerIdx; j < len(lines); j++ {
		ln := lines[j]
		scan := ln
		if j == headerIdx {
			scan = stripLeadingCloser(ln)
		}
		startDepth := depth
		for _, r := range stripBraceNoise(scan) {
			switch r {
			case '{':
				depth++
				opened = true
			case '}':
				depth--
			}
		}
		if !opened {
			// Haven't seen the opening brace yet. If we've scanned past a small
			// lookahead with no brace, treat the next non-blank line as a
			// brace-less body (defensive; afterCond already handles the common
			// single-line case).
			if j-headerIdx > 2 {
				return nil
			}
			continue
		}
		// We are inside (or just opened) the block. Collect statement lines that
		// live strictly inside the braces — exclude the header line itself
		// (which only opens the block) and the closing-brace line.
		if j > headerIdx && startDepth >= 1 {
			body = append(body, ln)
		} else if j == headerIdx {
			// K&R: header line may carry an inline statement after `{`.
			if idx := strings.Index(ln, "{"); idx >= 0 {
				rest := strings.TrimSpace(ln[idx+1:])
				rest = strings.TrimSuffix(rest, "}")
				if rest != "" {
					body = append(body, rest)
				}
			}
		}
		if opened && depth <= 0 {
			break
		}
	}
	return body
}

// stripLeadingCloser blanks a leading `}` (and surrounding whitespace) on a
// header line so the closing brace of the PRECEDING block (`} catch`, `} else`)
// is not counted toward the new block's brace depth.
func stripLeadingCloser(ln string) string {
	trimmed := strings.TrimLeft(ln, " \t")
	if strings.HasPrefix(trimmed, "}") {
		// Replace the leading `}` with a space, preserving the rest.
		idx := strings.IndexByte(ln, '}')
		return ln[:idx] + " " + ln[idx+1:]
	}
	return ln
}

// stripBraceNoise blanks out the contents of string/char literals and
// line/block-comment text on a single line so brace counting is not corrupted
// by braces inside literals or comments. Returns a same-length-ish rune-safe
// string with literal/comment bodies replaced by spaces.
func stripBraceNoise(ln string) string {
	var b strings.Builder
	runes := []rune(ln)
	i := 0
	for i < len(runes) {
		c := runes[i]
		switch c {
		case '/':
			if i+1 < len(runes) && runes[i+1] == '/' {
				return b.String() // rest of line is a comment
			}
			b.WriteRune(c)
			i++
		case '"', '\'', '`':
			quote := c
			b.WriteRune(' ')
			i++
			for i < len(runes) {
				if runes[i] == '\\' { // skip escaped char
					i += 2
					continue
				}
				if runes[i] == quote {
					break
				}
				i++
			}
			b.WriteRune(' ')
			i++
		default:
			b.WriteRune(c)
			i++
		}
	}
	return b.String()
}

// --- JS/TS analyzer ------------------------------------------------------

var (
	jstsCatchRe = regexp.MustCompile(`^\s*}?\s*catch\s*(\(([^)]*)\))?\s*\{?`)
	jstsIfRe    = regexp.MustCompile(`^\s*(?:\}\s*else\s+)?if\s*\((.*)\)\s*(.*)$`)

	jstsThrowRe    = regexp.MustCompile(`(^|\b)throw\b`)
	jstsReturnRe   = regexp.MustCompile(`(^|\b)return\b`)
	jstsRedirectRe = regexp.MustCompile(`\.\s*redirect\s*\(|\bRedirect\b`)
	// jstsLogOnlyRe — a catch body that only logs / is empty.
	jstsLogCallRe = regexp.MustCompile(`\b(?:console\.\w+|logger?\.\w+|log\.\w+)\s*\(`)

	// jstsEnvRefRe — process.env.X, process.env['X'], import.meta.env.X.
	jstsEnvRefRe = regexp.MustCompile(
		`\bprocess\s*\.\s*env\s*(?:\.\s*([A-Za-z_]\w*)|\[\s*['"]([^'"]+)['"]\s*\])` +
			`|\bimport\s*\.\s*meta\s*\.\s*env\s*\.\s*([A-Za-z_]\w*)`)

	// jstsStatusRe — res.status(NNN), throw new HttpException(.., NNN),
	// reply.code(NNN), statusCode = NNN, NestJS HttpStatus.NAME mapped loosely.
	jstsStatusCallRe = regexp.MustCompile(`\.\s*(?:status|code|sendStatus)\s*\(\s*(\d{3})\b`)
	jstsHTTPExcRe    = regexp.MustCompile(`HttpException\s*\([^)]*?\b(\d{3})\b`)
	jstsStatusNumRe  = regexp.MustCompile(`statusCode\s*[:=]\s*(\d{3})\b`)

	// jstsHTTPStatusEnumRe — NestJS `HttpStatus.CONFLICT` / `HttpStatus.CREATED`
	// (the enum form, mapped via httpStatusNameToCode — shared with the Java/Go
	// analyzers so the status vocabulary stays one source of truth).
	jstsHTTPStatusEnumRe = regexp.MustCompile(`HttpStatus\s*\.\s*([A-Z_]+)`)

	// jstsNestExcRe — NestJS built-in HTTP exception classes
	// (`throw new ConflictException(...)`). Nest maps each to a fixed status, so
	// a branching handler that throws one declares that branch's status without
	// any numeric literal. Mapped via nestExceptionStatus.
	jstsNestExcRe = regexp.MustCompile(`\bnew\s+([A-Z][A-Za-z]*Exception)\b`)
)

func analyzeBranchesJSTS(funcSource string, startLine int) []BranchFacet {
	if strings.TrimSpace(funcSource) == "" {
		return nil
	}
	lines := strings.Split(funcSource, "\n")
	var out []BranchFacet
	firstGuardSeen := false

	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		if strings.TrimSpace(raw) == "" {
			continue
		}
		absLine := startLine + i

		// catch handler.
		if m := jstsCatchRe.FindStringSubmatch(raw); m != nil {
			cond := "catch"
			if strings.TrimSpace(m[2]) != "" {
				cond = "catch (" + strings.TrimSpace(m[2]) + ")"
			}
			body := braceBlockBody(lines, i, afterCloseParen(raw))
			outcome := classifyBraceExceptOutcome(body, jstsThrowRe, jstsReturnRe, jstsRedirectRe, jstsLogCallRe)
			bf := BranchFacet{Kind: BranchExcept, Condition: cond, Outcome: outcome, Line: absLine}
			attachBraceReturns(&bf, body, jstsStatusFromBody, jstsRedirectRe)
			out = append(out, bf)
			continue
		}

		// if guard.
		if m := jstsIfRe.FindStringSubmatch(raw); m != nil {
			cond := "if (" + strings.TrimSpace(m[1]) + ")"
			body := braceBlockBody(lines, i, m[2])
			outcome, alters := classifyBraceGuardOutcome(body, jstsThrowRe, jstsReturnRe, jstsRedirectRe)
			if !alters {
				continue
			}
			envVar := matchEnvVar(jstsEnvRefRe, m[1])
			kind := guardKind(envVar, &firstGuardSeen)
			bf := BranchFacet{Kind: kind, Condition: cond, Outcome: outcome, EnvVar: envVar, Line: absLine}
			attachBraceReturns(&bf, body, jstsStatusFromBody, jstsRedirectRe)
			out = append(out, bf)
			continue
		}
	}
	return out
}

func jstsStatusFromBody(joined string) string {
	if m := jstsStatusCallRe.FindStringSubmatch(joined); m != nil {
		return m[1]
	}
	if m := jstsHTTPExcRe.FindStringSubmatch(joined); m != nil {
		return m[1]
	}
	if m := jstsStatusNumRe.FindStringSubmatch(joined); m != nil {
		return m[1]
	}
	// NestJS `HttpStatus.CONFLICT` enum reference (e.g.
	// `throw new HttpException(msg, HttpStatus.CONFLICT)`).
	if m := jstsHTTPStatusEnumRe.FindStringSubmatch(joined); m != nil {
		if code := httpStatusNameToCode(m[1]); code != "" {
			return code
		}
	}
	// NestJS built-in exception class (`throw new ConflictException()` → 409).
	if m := jstsNestExcRe.FindStringSubmatch(joined); m != nil {
		if code := nestExceptionStatus(m[1]); code != "" {
			return code
		}
	}
	return ""
}

// nestExceptionStatus maps a NestJS built-in HTTP exception class name to its
// fixed HTTP status code (the framework hard-codes these in
// @nestjs/common/exceptions). Conservative — only the built-in classes; an
// unknown / user-defined *Exception yields "" (honest-partial: no fabricated
// status). Shared by the response-branch status derivation.
func nestExceptionStatus(class string) string {
	switch class {
	case "BadRequestException":
		return "400"
	case "UnauthorizedException":
		return "401"
	case "PaymentRequiredException":
		return "402"
	case "ForbiddenException":
		return "403"
	case "NotFoundException":
		return "404"
	case "MethodNotAllowedException":
		return "405"
	case "NotAcceptableException":
		return "406"
	case "RequestTimeoutException":
		return "408"
	case "ConflictException":
		return "409"
	case "GoneException":
		return "410"
	case "PreconditionFailedException":
		return "412"
	case "PayloadTooLargeException":
		return "413"
	case "UnsupportedMediaTypeException":
		return "415"
	case "ImATeapotException":
		return "418"
	case "UnprocessableEntityException":
		return "422"
	case "TooManyRequestsException":
		return "429"
	case "InternalServerErrorException":
		return "500"
	case "NotImplementedException":
		return "501"
	case "BadGatewayException":
		return "502"
	case "ServiceUnavailableException":
		return "503"
	case "GatewayTimeoutException":
		return "504"
	case "HttpVersionNotSupportedException":
		return "505"
	default:
		return ""
	}
}

// --- Java analyzer -------------------------------------------------------

var (
	javaCatchRe = regexp.MustCompile(`^\s*}?\s*catch\s*\(([^)]*)\)\s*\{?`)
	javaIfRe    = regexp.MustCompile(`^\s*(?:\}\s*else\s+)?if\s*\((.*)\)\s*(.*)$`)

	javaThrowRe    = regexp.MustCompile(`(^|\b)throw\b`)
	javaReturnRe   = regexp.MustCompile(`(^|\b)return\b`)
	javaRedirectRe = regexp.MustCompile(`\bsendRedirect\s*\(|\bRedirectView\b|new\s+ModelAndView\s*\(\s*["']redirect:|["']redirect:`)
	javaLogCallRe  = regexp.MustCompile(`\b(?:log|logger|LOG|LOGGER)\s*\.\s*\w+\s*\(|\bSystem\s*\.\s*(?:out|err)\s*\.`)

	// javaEnvRefRe — System.getenv("X"), @Value("${x}"), env.getProperty("x").
	javaEnvRefRe = regexp.MustCompile(
		`\bSystem\s*\.\s*getenv\s*\(\s*"([^"]+)"` +
			`|@Value\s*\(\s*"\$\{\s*([A-Za-z_][\w.]*)` +
			`|\bgetProperty\s*\(\s*"([^"]+)"` +
			`|\bgetenv\s*\(\s*"([^"]+)"`)

	// javaStatusRe — ResponseEntity.status(NNN) / HttpStatus.NAME(code in name?)
	// and response.setStatus(NNN), response.sendError(NNN).
	javaStatusCallRe   = regexp.MustCompile(`\.\s*(?:status|setStatus|sendError)\s*\(\s*(\d{3})\b`)
	javaHttpStatusEnum = regexp.MustCompile(`HttpStatus\s*\.\s*([A-Z_]+)`)
)

func analyzeBranchesJava(funcSource string, startLine int) []BranchFacet {
	if strings.TrimSpace(funcSource) == "" {
		return nil
	}
	lines := strings.Split(funcSource, "\n")
	var out []BranchFacet
	firstGuardSeen := false

	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		if strings.TrimSpace(raw) == "" {
			continue
		}
		absLine := startLine + i

		if m := javaCatchRe.FindStringSubmatch(raw); m != nil {
			cond := "catch (" + strings.TrimSpace(m[1]) + ")"
			body := braceBlockBody(lines, i, afterCloseParen(raw))
			outcome := classifyBraceExceptOutcome(body, javaThrowRe, javaReturnRe, javaRedirectRe, javaLogCallRe)
			bf := BranchFacet{Kind: BranchExcept, Condition: cond, Outcome: outcome, Line: absLine}
			attachBraceReturns(&bf, body, javaStatusFromBody, javaRedirectRe)
			out = append(out, bf)
			continue
		}

		if m := javaIfRe.FindStringSubmatch(raw); m != nil {
			cond := "if (" + strings.TrimSpace(m[1]) + ")"
			body := braceBlockBody(lines, i, m[2])
			outcome, alters := classifyBraceGuardOutcome(body, javaThrowRe, javaReturnRe, javaRedirectRe)
			if !alters {
				continue
			}
			envVar := matchEnvVar(javaEnvRefRe, m[1])
			kind := guardKind(envVar, &firstGuardSeen)
			bf := BranchFacet{Kind: kind, Condition: cond, Outcome: outcome, EnvVar: envVar, Line: absLine}
			attachBraceReturns(&bf, body, javaStatusFromBody, javaRedirectRe)
			out = append(out, bf)
			continue
		}
	}
	return out
}

func javaStatusFromBody(joined string) string {
	if m := javaStatusCallRe.FindStringSubmatch(joined); m != nil {
		return m[1]
	}
	if m := javaHttpStatusEnum.FindStringSubmatch(joined); m != nil {
		if code := httpStatusNameToCode(m[1]); code != "" {
			return code
		}
	}
	return ""
}

// --- Go analyzer ---------------------------------------------------------

var (
	// goIfRe matches `if <cond> {` (Go requires the brace on the same line as
	// the header, optionally with a simple-statement init: `if err := f(); err
	// != nil {`).
	goIfRe = regexp.MustCompile(`^\s*}?\s*(?:else\s+)?if\s+(.*?)\s*\{\s*$`)

	goReturnRe   = regexp.MustCompile(`(^|\b)return\b`)
	goPanicRe    = regexp.MustCompile(`\bpanic\s*\(`)
	goRedirectRe = regexp.MustCompile(`\bhttp\.Redirect\s*\(|\.\s*Redirect\s*\(`)

	// goEnvRefRe — os.Getenv("X"), os.LookupEnv("X").
	goEnvRefRe = regexp.MustCompile(`\bos\s*\.\s*(?:Getenv|LookupEnv)\s*\(\s*"([^"]+)"`)

	// goStatusRe — http.Error(w, .., NNN), w.WriteHeader(NNN),
	// w.WriteHeader(http.StatusXxx), c.JSON(NNN, ..) (gin), c.Status(NNN).
	goStatusCallRe   = regexp.MustCompile(`\bhttp\.Error\s*\([^,]+,[^,]+,\s*(\d{3})\b|\bWriteHeader\s*\(\s*(\d{3})\b|\.\s*(?:JSON|Status|AbortWithStatus(?:JSON)?)\s*\(\s*(\d{3})\b`)
	goStatusEnumRe   = regexp.MustCompile(`http\.Status([A-Za-z]+)\b`)
	goWrappedErrorRe = regexp.MustCompile(`\b(?:fmt\.Errorf|errors\.(?:New|Wrap)|errors\.Wrapf)\s*\(`)
)

func analyzeBranchesGo(funcSource string, startLine int) []BranchFacet {
	if strings.TrimSpace(funcSource) == "" {
		return nil
	}
	lines := strings.Split(funcSource, "\n")
	var out []BranchFacet
	firstGuardSeen := false

	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		if strings.TrimSpace(raw) == "" {
			continue
		}
		absLine := startLine + i

		m := goIfRe.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		condExpr := strings.TrimSpace(m[1])
		// Strip a leading simple-statement init (`err := f(); err != nil`) so the
		// surfaced condition is the boolean expression.
		if semi := strings.LastIndex(condExpr, ";"); semi >= 0 {
			condExpr = strings.TrimSpace(condExpr[semi+1:])
		}
		cond := "if " + condExpr
		body := braceBlockBody(lines, i, "{")
		outcome, alters := classifyGoGuardOutcome(body)
		if !alters {
			continue
		}
		envVar := matchEnvVar(goEnvRefRe, m[1])
		// The dominant Go branch — `if err != nil { return ... }`. It is a
		// guard, not an env_gate, unless the condition reads an env var.
		kind := guardKind(envVar, &firstGuardSeen)
		bf := BranchFacet{Kind: kind, Condition: cond, Outcome: outcome, EnvVar: envVar, Line: absLine}
		attachBraceReturns(&bf, body, goStatusFromBody, goRedirectRe)
		out = append(out, bf)
	}
	return out
}

// classifyGoGuardOutcome — Go has no `throw`; the raise-equivalent is panic().
// `if err != nil { return ..., err }` is the canonical early-return. A bare
// `return` with no panic is return_value; a panic is raise; a redirect call is
// redirect.
func classifyGoGuardOutcome(body []string) (BranchOutcome, bool) {
	joined := strings.Join(body, "\n")
	for _, ln := range body {
		if goPanicRe.MatchString(ln) {
			return OutcomeRaise, true
		}
		if goReturnRe.MatchString(ln) {
			if goRedirectRe.MatchString(joined) {
				return OutcomeRedirect, true
			}
			return OutcomeReturnValue, true
		}
		if goRedirectRe.MatchString(ln) {
			return OutcomeRedirect, true
		}
	}
	// A block that writes an HTTP error status then falls through (common in
	// handlers: `http.Error(w, .., 400); return`) is caught by the return
	// above; a status write with no return is still a control-altering response
	// branch worth surfacing.
	if goStatusFromBody(joined) != "" {
		return OutcomeReturnValue, true
	}
	return "", false
}

func goStatusFromBody(joined string) string {
	if m := goStatusCallRe.FindStringSubmatch(joined); m != nil {
		for _, g := range m[1:] {
			if g != "" {
				return g
			}
		}
	}
	if m := goStatusEnumRe.FindStringSubmatch(joined); m != nil {
		if code := httpStatusNameToCode(strings.ToUpper(camelToSnake(m[1]))); code != "" {
			return code
		}
	}
	return ""
}

// --- shared brace-language classification --------------------------------

// classifyBraceExceptOutcome classifies a catch/handler block body. Re-throw
// or return → raise/return_value; a body that only logs / is empty → swallow.
func classifyBraceExceptOutcome(body []string, throwRe, returnRe, redirectRe, logRe *regexp.Regexp) BranchOutcome {
	joined := strings.Join(body, "\n")
	for _, ln := range body {
		if throwRe.MatchString(ln) {
			return OutcomeRaise
		}
		if returnRe.MatchString(ln) {
			if redirectRe.MatchString(joined) {
				return OutcomeRedirect
			}
			return OutcomeReturnValue
		}
		if redirectRe.MatchString(ln) {
			return OutcomeRedirect
		}
	}
	// No re-throw / return / redirect → swallow (catch-and-continue), whether it
	// logs or is empty. The audit-critical silent-failure path.
	_ = logRe
	return OutcomeSwallow
}

// classifyBraceGuardOutcome inspects an if-block body and returns its outcome +
// whether it alters control flow. A guard that neither throws nor returns nor
// redirects is not surfaced (conservative).
func classifyBraceGuardOutcome(body []string, throwRe, returnRe, redirectRe *regexp.Regexp) (BranchOutcome, bool) {
	joined := strings.Join(body, "\n")
	for _, ln := range body {
		if throwRe.MatchString(ln) {
			return OutcomeRaise, true
		}
		if returnRe.MatchString(ln) {
			if redirectRe.MatchString(joined) {
				return OutcomeRedirect, true
			}
			return OutcomeReturnValue, true
		}
		if redirectRe.MatchString(ln) {
			return OutcomeRedirect, true
		}
	}
	return "", false
}

// attachBraceReturns derives returns.status + returns.shape for a brace-language
// branch using a per-language status extractor. shape descriptor is left to the
// status for these languages (cheap-and-conservative); only attaches when a
// status is found.
func attachBraceReturns(bf *BranchFacet, body []string, statusFn func(string) string, redirectRe *regexp.Regexp) {
	if bf.Outcome != OutcomeReturnValue && bf.Outcome != OutcomeRaise && bf.Outcome != OutcomeRedirect {
		return
	}
	joined := strings.Join(body, "\n")
	if st := statusFn(joined); st != "" {
		bf.Returns = &BranchReturns{Status: st}
	}
}

// guardKind picks env_gate / early_return / guard, threading the
// leading-guard flag exactly like the Python analyzer.
func guardKind(envVar string, firstGuardSeen *bool) BranchKind {
	kind := BranchGuard
	switch {
	case envVar != "":
		kind = BranchEnvGate
	case !*firstGuardSeen:
		kind = BranchEarlyReturn
	}
	*firstGuardSeen = true
	return kind
}

// matchEnvVar runs an env-ref regex over a condition and returns the first
// non-empty capture group (the env/setting name), or "".
func matchEnvVar(re *regexp.Regexp, cond string) string {
	m := re.FindStringSubmatch(cond)
	if m == nil {
		return ""
	}
	for _, g := range m[1:] {
		if g != "" {
			return g
		}
	}
	return ""
}

// afterCloseParen returns the text on a header line after the final `)`, used
// to detect an inline single-statement body (`if (x) return;`) or an opening
// brace. Empty when there is no `)`.
func afterCloseParen(ln string) string {
	idx := strings.LastIndex(ln, ")")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(ln[idx+1:])
}

// httpStatusNameToCode maps the common Spring HttpStatus / Go http.Status enum
// names to their numeric code. Conservative — only the codes a porting agent
// most needs; unknown names yield "".
func httpStatusNameToCode(name string) string {
	switch name {
	case "OK":
		return "200"
	case "CREATED":
		return "201"
	case "ACCEPTED":
		return "202"
	case "NO_CONTENT", "NOCONTENT":
		return "204"
	case "MOVED_PERMANENTLY", "MOVEDPERMANENTLY":
		return "301"
	case "FOUND":
		return "302"
	case "SEE_OTHER", "SEEOTHER":
		return "303"
	case "NOT_MODIFIED", "NOTMODIFIED":
		return "304"
	case "TEMPORARY_REDIRECT", "TEMPORARYREDIRECT":
		return "307"
	case "BAD_REQUEST", "BADREQUEST":
		return "400"
	case "UNAUTHORIZED":
		return "401"
	case "FORBIDDEN":
		return "403"
	case "NOT_FOUND", "NOTFOUND":
		return "404"
	case "METHOD_NOT_ALLOWED", "METHODNOTALLOWED":
		return "405"
	case "CONFLICT":
		return "409"
	case "GONE":
		return "410"
	case "UNPROCESSABLE_ENTITY", "UNPROCESSABLEENTITY":
		return "422"
	case "TOO_MANY_REQUESTS", "TOOMANYREQUESTS":
		return "429"
	case "INTERNAL_SERVER_ERROR", "INTERNALSERVERERROR":
		return "500"
	case "NOT_IMPLEMENTED", "NOTIMPLEMENTED":
		return "501"
	case "BAD_GATEWAY", "BADGATEWAY":
		return "502"
	case "SERVICE_UNAVAILABLE", "SERVICEUNAVAILABLE":
		return "503"
	case "GATEWAY_TIMEOUT", "GATEWAYTIMEOUT":
		return "504"
	}
	return ""
}

// camelToSnake converts a Go http.Status enum tail (e.g. "BadRequest",
// "InternalServerError") to SNAKE form so it can reuse httpStatusNameToCode's
// underscore keys. Cheap; handles ASCII camelCase only.
func camelToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return b.String()
}
