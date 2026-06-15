package ruby_test

// auth_deep_test.go — value-asserting tests for the Rails auth deep extractor.
//
// These tests assert SPECIFIC entity properties (mechanism, action, resource,
// auth_required, policy_class, authenticatable, etc.) — NOT merely "≥1 entity
// exists". This brings Rails auth_coverage to the TS/JS bar as defined in
// internal/engine/http_endpoint_jsts_auth_test.go.
//
// Part of issue #3339.

import (
	"context"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

// extractAuth returns full EntityRecord slice from the ruby_auth extractor.
func extractAuth(t *testing.T, path, src string) []types.EntityRecord {
	t.Helper()
	e, ok := extreg.Get("custom_ruby_auth")
	if !ok {
		t.Fatal("custom_ruby_auth extractor not registered")
	}
	ents, err := e.Extract(context.Background(), extreg.FileInput{
		Path: path, Language: "ruby", Content: []byte(src),
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return ents
}

// findAuthEntity finds the first entity with the given name and subtype.
// Returns nil if not found.
func findAuthEntity(ents []types.EntityRecord, name, subtype string) *types.EntityRecord {
	for i := range ents {
		if ents[i].Name == name && ents[i].Subtype == subtype {
			return &ents[i]
		}
	}
	return nil
}

// findAuthEntityByName finds the first entity with the given name.
func findAuthEntityByName(ents []types.EntityRecord, name string) *types.EntityRecord {
	for i := range ents {
		if ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

// findAuthEntitiesBySubtype finds all entities with the given subtype.
func findAuthEntitiesBySubtype(ents []types.EntityRecord, subtype string) []types.EntityRecord {
	var out []types.EntityRecord
	for _, e := range ents {
		if e.Subtype == subtype {
			out = append(out, e)
		}
	}
	return out
}

// assertProp fails unless the entity has the expected property value.
func assertProp(t *testing.T, ent *types.EntityRecord, key, want string) {
	t.Helper()
	got := ent.Properties[key]
	if got != want {
		t.Errorf("entity %q: prop %q = %q, want %q (all props: %v)", ent.Name, key, got, want, ent.Properties)
	}
}

// ---------------------------------------------------------------------------
// Deep Devise
// ---------------------------------------------------------------------------

// TestRubyAuthDeep_DeviseModulesAuthenticatable asserts that the
// devise_modules entity correctly stamps authenticatable=true when
// :database_authenticatable is in the module list.
func TestRubyAuthDeep_DeviseModulesAuthenticatable(t *testing.T) {
	src := `
class User < ApplicationRecord
  devise :database_authenticatable, :registerable,
         :recoverable, :rememberable, :validatable
end
`
	ents := extractAuth(t, "app/models/user.rb", src)
	e := findAuthEntity(ents, "devise_modules", "auth_config")
	if e == nil {
		t.Fatal("expected devise_modules entity with subtype auth_config")
	}
	assertProp(t, e, "library", "devise")
	assertProp(t, e, "mechanism", "devise")
	assertProp(t, e, "authenticatable", "true")
	if e.Properties["modules_list"] == "" {
		t.Errorf("devise_modules: modules_list must not be empty")
	}
}

// TestRubyAuthDeep_DeviseModulesNonAuthenticatable verifies that
// authenticatable=false when :database_authenticatable is absent.
func TestRubyAuthDeep_DeviseModulesNonAuthenticatable(t *testing.T) {
	src := `
class User < ApplicationRecord
  devise :registerable, :trackable
end
`
	ents := extractAuth(t, "app/models/user.rb", src)
	e := findAuthEntity(ents, "devise_modules", "auth_config")
	if e == nil {
		t.Fatal("expected devise_modules entity")
	}
	assertProp(t, e, "authenticatable", "false")
}

// TestRubyAuthDeep_DeviseBeforeActionMechanism asserts that
// authenticate_user! emits mechanism=before_action and auth_required=true.
func TestRubyAuthDeep_DeviseBeforeActionMechanism(t *testing.T) {
	src := `
class ApplicationController < ActionController::Base
  before_action :authenticate_user!
end
`
	ents := extractAuth(t, "app/controllers/application_controller.rb", src)
	e := findAuthEntityByName(ents, "authenticate_user!")
	if e == nil {
		t.Fatal("expected authenticate_user! entity")
	}
	assertProp(t, e, "library", "devise")
	assertProp(t, e, "mechanism", "before_action")
	assertProp(t, e, "auth_required", "true")
	assertProp(t, e, "model", "user")
}

// TestRubyAuthDeep_DeviseSignedInHelper verifies that user_signed_in?
// is extracted with the correct model.
func TestRubyAuthDeep_DeviseSignedInHelper(t *testing.T) {
	src := `
Rails.application.routes.draw do
  devise_for :users
end

class DashboardController < ApplicationController
  def index
    if user_signed_in?
      @dashboard = Dashboard.for(current_user)
    end
  end
end
`
	ents := extractAuth(t, "app/controllers/dashboard_controller.rb", src)
	e := findAuthEntityByName(ents, "user_signed_in?")
	if e == nil {
		t.Fatal("expected user_signed_in? entity")
	}
	assertProp(t, e, "library", "devise")
	assertProp(t, e, "kind", "signed_in_helper")
	assertProp(t, e, "model", "user")
}

// ---------------------------------------------------------------------------
// Deep Pundit
// ---------------------------------------------------------------------------

// TestRubyAuthDeep_PunditPolicyClassAndActions asserts that a Pundit policy
// class emits: (a) the class-level entity with policy_class property, and
// (b) per-action entities (update?, create?, show?) with action+policy_class.
func TestRubyAuthDeep_PunditPolicyClassAndActions(t *testing.T) {
	src := `
class ProjectPolicy < ApplicationPolicy
  def update?
    user.admin? || record.owner == user
  end

  def create?
    user.admin?
  end

  def show?
    true
  end
end
`
	ents := extractAuth(t, "app/policies/project_policy.rb", src)

	// The policy class entity
	classEnt := findAuthEntityByName(ents, "ProjectPolicy")
	if classEnt == nil {
		t.Fatal("expected ProjectPolicy entity")
	}
	assertProp(t, classEnt, "library", "pundit")
	assertProp(t, classEnt, "kind", "policy_class_definition")
	assertProp(t, classEnt, "mechanism", "pundit")
	assertProp(t, classEnt, "policy_class", "ProjectPolicy")

	// update? action
	updateEnt := findAuthEntityByName(ents, "ProjectPolicy.update?")
	if updateEnt == nil {
		t.Fatal("expected ProjectPolicy.update? entity")
	}
	assertProp(t, updateEnt, "library", "pundit")
	assertProp(t, updateEnt, "kind", "policy_action")
	assertProp(t, updateEnt, "action", "update?")
	assertProp(t, updateEnt, "policy_class", "ProjectPolicy")
	assertProp(t, updateEnt, "mechanism", "pundit")

	// create? action
	createEnt := findAuthEntityByName(ents, "ProjectPolicy.create?")
	if createEnt == nil {
		t.Fatal("expected ProjectPolicy.create? entity")
	}
	assertProp(t, createEnt, "action", "create?")

	// show? action
	showEnt := findAuthEntityByName(ents, "ProjectPolicy.show?")
	if showEnt == nil {
		t.Fatal("expected ProjectPolicy.show? entity")
	}
	assertProp(t, showEnt, "action", "show?")
}

// TestRubyAuthDeep_PunditAuthorizeMechanism asserts that authorize calls
// emit mechanism=pundit and auth_required=true.
func TestRubyAuthDeep_PunditAuthorizeMechanism(t *testing.T) {
	src := `
class ProjectsController < ApplicationController
  include Pundit::Authorization

  def update
    @project = Project.find(params[:id])
    authorize @project
    @project.update(project_params)
  end

  def index
    @projects = policy_scope(Project)
  end
end
`
	ents := extractAuth(t, "app/controllers/projects_controller.rb", src)

	authEnt := findAuthEntityByName(ents, "authorize")
	if authEnt == nil {
		t.Fatal("expected authorize entity")
	}
	assertProp(t, authEnt, "library", "pundit")
	assertProp(t, authEnt, "mechanism", "pundit")
	assertProp(t, authEnt, "auth_required", "true")

	// Pundit::Policy sentinel from include Pundit::Authorization
	policyEnt := findAuthEntityByName(ents, "Pundit::Policy")
	if policyEnt == nil {
		t.Fatal("expected Pundit::Policy entity from include Pundit::Authorization")
	}
	assertProp(t, policyEnt, "library", "pundit")
	assertProp(t, policyEnt, "kind", "policy_class")
}

// TestRubyAuthDeep_PunditMultiplePolicies tests extraction of multiple
// policy classes in a shared policy file — verifies all class names extracted.
func TestRubyAuthDeep_PunditMultiplePolicies(t *testing.T) {
	src := `
class ArticlePolicy < ApplicationPolicy
  def update?
    user.admin?
  end
end

class CommentPolicy < ApplicationPolicy
  def destroy?
    user.admin? || record.author == user
  end
end
`
	ents := extractAuth(t, "app/policies/combined_policy.rb", src)

	articleClass := findAuthEntityByName(ents, "ArticlePolicy")
	if articleClass == nil {
		t.Fatal("expected ArticlePolicy entity")
	}
	assertProp(t, articleClass, "policy_class", "ArticlePolicy")

	commentClass := findAuthEntityByName(ents, "CommentPolicy")
	if commentClass == nil {
		t.Fatal("expected CommentPolicy entity")
	}
	assertProp(t, commentClass, "policy_class", "CommentPolicy")

	// Actions must carry their own policy class name
	updateEnt := findAuthEntityByName(ents, "ArticlePolicy.update?")
	if updateEnt == nil {
		t.Fatal("expected ArticlePolicy.update? entity")
	}
	assertProp(t, updateEnt, "policy_class", "ArticlePolicy")

	destroyEnt := findAuthEntityByName(ents, "CommentPolicy.destroy?")
	if destroyEnt == nil {
		t.Fatal("expected CommentPolicy.destroy? entity")
	}
	assertProp(t, destroyEnt, "policy_class", "CommentPolicy")
}

// ---------------------------------------------------------------------------
// Deep CanCanCan
// ---------------------------------------------------------------------------

// TestRubyAuthDeep_CanCanAbilityRules asserts that can/cannot rules emit
// the correct action+resource properties and in_ability_class=true.
func TestRubyAuthDeep_CanCanAbilityRules(t *testing.T) {
	src := `
class Ability
  include CanCan::Ability

  def initialize(user)
    if user.admin?
      can :manage, :all
    else
      can :read, Article
      can :create, Comment
      cannot :destroy, Article
    end
  end
end
`
	ents := extractAuth(t, "app/models/ability.rb", src)

	// Ability class sentinel
	abilityEnt := findAuthEntity(ents, "Ability", "auth_policy")
	if abilityEnt == nil {
		t.Fatal("expected Ability entity with subtype auth_policy")
	}
	assertProp(t, abilityEnt, "library", "cancancan")
	assertProp(t, abilityEnt, "kind", "ability_class")
	assertProp(t, abilityEnt, "mechanism", "cancancan")

	// can :manage, :all
	manageEnt := findAuthEntityByName(ents, "can :manage :all")
	if manageEnt == nil {
		// Try alternate name format
		manageEnt = findAuthEntityByName(ents, "can :manage :all")
	}
	// Find by scanning for action=manage
	var manageFound, readFound, createFound, cannotDestroyFound bool
	for _, e := range ents {
		if e.Subtype != "auth_policy" || e.Properties["kind"] != "ability_rule" {
			continue
		}
		switch e.Properties["action"] {
		case "manage":
			manageFound = true
			assertProp(t, &e, "mechanism", "cancancan")
			assertProp(t, &e, "in_ability_class", "true")
			assertProp(t, &e, "permission", "can")
		case "read":
			readFound = true
			assertProp(t, &e, "resource", "Article")
			assertProp(t, &e, "permission", "can")
		case "create":
			createFound = true
			assertProp(t, &e, "resource", "Comment")
		case "destroy":
			cannotDestroyFound = true
			assertProp(t, &e, "permission", "cannot")
			assertProp(t, &e, "resource", "Article")
		}
	}

	if !manageFound {
		t.Error("expected ability rule with action=manage")
	}
	if !readFound {
		t.Error("expected ability rule with action=read, resource=Article")
	}
	if !createFound {
		t.Error("expected ability rule with action=create, resource=Comment")
	}
	if !cannotDestroyFound {
		t.Error("expected ability rule with action=destroy, permission=cannot, resource=Article")
	}
}

// TestRubyAuthDeep_CanCanAuthorizeCheckMechanism asserts that authorize!
// emits mechanism=cancancan and auth_required=true.
func TestRubyAuthDeep_CanCanAuthorizeCheckMechanism(t *testing.T) {
	src := `
class ArticlesController < ApplicationController
  def update
    @article = Article.find(params[:id])
    authorize! :update, @article
    @article.update(article_params)
  end
end
`
	ents := extractAuth(t, "app/controllers/articles_controller.rb", src)
	e := findAuthEntityByName(ents, "authorize!")
	if e == nil {
		t.Fatal("expected authorize! entity")
	}
	assertProp(t, e, "library", "cancancan")
	assertProp(t, e, "mechanism", "cancancan")
	assertProp(t, e, "auth_required", "true")
}

// TestRubyAuthDeep_CanCanLoadAndAuthorize asserts load_and_authorize_resource
// emits mechanism=cancancan and auth_required=true.
func TestRubyAuthDeep_CanCanLoadAndAuthorize(t *testing.T) {
	src := `
class PostsController < ApplicationController
  load_and_authorize_resource

  def index
    # @posts is already loaded and authorized
  end
end
`
	ents := extractAuth(t, "app/controllers/posts_controller.rb", src)
	e := findAuthEntityByName(ents, "load_and_authorize_resource")
	if e == nil {
		t.Fatal("expected load_and_authorize_resource entity")
	}
	assertProp(t, e, "library", "cancancan")
	assertProp(t, e, "mechanism", "cancancan")
	assertProp(t, e, "auth_required", "true")
}

// ---------------------------------------------------------------------------
// Mixed: Devise + Pundit in same controller
// ---------------------------------------------------------------------------

// TestRubyAuthDeep_DevisePunditCombined verifies that a controller using both
// Devise (authenticate_user!) and Pundit (authorize) emits both mechanisms.
func TestRubyAuthDeep_DevisePunditCombined(t *testing.T) {
	src := `
class ProjectsController < ApplicationController
  include Pundit

  before_action :authenticate_user!

  def update
    @project = Project.find(params[:id])
    authorize @project
    @project.update(project_params)
  end

  def index
    @projects = policy_scope(Project)
  end
end
`
	ents := extractAuth(t, "app/controllers/projects_controller.rb", src)

	// Devise guard
	deviseEnt := findAuthEntityByName(ents, "authenticate_user!")
	if deviseEnt == nil {
		t.Fatal("expected authenticate_user! entity")
	}
	assertProp(t, deviseEnt, "library", "devise")
	assertProp(t, deviseEnt, "auth_required", "true")

	// Pundit authorize
	punditEnt := findAuthEntityByName(ents, "authorize")
	if punditEnt == nil {
		t.Fatal("expected authorize entity")
	}
	assertProp(t, punditEnt, "library", "pundit")
	assertProp(t, punditEnt, "auth_required", "true")
}

// ---------------------------------------------------------------------------
// Devise + CanCanCan combined (realistic app scenario)
// ---------------------------------------------------------------------------

// TestRubyAuthDeep_DeviseCanCanCombined verifies a realistic Devise+CanCanCan
// combination: model has devise modules + controller has load_and_authorize_resource.
func TestRubyAuthDeep_DeviseCanCanCombined(t *testing.T) {
	src := `
class Article < ApplicationRecord
  devise :database_authenticatable, :registerable, :validatable
end
`
	modelEnts := extractAuth(t, "app/models/article.rb", src)
	modEnt := findAuthEntity(modelEnts, "devise_modules", "auth_config")
	if modEnt == nil {
		t.Fatal("expected devise_modules entity in model")
	}
	assertProp(t, modEnt, "authenticatable", "true")

	ctrlSrc := `
class ArticlesController < ApplicationController
  load_and_authorize_resource
  before_action :authenticate_user!
end
`
	ctrlEnts := extractAuth(t, "app/controllers/articles_controller.rb", ctrlSrc)
	loadEnt := findAuthEntityByName(ctrlEnts, "load_and_authorize_resource")
	if loadEnt == nil {
		t.Fatal("expected load_and_authorize_resource entity")
	}
	assertProp(t, loadEnt, "mechanism", "cancancan")

	authEnt := findAuthEntityByName(ctrlEnts, "authenticate_user!")
	if authEnt == nil {
		t.Fatal("expected authenticate_user! entity")
	}
	assertProp(t, authEnt, "mechanism", "before_action")
}

// ---------------------------------------------------------------------------
// General auth before_action filters
// ---------------------------------------------------------------------------

// TestRubyAuthDeep_GeneralBeforeActionFilter verifies that generic auth
// before_action filters (require_auth, check_authentication, verify_auth)
// are extracted with auth_required=true.
func TestRubyAuthDeep_GeneralBeforeActionFilter(t *testing.T) {
	src := `
class ApiController < ApplicationController
  before_action :require_auth

  def index
    render json: { data: "secure" }
  end
end
`
	ents := extractAuth(t, "app/controllers/api_controller.rb", src)
	var found bool
	for _, e := range ents {
		if e.Subtype == "auth_guard" && e.Properties["kind"] == "before_action" &&
			e.Properties["auth_required"] == "true" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected an auth_guard entity with kind=before_action and auth_required=true for :require_auth")
	}
}
