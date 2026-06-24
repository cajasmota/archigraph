package substrate

// effect_sinks_kysely_5491_test.go — the Kysely type-safe query-builder
// data-access layer (#5491). A Kysely query chain's ROOT method on the
// db/kysely/trx instance determines read vs write (selectFrom → db_read;
// insertInto/updateTable/deleteFrom/replaceInto → db_write), terminating in
// .execute()/.executeTakeFirst()/.executeTakeFirstOrThrow()/.stream(). The
// string-literal table arg is captured in the sink tag (kysely.read:user /
// kysely.write:post) and the effect is attributed to the enclosing function,
// mirroring the #5490 Prisma model-bearing uplift. The chain-root + receiver
// gate keeps an unrelated .execute() from being misread.

import "testing"

// kyselySinks returns the set of kysely sink tags carried by fn for effect eff.
func kyselySinks(ms []EffectMatch, fn string, eff Effect) map[string]bool {
	out := map[string]bool{}
	for _, m := range ms {
		if m.Function == fn && m.Effect == eff {
			out[m.Sink] = true
		}
	}
	return out
}

// The data-access layer: read (selectFrom), writes (insertInto/updateTable/
// deleteFrom), executeTakeFirst terminal, and a raw sql`…` query. Each effect
// lands on the right function with the right table in the sink tag.
const kyselyUserRepoTS = `
import { db } from './db';
import { sql } from 'kysely';

export async function getUsers() {
  return db.selectFrom("user").selectAll().execute();
}

export async function getUserById(id) {
  return db.selectFrom("user").where("id", "=", id).selectAll().executeTakeFirst();
}

export async function createPost(data) {
  return db.insertInto("post").values(data).execute();
}

export async function publishPost(id) {
  return db.updateTable("post").set({ published: true }).where("id", "=", id).execute();
}

export async function removePost(id) {
  return db.deleteFrom("post").where("id", "=", id).execute();
}

export async function activeCount() {
  return sql` + "`" + `SELECT count(*) FROM "user" WHERE active` + "`" + `.execute(db);
}

export async function archiveUser(id) {
  return sql` + "`" + `UPDATE "user" SET archived = true WHERE id = ${id}` + "`" + `.execute(db);
}
`

func TestKyselyReadEffects_5491(t *testing.T) {
	ms := sniffEffectsJSTS(kyselyUserRepoTS)
	by := groupByEffect(ms)
	mustHave(t, by, EffectDBRead, "getUsers")
	mustHave(t, by, EffectDBRead, "getUserById")

	if got := kyselySinks(ms, "getUsers", EffectDBRead); !got["kysely.read:user"] {
		t.Errorf("getUsers: expected sink kysely.read:user, got %v", got)
	}
	// executeTakeFirst terminal still credits the read with the table.
	if got := kyselySinks(ms, "getUserById", EffectDBRead); !got["kysely.read:user"] {
		t.Errorf("getUserById: expected sink kysely.read:user, got %v", got)
	}
}

func TestKyselyWriteEffects_5491(t *testing.T) {
	ms := sniffEffectsJSTS(kyselyUserRepoTS)
	by := groupByEffect(ms)
	mustHave(t, by, EffectDBWrite, "createPost")  // insertInto
	mustHave(t, by, EffectDBWrite, "publishPost") // updateTable
	mustHave(t, by, EffectDBWrite, "removePost")  // deleteFrom

	if got := kyselySinks(ms, "createPost", EffectDBWrite); !got["kysely.write:post"] {
		t.Errorf("createPost: expected sink kysely.write:post, got %v", got)
	}
	if got := kyselySinks(ms, "publishPost", EffectDBWrite); !got["kysely.write:post"] {
		t.Errorf("publishPost: expected sink kysely.write:post, got %v", got)
	}
	if got := kyselySinks(ms, "removePost", EffectDBWrite); !got["kysely.write:post"] {
		t.Errorf("removePost: expected sink kysely.write:post, got %v", got)
	}
}

// replaceInto is a write chain root too.
func TestKyselyReplaceInto_5491(t *testing.T) {
	src := `
export async function upsertRow(data) {
  return db.replaceInto("session").values(data).execute();
}
`
	ms := sniffEffectsJSTS(src)
	if got := kyselySinks(ms, "upsertRow", EffectDBWrite); !got["kysely.write:session"] {
		t.Errorf("upsertRow: expected sink kysely.write:session, got %v", got)
	}
}

// Raw sql`…`.execute(db) is classified by the leading SQL keyword.
func TestKyselyRawSQL_5491(t *testing.T) {
	ms := sniffEffectsJSTS(kyselyUserRepoTS)
	by := groupByEffect(ms)
	mustHave(t, by, EffectDBRead, "activeCount")  // SELECT → read
	mustHave(t, by, EffectDBWrite, "archiveUser") // UPDATE → write

	if got := kyselySinks(ms, "activeCount", EffectDBRead); !got["kysely.raw:read"] {
		t.Errorf("activeCount: expected sink kysely.raw:read, got %v", got)
	}
	if got := kyselySinks(ms, "archiveUser", EffectDBWrite); !got["kysely.raw:write"] {
		t.Errorf("archiveUser: expected sink kysely.raw:write, got %v", got)
	}
}

// trx (transaction handle) and kysely are gated receivers too.
func TestKyselyTrxAndKyselyReceiver_5491(t *testing.T) {
	src := `
export async function transfer(trx) {
  await trx.updateTable("account").set({ balance: 0 }).execute();
  return trx.selectFrom("ledger").selectAll().execute();
}
export async function listAll(kysely) {
  return kysely.selectFrom("widget").selectAll().execute();
}
`
	ms := sniffEffectsJSTS(src)
	if got := kyselySinks(ms, "transfer", EffectDBWrite); !got["kysely.write:account"] {
		t.Errorf("transfer write: expected sink kysely.write:account, got %v", got)
	}
	if got := kyselySinks(ms, "transfer", EffectDBRead); !got["kysely.read:ledger"] {
		t.Errorf("transfer read: expected sink kysely.read:ledger, got %v", got)
	}
	if got := kyselySinks(ms, "listAll", EffectDBRead); !got["kysely.read:widget"] {
		t.Errorf("listAll: expected sink kysely.read:widget, got %v", got)
	}
}

// Negative: a non-Kysely .execute() (a plain Promise/builder with no Kysely
// chain root and no gated receiver) must NOT earn a kysely.* sink.
func TestKyselyNonKyselyExecuteNotCredited_5491(t *testing.T) {
	src := `
export async function run(stmt) {
  await stmt.execute();                 // plain prepared-statement, no kysely root
  return somePromise.executeTakeFirst(); // not a kysely chain
}
`
	ms := sniffEffectsJSTS(src)
	for _, m := range ms {
		if m.Function == "run" && len(m.Sink) >= 7 && m.Sink[:7] == "kysely." {
			t.Errorf("non-kysely .execute() was misread as a Kysely effect: %+v", m)
		}
	}
}
