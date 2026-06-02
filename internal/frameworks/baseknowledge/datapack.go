package baseknowledge

// datapack.go — shared helpers for DATA-LAYER framework knowledge packs
// (Spring Data, ActiveRecord, Eloquent, TypeORM, NestJS Crud) added by T9
// (#3841).
//
// Unlike the DRF pack (route handlers carrying HTTP verbs + default statuses),
// these packs describe ORM/repository base classes whose inherited members are
// DATA accessors (save / findById / where / delete). For a data member we:
//
//   - NEVER fabricate an HTTP status: DefaultStatus stays StatusUnknown and
//     HTTPVerb stays "" (these are not route handlers).
//   - Encode the load-bearing DB EFFECT (read vs write) as a "db_read" /
//     "db_write" token at the HEAD of the Behaviour string. Member has no
//     Effect field, and adding one would ripple through the route-oriented
//     consumers; the effect token rides in Behaviour where the MRO walk and
//     effective_contract already surface it verbatim. Downstream callers that
//     want the effect parse the leading token (see dbEffect in the tests).
//
// This keeps the packs honest: we only encode the read/write effect, which is
// a verifiable property of each documented library method, and never invent a
// status a data method does not carry.

// dbRead builds a non-route data member whose effect is a read (find / query /
// count / exists). definingFQN is the framework base that owns the method body
// in the library; detail is appended to the "db_read" effect token.
func dbRead(name, definingFQN, detail string) Member {
	return dataMember(name, definingFQN, "db_read", detail)
}

// dbWrite builds a non-route data member whose effect is a write (save /
// update / delete / insert).
func dbWrite(name, definingFQN, detail string) Member {
	return dataMember(name, definingFQN, "db_write", detail)
}

func dataMember(name, definingFQN, effect, detail string) Member {
	beh := effect
	if detail != "" {
		beh = effect + " — " + detail
	}
	return Member{
		Name:          name,
		DefiningClass: definingFQN,
		HTTPVerb:      "", // data methods are not route handlers
		DefaultStatus: StatusUnknown,
		Behaviour:     beh,
	}
}

// dataMembers collects the given members into a name->Member map (reuses the
// route-pack `members` helper shape).
func dataMembers(ms ...Member) map[string]Member { return members(ms...) }

// dataContract builds a BaseClassContract for a data-layer base. fqns are the
// match keys (most-qualified first). lang is the source language; fwk the
// framework key.
func dataContract(lang, fwk string, fqns []string, ms map[string]Member) BaseClassContract {
	return BaseClassContract{FQNs: fqns, Language: lang, Framework: fwk, Members: ms}
}
