package substrate

// effect_sinks_querybuilder_4335_4336_test.go — fluent query-builder
// data-access reach (#4335 TypeORM/Prisma, #4336 Mongoose-aggregate/Knex/
// SQLAlchemy-Core), generalising the receiver-typed read/write discipline of
// #4691/#4692/#4320/#4737/#4736.
//
// Per builder, three shapes:
//   A — a builder chain ending in a READ terminal  → db_read
//   A'— a builder chain ending in a WRITE terminal → db_write
//   B — (negative) a plain array/Promise/collection chain whose terminal NAME
//       collides with a builder terminal (.first()/.execute()) → STAYS pure
//   C — a thin controller→service→builder-read method carries the db_read sink
//       (the per-method sink the CALLS-union propagates up to the endpoint).

import "testing"

// ----------------------------------------------------- #4335 TypeORM QueryBuilder

// A: createQueryBuilder().where().leftJoin().getMany()/getOne()/getRawMany() → read.
func TestTypeORMQueryBuilderRead_4335(t *testing.T) {
	src := `
class UserRepository {
  async listActive() {
    const qb = this.repo.createQueryBuilder('u');
    return qb.where('u.active = :a', { a: true }).leftJoinAndSelect('u.org', 'o').getMany();
  }
  async one(id) {
    return this.repo.createQueryBuilder('u').where('u.id = :id', { id }).getOne();
  }
  async raw() {
    const qb2 = this.repo.createQueryBuilder('u');
    return qb2.select('u.name').getRawMany();
  }
}`
	by := groupByEffect(sniffEffectsJSTS(src))
	mustHave(t, by, EffectDBRead, "listActive")
	mustHave(t, by, EffectDBRead, "one")
	mustHave(t, by, EffectDBRead, "raw")
}

// A': createQueryBuilder().update(...).set(...).execute() / .delete().execute() → write.
func TestTypeORMQueryBuilderWrite_4335(t *testing.T) {
	src := `
class UserRepository {
  async deactivate(id) {
    return this.repo.createQueryBuilder()
      .update(User).set({ active: false }).where('id = :id', { id }).execute();
  }
  async purge() {
    const qb = this.repo.createQueryBuilder();
    return qb.delete().from(User).where('stale = true').execute();
  }
}`
	by := groupByEffect(sniffEffectsJSTS(src))
	mustHave(t, by, EffectDBWrite, "deactivate")
	mustHave(t, by, EffectDBWrite, "purge")
}

// B (negative): a plain Promise chain ending in .execute() / array ending in
// .first() stays pure — the builder-typed-receiver gate.
func TestJSPlainChainNoFalsePositive_4335(t *testing.T) {
	src := `
function process(items, task) {
  const head = items.first;
  const out = items.map(x => x.id).filter(Boolean);
  return task.execute(out);
}`
	by := groupByEffect(sniffEffectsJSTS(src))
	mustNotHave(t, by, EffectDBRead, "process")
	mustNotHave(t, by, EffectDBWrite, "process")
}

// C: thin repo read method carries the db_read sink for the CALLS-union.
func TestTypeORMRepoReadChainSink_4335(t *testing.T) {
	src := `
class OrderRepository {
  async getOrders() {
    return this.repo.createQueryBuilder('o').where('o.open = true').getMany();
  }
}`
	by := groupByEffect(sniffEffectsJSTS(src))
	mustHave(t, by, EffectDBRead, "getOrders")
}

// ----------------------------------------------------------- #4335 Prisma fluent

// A: prisma.<model>.findMany/findUnique/findFirst (distinctive) + $queryRaw.
func TestPrismaFluentRead_4335(t *testing.T) {
	src := `
class UserService {
  async list() {
    return this.prisma.user.findMany({ where: { active: true }, include: { org: true } });
  }
  async byId(id) {
    return prisma.user.findUnique({ where: { id } });
  }
  async raw() {
    return this.prisma.$queryRaw` + "`SELECT * FROM users`" + `;
  }
}`
	by := groupByEffect(sniffEffectsJSTS(src))
	mustHave(t, by, EffectDBRead, "list")
	mustHave(t, by, EffectDBRead, "byId")
	mustHave(t, by, EffectDBRead, "raw")
}

// A': prisma.<model>.create/update/upsert/delete/createMany + $executeRaw.
func TestPrismaFluentWrite_4335(t *testing.T) {
	src := `
class UserService {
  async add(data) {
    return this.prisma.user.create({ data });
  }
  async bulk(rows) {
    return prisma.user.createMany({ data: rows });
  }
  async rawWrite() {
    return this.prisma.$executeRaw` + "`UPDATE users SET active = false`" + `;
  }
}`
	by := groupByEffect(sniffEffectsJSTS(src))
	mustHave(t, by, EffectDBWrite, "add")
	mustHave(t, by, EffectDBWrite, "bulk")
	mustHave(t, by, EffectDBWrite, "rawWrite")
}

// ------------------------------------------------ #4336 Mongoose aggregate/populate

// A: Model.aggregate([...]).lookup(...) and Model.find().populate(...) → read.
func TestMongooseAggregateRead_4336(t *testing.T) {
	src := `
class FeedRepo {
  async pipeline() {
    return this.model.aggregate([{ $match: {} }]).lookup({ from: 'authors', as: 'a' });
  }
  async withAuthors() {
    return this.model.find({ active: true }).populate('author');
  }
}`
	by := groupByEffect(sniffEffectsJSTS(src))
	mustHave(t, by, EffectDBRead, "pipeline")
	mustHave(t, by, EffectDBRead, "withAuthors")
}

// ------------------------------------------------------------------ #4336 Knex

// A: knex('t').where().select()/first()/pluck() → read.
func TestKnexBuilderRead_4336(t *testing.T) {
	src := `
class UserDao {
  async active() {
    return knex('users').where({ active: true }).select('id', 'name');
  }
  async newest() {
    const q = knex('users');
    return q.orderBy('created_at', 'desc').first();
  }
  async ids() {
    return knex('users').where('active', true).pluck('id');
  }
}`
	by := groupByEffect(sniffEffectsJSTS(src))
	mustHave(t, by, EffectDBRead, "active")
	mustHave(t, by, EffectDBRead, "newest")
	mustHave(t, by, EffectDBRead, "ids")
}

// A': knex('t').insert()/update()/del() → write.
func TestKnexBuilderWrite_4336(t *testing.T) {
	src := `
class UserDao {
  async add(row) {
    return knex('users').insert(row);
  }
  async deactivate(id) {
    const q = knex('users');
    return q.where('id', id).del();
  }
}`
	by := groupByEffect(sniffEffectsJSTS(src))
	mustHave(t, by, EffectDBWrite, "add")
	mustHave(t, by, EffectDBWrite, "deactivate")
}

// C: thin Knex repo read method carries db_read.
func TestKnexRepoReadChainSink_4336(t *testing.T) {
	src := `
class OrderDao {
  async getOrder(id) {
    return knex('orders').where('id', id).first();
  }
}`
	by := groupByEffect(sniffEffectsJSTS(src))
	mustHave(t, by, EffectDBRead, "getOrder")
}

// ---------------------------------------------------- #4336 SQLAlchemy Core (Python)

// A: conn.execute(select(...)) and session.execute(text("SELECT ...")) → read.
func TestSQLAlchemyCoreRead_4336(t *testing.T) {
	src := `
class UserRepo:
    def active(self):
        with self.engine.connect() as conn:
            return conn.execute(select(users).where(users.c.active == True)).fetchall()

    def raw(self):
        return self.session.execute(text("SELECT * FROM users"))
`
	by := groupByEffect(sniffEffectsPython(src))
	mustHave(t, by, EffectDBRead, "active")
	mustHave(t, by, EffectDBRead, "raw")
}

// A': conn.execute(insert(...))/update(...)/delete(...) and text("INSERT") → write.
func TestSQLAlchemyCoreWrite_4336(t *testing.T) {
	src := `
class UserRepo:
    def add(self, conn, data):
        return conn.execute(insert(users).values(**data))

    def deactivate(self, conn, uid):
        conn.execute(update(users).where(users.c.id == uid).values(active=False))

    def purge(self, conn):
        conn.execute(text("DELETE FROM users WHERE stale = 1"))
`
	by := groupByEffect(sniffEffectsPython(src))
	mustHave(t, by, EffectDBWrite, "add")
	mustHave(t, by, EffectDBWrite, "deactivate")
	mustHave(t, by, EffectDBWrite, "purge")
}

// C: thin SQLAlchemy Core repo read method carries db_read.
func TestSQLAlchemyCoreReadChainSink_4336(t *testing.T) {
	src := `
class OrderRepo:
    def get_order(self, conn, oid):
        return conn.execute(select(orders).where(orders.c.id == oid)).fetchone()
`
	by := groupByEffect(sniffEffectsPython(src))
	mustHave(t, by, EffectDBRead, "get_order")
}
