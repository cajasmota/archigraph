package ruby_test

import (
	"testing"

	"github.com/cajasmota/archigraph/internal/types"

	_ "github.com/cajasmota/archigraph/internal/custom/ruby"
)

// findEntity locates a record by kind+name so ActiveJob queue/job_class
// attribution can be asserted on Properties (extractFull lives in
// observability_test.go).
func findEntity(ents []types.EntityRecord, kind, name string) *types.EntityRecord {
	for i := range ents {
		if ents[i].Kind == kind && ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

// TestActiveJobClassAndQueue: a `class SendWelcomeEmailJob < ApplicationJob`
// with `queue_as :mailers` yields a SCOPE.Service consumer carrying the queue
// attribution and job base, plus the perform handler tagged with the queue.
func TestActiveJobClassAndQueue(t *testing.T) {
	src := `class SendWelcomeEmailJob < ApplicationJob
  queue_as :mailers

  def perform(user_id)
    user = User.find(user_id)
    UserMailer.welcome(user).deliver_now
  end
end`
	ents := extractFull(t, "custom_ruby_rails", fi("app/jobs/send_welcome_email_job.rb", "ruby", src))

	svc := findEntity(ents, "SCOPE.Service", "SendWelcomeEmailJob")
	if svc == nil {
		t.Fatal("expected SCOPE.Service SendWelcomeEmailJob (ActiveJob consumer)")
	}
	if got := svc.Properties["framework"]; got != "activejob" {
		t.Errorf("framework = %q, want activejob", got)
	}
	if got := svc.Properties["queue"]; got != "mailers" {
		t.Errorf("queue = %q, want mailers", got)
	}
	if got := svc.Properties["job_base"]; got != "ApplicationJob" {
		t.Errorf("job_base = %q, want ApplicationJob", got)
	}

	perform := findEntity(ents, "SCOPE.Operation", "perform")
	if perform == nil {
		t.Fatal("expected perform SCOPE.Operation (consumer handler)")
	}
	if got := perform.Properties["queue"]; got != "mailers" {
		t.Errorf("perform queue = %q, want mailers", got)
	}
}

// TestActiveJobBaseClass: ActiveJob::Base (non-Rails-generated base) is also a
// recognised job class.
func TestActiveJobBaseClass(t *testing.T) {
	src := `class ReportJob < ActiveJob::Base
  queue_as "reports"
  def perform; end
end`
	ents := extractFull(t, "custom_ruby_rails", fi("app/jobs/report_job.rb", "ruby", src))
	svc := findEntity(ents, "SCOPE.Service", "ReportJob")
	if svc == nil {
		t.Fatal("expected SCOPE.Service ReportJob")
	}
	if got := svc.Properties["job_base"]; got != "ActiveJob::Base" {
		t.Errorf("job_base = %q, want ActiveJob::Base", got)
	}
	if got := svc.Properties["queue"]; got != "reports" {
		t.Errorf("queue = %q, want reports", got)
	}
}

// TestActiveJobProducerDispatch: `FooJob.perform_later(...)` and the
// `.set(...).perform_later` deferred form are the producer side that was
// previously missing entirely. Constant receiver => job_class attribution.
func TestActiveJobProducerDispatch(t *testing.T) {
	src := `class UsersController < ApplicationController
  def create
    user = User.create!(params)
    SendWelcomeEmailJob.perform_later(user.id)
    AuditJob.set(wait: 5.minutes).perform_later(user.id)
    ReportJob.perform_now
  end
end`
	ents := extractFull(t, "custom_ruby_rails", fi("app/controllers/users_controller.rb", "ruby", src))

	later := findEntity(ents, "SCOPE.Operation", "SendWelcomeEmailJob.perform_later")
	if later == nil {
		t.Fatal("expected SendWelcomeEmailJob.perform_later producer")
	}
	if got := later.Properties["job_class"]; got != "SendWelcomeEmailJob" {
		t.Errorf("job_class = %q, want SendWelcomeEmailJob", got)
	}
	if got := later.Properties["provenance"]; got != "INFERRED_FROM_ACTIVEJOB_DISPATCH" {
		t.Errorf("provenance = %q, want INFERRED_FROM_ACTIVEJOB_DISPATCH", got)
	}

	if findEntity(ents, "SCOPE.Operation", "AuditJob.perform_later") == nil {
		t.Error("expected AuditJob.perform_later (deferred .set(...) form)")
	}
	if findEntity(ents, "SCOPE.Operation", "ReportJob.perform_now") == nil {
		t.Error("expected ReportJob.perform_now producer")
	}
}

// TestActiveJobIgnoresLowercaseReceiver: a lowercase receiver is NOT an
// ActiveJob dispatch (it is a Sidekiq worker instance or local var); the
// activejob producer pass must not claim it.
func TestActiveJobIgnoresLowercaseReceiver(t *testing.T) {
	src := `worker.perform_later(1)`
	ents := extractFull(t, "custom_ruby_rails", fi("x.rb", "ruby", src))
	if findEntity(ents, "SCOPE.Operation", "worker.perform_later") != nil {
		t.Error("lowercase receiver must not be an ActiveJob dispatch")
	}
}
