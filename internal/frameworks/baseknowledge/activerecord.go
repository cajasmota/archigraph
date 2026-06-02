package baseknowledge

// activerecord.go — Rails ActiveRecord knowledge pack (T9 #3841).
//
// A Rails model `class User < ApplicationRecord; end` inherits the entire
// ActiveRecord finder + persistence API from ActiveRecord::Base (via
// ApplicationRecord, the app's abstract base) WITHOUT declaring any of it. The
// bodies live in the activerecord gem, never in the indexed repo. The MRO walk
// needs each inherited finder/persistence method attributed to
// ActiveRecord::Base with its db read/write effect.
//
// ApplicationRecord is `class ApplicationRecord < ActiveRecord::Base;
// self.abstract_class = true; end` — it adds no members of its own, so it
// inherits the same surface as ActiveRecord::Base. We register BOTH spellings
// with the same effective member set so a `< ApplicationRecord` or a direct
// `< ActiveRecord::Base` resolves identically.
//
// NOTE: ActiveRecord finders/persistence are a mix of class methods (find,
// where, all, create) and instance methods (save, update, destroy, reload).
// The MRO walk resolves by member NAME, so we record both kinds in one set.
//
// We do NOT fabricate HTTP statuses — ActiveController's resourceful 7-action
// route contract (index/show/new/create/edit/update/destroy with statuses) is
// T10 #3842's route-synthesis job. This pack is data-layer only.
//
// Sources: Rails ActiveRecord::FinderMethods, ActiveRecord::Persistence,
// ActiveRecord::Querying, ActiveRecord::Relation Javadoc-equivalent (Rails API
// docs).

const activeRecordBase = "ActiveRecord::Base"

// activeRecordMembers is the effective ActiveRecord finder + persistence set a
// model inherits. defining is the attributed base (ActiveRecord::Base).
func activeRecordMembers(defining string) []Member {
	read := []string{
		"find", "find_by", "find_by!", "find_or_create_by", "find_or_initialize_by",
		"where", "all", "first", "last", "take", "find_each", "find_in_batches",
		"exists?", "count", "sum", "average", "minimum", "maximum", "pluck",
		"order", "limit", "select", "group", "joins", "includes", "none", "reload",
	}
	write := []string{
		"save", "save!", "update", "update!", "update_attribute", "update_column",
		"update_columns", "update_all", "create", "create!", "destroy", "destroy!",
		"destroy_all", "delete", "delete_all", "touch", "increment!", "decrement!",
		"toggle!", "insert", "insert_all", "upsert", "upsert_all",
	}
	out := make([]Member, 0, len(read)+len(write))
	for _, n := range read {
		out = append(out, dbRead(n, defining, "ActiveRecord query/finder"))
	}
	for _, n := range write {
		out = append(out, dbWrite(n, defining, "ActiveRecord persistence"))
	}
	return out
}

type activeRecordPack struct{}

func (activeRecordPack) Framework() string { return "activerecord" }

func (activeRecordPack) Contracts() []BaseClassContract {
	ms := func() map[string]Member { return dataMembers(activeRecordMembers(activeRecordBase)...) }
	return []BaseClassContract{
		// ActiveRecord::Base — the gem base. Leaf "Base" is too generic to index
		// as a bare leaf (collision risk), so we register only the namespaced
		// FQN spelling plus the colon-leaf "ActiveRecord::Base"; leaf() splits on
		// '.', not ':', so the whole "ActiveRecord::Base" stays the key.
		dataContract("ruby", "activerecord", []string{activeRecordBase}, ms()),
		// ApplicationRecord — the app abstract base every model extends.
		dataContract("ruby", "activerecord", []string{"ApplicationRecord"}, ms()),
	}
}

func init() { Register(activeRecordPack{}) }
