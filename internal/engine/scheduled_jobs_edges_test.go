// Tests for the scheduled-job entry-point detection pass (#728).
//
// Each framework has at least one test covering a happy-path detection.
// Tests call applyScheduledJobEdges directly (same pattern as
// kafka_edges_test.go) so they run without the full YAML-rule compiler.
package engine

import (
	"strings"
	"testing"

	"github.com/cajasmota/archigraph/internal/types"
)

// runScheduledDetect is a lightweight in-process driver.
func runScheduledDetect(t *testing.T, lang, path, src string) ([]types.EntityRecord, []types.RelationshipRecord) {
	t.Helper()
	res := applyScheduledJobEdges(DetectorPassArgs{Lang: lang, Path: path, Content: []byte(src)})
	return res.Entities, res.Relationships
}

func scheduledJobsByFramework(ents []types.EntityRecord, framework string) []types.EntityRecord {
	var out []types.EntityRecord
	for _, e := range ents {
		if e.Kind == scheduledJobKind && e.Properties["framework"] == framework {
			out = append(out, e)
		}
	}
	return out
}

func triggersEdges(rels []types.RelationshipRecord) []types.RelationshipRecord {
	var out []types.RelationshipRecord
	for _, r := range rels {
		if r.Kind == triggersEdgeKind {
			out = append(out, r)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Python — Celery task decorator
// ---------------------------------------------------------------------------

func TestScheduledJobs_PyCelery_TaskDecorator(t *testing.T) {
	src := `import celery
app = celery.Celery('tasks', broker='redis://localhost')

@app.task
def send_daily_report():
    pass
`
	ents, rels := runScheduledDetect(t, "python", "tasks.py", src)
	jobs := scheduledJobsByFramework(ents, "celery")
	if len(jobs) == 0 {
		t.Fatalf("expected at least 1 celery ScheduledJob entity, got 0 (entities=%v)", ents)
	}
	found := false
	for _, j := range jobs {
		if j.Properties["handler"] == "send_daily_report" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected handler=send_daily_report in %v", jobs)
	}
	edges := triggersEdges(rels)
	if len(edges) == 0 {
		t.Fatalf("expected TRIGGERS edge, got none")
	}
}

// ---------------------------------------------------------------------------
// Python — Celery beat_schedule config dictionary
// ---------------------------------------------------------------------------

func TestScheduledJobs_PyCeleryBeat_ScheduleDict(t *testing.T) {
	src := `from celery.schedules import crontab

beat_schedule = {
    'generate-report': {
        'task': 'app.tasks.generate_report',
        'schedule': crontab(hour=0, minute=0),
    },
}
`
	ents, rels := runScheduledDetect(t, "python", "celeryconfig.py", src)
	jobs := scheduledJobsByFramework(ents, "celery_beat")
	if len(jobs) == 0 {
		t.Fatalf("expected at least 1 celery_beat ScheduledJob entity, got 0")
	}
	taskPath := jobs[0].Properties["task_path"]
	if taskPath != "app.tasks.generate_report" {
		t.Errorf("task_path = %q, want app.tasks.generate_report", taskPath)
	}
	_ = rels
}

// ---------------------------------------------------------------------------
// Python — APScheduler
// ---------------------------------------------------------------------------

func TestScheduledJobs_PyAPScheduler_CronJob(t *testing.T) {
	src := `from apscheduler.schedulers.background import BackgroundScheduler

scheduler = BackgroundScheduler()
scheduler.add_job(cleanup_old_records, trigger='cron', hour=2, minute=30)
scheduler.start()
`
	ents, rels := runScheduledDetect(t, "python", "scheduler.py", src)
	jobs := scheduledJobsByFramework(ents, "apscheduler")
	if len(jobs) == 0 {
		t.Fatalf("expected at least 1 apscheduler ScheduledJob entity, got 0")
	}
	if jobs[0].Properties["handler"] != "cleanup_old_records" {
		t.Errorf("handler = %q, want cleanup_old_records", jobs[0].Properties["handler"])
	}
	edges := triggersEdges(rels)
	if len(edges) == 0 {
		t.Fatalf("expected TRIGGERS edge from apscheduler job, got none")
	}
}

// ---------------------------------------------------------------------------
// Python — schedule library
// ---------------------------------------------------------------------------

func TestScheduledJobs_PyScheduleLib(t *testing.T) {
	src := `import schedule
import time

def send_heartbeat():
    requests.get('http://monitor/ping')

schedule.every(10).minutes.do(send_heartbeat)
`
	ents, rels := runScheduledDetect(t, "python", "heartbeat.py", src)
	jobs := scheduledJobsByFramework(ents, "schedule_lib")
	if len(jobs) == 0 {
		t.Fatalf("expected at least 1 schedule_lib ScheduledJob, got 0")
	}
	if jobs[0].Properties["schedule"] != "every(10).minutes" {
		t.Errorf("schedule = %q, want every(10).minutes", jobs[0].Properties["schedule"])
	}
	_ = rels
}

// ---------------------------------------------------------------------------
// Node — node-cron
// ---------------------------------------------------------------------------

func TestScheduledJobs_NodeCron_Schedule(t *testing.T) {
	src := `const cron = require('node-cron');

cron.schedule('0 0 * * *', sendNightlyDigest);
`
	ents, rels := runScheduledDetect(t, "javascript", "cron.js", src)
	jobs := scheduledJobsByFramework(ents, "node_cron")
	if len(jobs) == 0 {
		t.Fatalf("expected at least 1 node_cron ScheduledJob, got 0")
	}
	if jobs[0].Properties["schedule"] != "0 0 * * *" {
		t.Errorf("schedule = %q, want 0 0 * * *", jobs[0].Properties["schedule"])
	}
	_ = rels
}

// ---------------------------------------------------------------------------
// Node — bull/bullmq repeat job
// ---------------------------------------------------------------------------

func TestScheduledJobs_NodeBull_RepeatCron(t *testing.T) {
	src := `const Queue = require('bull');
const emailQueue = new Queue('emails');

emailQueue.add('sendWeeklyReport', { to: 'all' }, {
  repeat: { cron: '0 9 * * 1' }
});
`
	ents, rels := runScheduledDetect(t, "javascript", "queue.js", src)
	jobs := scheduledJobsByFramework(ents, "bullmq")
	if len(jobs) == 0 {
		t.Fatalf("expected at least 1 bullmq ScheduledJob, got 0")
	}
	if jobs[0].Properties["schedule"] != "0 9 * * 1" {
		t.Errorf("schedule = %q, want 0 9 * * 1", jobs[0].Properties["schedule"])
	}
	_ = rels
}

// ---------------------------------------------------------------------------
// Java — Spring @Scheduled
// ---------------------------------------------------------------------------

func TestScheduledJobs_SpringScheduled_Cron(t *testing.T) {
	src := `package com.example;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

@Component
public class ReportService {

    @Scheduled(cron = "0 0 2 * * *")
    public void generateNightlyReport() {
        // ...
    }
}
`
	ents, rels := runScheduledDetect(t, "java", "ReportService.java", src)
	jobs := scheduledJobsByFramework(ents, "spring_scheduled")
	if len(jobs) == 0 {
		t.Fatalf("expected at least 1 spring_scheduled ScheduledJob, got 0")
	}
	sched := jobs[0].Properties["schedule"]
	if !strings.Contains(sched, "0 0 2 * * *") {
		t.Errorf("schedule %q does not contain cron expression", sched)
	}
	edges := triggersEdges(rels)
	if len(edges) == 0 {
		t.Fatalf("expected TRIGGERS edge from Spring @Scheduled job, got none")
	}
}

// ---------------------------------------------------------------------------
// Java — Spring @Scheduled with fixedRate
// ---------------------------------------------------------------------------

func TestScheduledJobs_SpringScheduled_FixedRate(t *testing.T) {
	src := `import org.springframework.scheduling.annotation.Scheduled;

public class HealthChecker {
    @Scheduled(fixedRate = 30000)
    public void checkHealth() {}
}
`
	ents, rels := runScheduledDetect(t, "java", "HealthChecker.java", src)
	jobs := scheduledJobsByFramework(ents, "spring_scheduled")
	if len(jobs) == 0 {
		t.Fatalf("expected fixedRate Spring job, got 0")
	}
	_ = rels
}

// ---------------------------------------------------------------------------
// Java — Quartz
// ---------------------------------------------------------------------------

func TestScheduledJobs_JavaQuartz(t *testing.T) {
	src := `import org.quartz.*;

JobDetail job = JobBuilder.newJob(EmailJob.class)
    .withIdentity("emailJob").build();

Trigger trigger = TriggerBuilder.newTrigger()
    .withSchedule(CronScheduleBuilder.cronSchedule("0 0 0 * * ?"))
    .build();

scheduler.scheduleJob(job, trigger);
`
	ents, rels := runScheduledDetect(t, "java", "JobScheduler.java", src)
	jobs := scheduledJobsByFramework(ents, "quartz")
	if len(jobs) == 0 {
		t.Fatalf("expected at least 1 quartz ScheduledJob, got 0")
	}
	if jobs[0].Properties["handler"] != "EmailJob" {
		t.Errorf("handler = %q, want EmailJob", jobs[0].Properties["handler"])
	}
	_ = rels
}

// ---------------------------------------------------------------------------
// Go — robfig/cron
// ---------------------------------------------------------------------------

func TestScheduledJobs_GoCron_AddFunc(t *testing.T) {
	src := `package main

import (
    "github.com/robfig/cron/v3"
)

func main() {
    c := cron.New()
    c.AddFunc("0 0 * * *", cleanupExpiredSessions)
    c.Start()
}
`
	ents, rels := runScheduledDetect(t, "go", "main.go", src)
	jobs := scheduledJobsByFramework(ents, "go_cron")
	if len(jobs) == 0 {
		t.Fatalf("expected at least 1 go_cron ScheduledJob, got 0")
	}
	if jobs[0].Properties["schedule"] != "0 0 * * *" {
		t.Errorf("schedule = %q, want 0 0 * * *", jobs[0].Properties["schedule"])
	}
	edges := triggersEdges(rels)
	if len(edges) == 0 {
		t.Fatalf("expected TRIGGERS edge for go_cron, got none")
	}
}

// ---------------------------------------------------------------------------
// Kubernetes CronJob YAML
// ---------------------------------------------------------------------------

func TestScheduledJobs_K8sCronJob_YAML(t *testing.T) {
	src := `apiVersion: batch/v1
kind: CronJob
metadata:
  name: report-generator
spec:
  schedule: "0 2 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: reporter
            image: myapp/reporter:latest
`
	ents, rels := runScheduledDetect(t, "", "k8s/cronjob.yaml", src)
	jobs := scheduledJobsByFramework(ents, "kubernetes_cronjob")
	if len(jobs) == 0 {
		t.Fatalf("expected at least 1 kubernetes_cronjob ScheduledJob, got 0")
	}
	if jobs[0].Properties["schedule"] != "0 2 * * *" {
		t.Errorf("schedule = %q, want 0 2 * * *", jobs[0].Properties["schedule"])
	}
	_ = rels
}

// ---------------------------------------------------------------------------
// GitHub Actions schedule trigger
// ---------------------------------------------------------------------------

func TestScheduledJobs_GitHubActionsSchedule(t *testing.T) {
	src := `name: Nightly Build

on:
  schedule:
    - cron: '0 3 * * *'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
`
	ents, rels := runScheduledDetect(t, "", ".github/workflows/nightly.yml", src)
	jobs := scheduledJobsByFramework(ents, "github_actions_schedule")
	if len(jobs) == 0 {
		t.Fatalf("expected at least 1 github_actions_schedule ScheduledJob, got 0")
	}
	if jobs[0].Properties["schedule"] != "0 3 * * *" {
		t.Errorf("schedule = %q, want 0 3 * * *", jobs[0].Properties["schedule"])
	}
	_ = rels
}

// ---------------------------------------------------------------------------
// Dedup: same job registered twice should yield only one entity
// ---------------------------------------------------------------------------

func TestScheduledJobs_Dedup(t *testing.T) {
	src := `import schedule

def do_work(): pass

schedule.every(5).minutes.do(do_work)
schedule.every(5).minutes.do(do_work)
`
	ents, _ := runScheduledDetect(t, "python", "worker.py", src)
	jobs := scheduledJobsByFramework(ents, "schedule_lib")
	if len(jobs) != 1 {
		t.Errorf("expected 1 deduplicated job entity, got %d", len(jobs))
	}
}

// ---------------------------------------------------------------------------
// Non-match: file with no scheduler content emits nothing
// ---------------------------------------------------------------------------

func TestScheduledJobs_NoMatch(t *testing.T) {
	src := `package main

import "fmt"

func main() {
    fmt.Println("hello world")
}
`
	ents, rels := runScheduledDetect(t, "go", "main.go", src)
	if len(ents) != 0 || len(rels) != 0 {
		t.Errorf("expected no entities/rels for unrelated file, got %d/%d", len(ents), len(rels))
	}
}
