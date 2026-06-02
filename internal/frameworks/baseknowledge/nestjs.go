package baseknowledge

// nestjs.go — TypeORM + NestJS @nestjsx/crud knowledge pack (T9 #3841).
//
// Two TypeScript data bases whose subclasses inherit a fixed data API without
// declaring it:
//
//  1. TypeORM `Repository<Entity>` (typeorm) — a custom repository
//     `class UserRepository extends Repository<User> {}` (or the injected
//     `Repository<User>`) inherits save/find/findOne/remove/delete/... whose
//     bodies live in the typeorm package.
//
//  2. NestJS `@nestjsx/crud` `TypeOrmCrudService<T>` — a
//     `class UsersService extends TypeOrmCrudService<User> {}` inherits the
//     CRUD service surface (getMany/getOne/createOne/updateOne/deleteOne/...)
//     backing the `@Crud()` controller. The controller's HTTP verbs + statuses
//     are route-synthesis (T10 #3842); here we record only the SERVICE-method
//     data effects, which are verifiable from the library.
//
// We do NOT fabricate HTTP statuses — these are data-service methods. The
// @Crud() controller route contract is deferred to T10.
//
// Sources: TypeORM Repository API; @nestjsx/crud TypeOrmCrudService /
// CrudService API.

const (
	typeormRepoFQN = "typeorm.Repository"
	nestjsxCrudFQN = "@nestjsx/crud-typeorm.TypeOrmCrudService"
	nestjsxBaseFQN = "@nestjsx/crud.CrudService"
)

func typeormRepositoryMembers(defining string) []Member {
	read := []string{
		"find", "findBy", "findOne", "findOneBy", "findOneOrFail", "findOneByOrFail",
		"findAndCount", "findAndCountBy", "count", "countBy", "exists", "existsBy",
		"sum", "average", "minimum", "maximum", "query", "createQueryBuilder", "preload",
	}
	write := []string{
		"save", "insert", "update", "upsert", "remove", "delete", "softDelete",
		"softRemove", "recover", "restore", "increment", "decrement", "clear",
	}
	out := make([]Member, 0, len(read)+len(write))
	for _, n := range read {
		out = append(out, dbRead(n, defining, "TypeORM repository query"))
	}
	for _, n := range write {
		out = append(out, dbWrite(n, defining, "TypeORM repository write"))
	}
	return out
}

func nestjsxCrudServiceMembers(defining string) []Member {
	return []Member{
		dbRead("getMany", defining, "@nestjsx/crud paginated/list fetch"),
		dbRead("getOne", defining, "@nestjsx/crud single-entity fetch"),
		dbWrite("createOne", defining, "@nestjsx/crud create a single entity"),
		dbWrite("createMany", defining, "@nestjsx/crud bulk create"),
		dbWrite("updateOne", defining, "@nestjsx/crud partial update of one entity"),
		dbWrite("replaceOne", defining, "@nestjsx/crud full replace (PUT) of one entity"),
		dbWrite("deleteOne", defining, "@nestjsx/crud delete a single entity"),
		dbWrite("recoverOne", defining, "@nestjsx/crud restore a soft-deleted entity"),
	}
}

type nestjsPack struct{}

func (nestjsPack) Framework() string { return "nestjsx-crud" }

func (nestjsPack) Contracts() []BaseClassContract {
	return []BaseClassContract{
		dataContract("typescript", "typeorm", []string{typeormRepoFQN, "Repository"},
			dataMembers(typeormRepositoryMembers(typeormRepoFQN)...)),
		dataContract("typescript", "nestjsx-crud", []string{nestjsxCrudFQN, "TypeOrmCrudService"},
			dataMembers(nestjsxCrudServiceMembers(nestjsxCrudFQN)...)),
		dataContract("typescript", "nestjsx-crud", []string{nestjsxBaseFQN, "CrudService"},
			dataMembers(nestjsxCrudServiceMembers(nestjsxBaseFQN)...)),
	}
}

func init() { Register(nestjsPack{}) }
