package baseknowledge

// eloquent_pack.go — Laravel Eloquent knowledge pack (T9 #3841).
//
// A Laravel model `class Order extends Model {}` (Model =
// Illuminate\Database\Eloquent\Model) inherits the entire Eloquent finder +
// persistence API WITHOUT declaring any of it — the bodies live in the
// laravel/framework package, never in the indexed repo. Many of the "finders"
// (find/where/all) are static facades forwarded to the query builder; the MRO
// walk resolves by member NAME so we record them flat with the db effect.
//
// This ports the `internal/extractors/php/eloquent.go` Model knowledge (which
// only RECOGNISES the base) into the typed contract model, adding the per-
// method read/write effect.
//
// We do NOT fabricate HTTP statuses — Laravel's API-resource-controller 7-action
// route contract is T10 #3842's route-synthesis job. This pack is data-layer
// only.
//
// Sources: Laravel Eloquent docs (Retrieving/Inserting/Updating/Deleting
// Models) + Illuminate\Database\Eloquent\Model / Builder API.

const eloquentModelFQN = "Illuminate\\Database\\Eloquent\\Model"

func eloquentModelMembers(defining string) []Member {
	read := []string{
		"find", "findOrFail", "findMany", "first", "firstOrFail", "firstWhere",
		"all", "get", "where", "whereIn", "orderBy", "pluck", "count", "exists",
		"sum", "avg", "min", "max", "value", "paginate", "fresh", "refresh",
	}
	write := []string{
		"save", "update", "create", "delete", "destroy", "forceDelete",
		"insert", "increment", "decrement", "updateOrCreate", "firstOrCreate",
		"firstOrNew", "fill", "push", "touch", "restore", "truncate", "upsert",
	}
	out := make([]Member, 0, len(read)+len(write))
	for _, n := range read {
		out = append(out, dbRead(n, defining, "Eloquent query/finder"))
	}
	for _, n := range write {
		out = append(out, dbWrite(n, defining, "Eloquent persistence"))
	}
	return out
}

type eloquentPack struct{}

func (eloquentPack) Framework() string { return "eloquent" }

func (eloquentPack) Contracts() []BaseClassContract {
	ms := dataMembers(eloquentModelMembers(eloquentModelFQN)...)
	// leaf() splits on '.', so the backslashed FQN's leaf is the whole string;
	// list the bare "Model" spelling explicitly so `extends Model` resolves.
	return []BaseClassContract{
		dataContract("php", "eloquent", []string{eloquentModelFQN, "Eloquent\\Model", "Model"}, ms),
	}
}

func init() { Register(eloquentPack{}) }
