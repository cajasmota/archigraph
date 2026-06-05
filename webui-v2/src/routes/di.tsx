/* ============================================================
   Dependency-Injection view (#4266, epic #4249).

   Route: /g/:groupId/di

   The graph models DI via the INJECTED_INTO edge (provider
   INJECTED_INTO consumer), emitted by every framework's DI pass
   (NestJS / Angular / Spring / Micronaut / Quarkus / Guice /
   FastAPI / dependency-injector / ASP.NET / …). The force-graph
   renders those edges but is a poor surface for "which providers
   inject into which consumers": you cannot scan a provider's full
   consumer fan-out, nor group by framework.

   This screen surfaces that as a scannable DI map: providers
   grouped by DI framework, each provider listing every consumer it
   is injected into (the DI hubs — providers with the widest
   fan-out — sort first), with a Kind badge, the injection
   mechanism (constructor / field), a qualifier when present, and a
   RefLine source ref.

   Data: GET /api/di/{group} (handlers_di.go → handleDI), raw-JSON
   DIReport.

   Layout mirrors IaC / Data-flow: full-height column, a summary
   stat row, a framework filter, and a grouped provider → consumers
   list. Reuses Card / Badge / Pill / Skeleton + RefLine + RepoChip.

   HONESTY: only genuine INJECTED_INTO edges render. Endpoints that
   resolve to an entity get a real name + ref; unresolved ones show
   the edge's provider/consumer label or the raw key tail — never an
   invented name. A repo with no DI shows a clean empty state.
   ============================================================ */

import { useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import {
  Syringe,
  Boxes,
  ArrowRight,
  ChevronRight,
  ListFilter,
  AlertTriangle,
  Package,
} from "lucide-react";

import { Badge, Card, CardBody, Pill } from "@/components/ui";
import { Skeleton } from "@/components/ui/skeleton";
import { RefLine } from "@/components/RefLine";
import { RepoChip } from "@/lib/repo-color";
import { cn } from "@/lib/utils";
import { useDI } from "@/hooks/use-di";
import type { DIConsumer, DIProvider, DIReport } from "@/data/types";

type Tone = "neutral" | "accent" | "success" | "warning" | "danger" | "info";

// ---------------------------------------------------------------------------
// § Styling helpers
// ---------------------------------------------------------------------------

/** A short, readable label for a graph entity Kind (SCOPE.Controller → Controller). */
function kindLabel(kind?: string): string {
  if (!kind) return "";
  const tail = kind.includes(".") ? kind.split(".").pop()! : kind;
  return tail;
}

/** Entity-Kind → tone, so controllers vs services vs plain classes read distinctly. */
function kindTone(kind?: string): Tone {
  switch (kindLabel(kind).toLowerCase()) {
    case "controller":
    case "resolver":
    case "gateway":
      return "info";
    case "service":
    case "provider":
      return "accent";
    case "repository":
    case "datasource":
      return "success";
    default:
      return "neutral";
  }
}

/** DI framework → tone for the framework heading pill. */
function frameworkTone(fw: string): Tone {
  switch ((fw || "").toLowerCase()) {
    case "nestjs":
    case "angular":
      return "danger";
    case "spring":
    case "micronaut":
    case "quarkus":
    case "guice":
      return "success";
    case "fastapi":
    case "dependency-injector":
      return "info";
    default:
      return "neutral";
  }
}

// ---------------------------------------------------------------------------
// § Shared shells
// ---------------------------------------------------------------------------

function SkeletonRows({ n = 6 }: { n?: number }) {
  return (
    <div className="space-y-2">
      {Array.from({ length: n }).map((_, i) => (
        <div
          key={i}
          className="flex items-center gap-3 h-16 px-4 rounded-lg border border-border"
        >
          <Skeleton w="w-1/4" />
          <Skeleton w="w-6" h="h-2" />
          <Skeleton w="w-1/3" />
        </div>
      ))}
    </div>
  );
}

function EmptyState({ title, hint }: { title: string; hint: string }) {
  return (
    <div className="flex flex-col items-center py-16 text-center gap-3">
      <Syringe size={32} className="text-text-4" />
      <p className="text-md font-medium text-text">{title}</p>
      <p className="text-sm text-text-3 max-w-md">{hint}</p>
    </div>
  );
}

function ErrorState() {
  return (
    <div className="flex flex-col items-center py-16 text-center gap-3">
      <AlertTriangle size={32} className="text-danger" />
      <p className="text-md font-medium text-text">Could not load dependency-injection map</p>
      <p className="text-sm text-text-3 max-w-sm">
        The daemon returned an error. Confirm the group is indexed and the daemon
        is reachable, then retry.
      </p>
    </div>
  );
}

function SummaryStat({ label, value }: { label: string; value: number }) {
  return (
    <Card className={cn("flex-1 min-w-[120px]")}>
      <CardBody className="py-3">
        <p className="text-2xl font-semibold tabular-nums text-text">{value}</p>
        <p className="text-xs text-text-4 mt-0.5">{label}</p>
      </CardBody>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// § Endpoint label (provider / consumer)
// ---------------------------------------------------------------------------

function EntityName({
  name,
  kind,
  entityId,
}: {
  name: string;
  kind?: string;
  entityId: string;
}) {
  return (
    <span className="inline-flex items-center gap-1.5 min-w-0">
      <span className="font-mono text-sm text-text truncate" title={entityId}>
        {name}
      </span>
      {kind && (
        <Badge tone={kindTone(kind)} className="shrink-0">
          {kindLabel(kind)}
        </Badge>
      )}
    </span>
  );
}

function EntityRefLine({
  repo,
  sourceFile,
  startLine,
  name,
}: {
  repo?: string;
  sourceFile?: string;
  startLine?: number;
  name: string;
}) {
  if (!repo || !sourceFile) return null;
  return (
    <RefLine
      repo={repo}
      file={sourceFile}
      line={startLine ?? 0}
      name={name}
      className="text-[11px]"
    />
  );
}

// ---------------------------------------------------------------------------
// § One consumer row (provider INJECTED_INTO consumer)
// ---------------------------------------------------------------------------

function ConsumerRow({ consumer, groupId }: { consumer: DIConsumer; groupId: string }) {
  return (
    <div className="flex flex-col gap-0.5 pl-6 py-1">
      <div className="flex items-center gap-2 min-w-0 flex-wrap">
        <ArrowRight size={12} className="text-text-4 shrink-0" />
        <EntityName name={consumer.name} kind={consumer.kind} entityId={consumer.entity_id} />
        {consumer.via && (
          <span
            className="font-mono text-[10px] text-text-4 shrink-0"
            title={`injected via ${consumer.via}`}
          >
            {consumer.via}
          </span>
        )}
        {consumer.qualifier && (
          <span
            className="font-mono text-[10px] text-text-3 shrink-0"
            title={`DI qualifier / token: ${consumer.qualifier}`}
          >
            @{consumer.qualifier}
          </span>
        )}
        {consumer.repo && (
          <RepoChip slug={consumer.repo} groupId={groupId} maxLength={14} />
        )}
      </div>
      <EntityRefLine
        repo={consumer.repo}
        sourceFile={consumer.source_file}
        startLine={consumer.start_line}
        name={consumer.name}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// § One provider card (provider + the consumers it injects into)
// ---------------------------------------------------------------------------

const COLLAPSE_AT = 5; // collapse consumer lists longer than this

function ProviderCard({ provider, groupId }: { provider: DIProvider; groupId: string }) {
  const [open, setOpen] = useState(false);
  const many = provider.consumers.length > COLLAPSE_AT;
  const shown = many && !open ? provider.consumers.slice(0, COLLAPSE_AT) : provider.consumers;

  return (
    <Card>
      <CardBody className="py-2.5 space-y-1">
        {/* Provider header */}
        <div className="flex items-center gap-2 min-w-0 flex-wrap">
          <Syringe size={13} className="text-accent shrink-0" />
          <EntityName name={provider.name} kind={provider.kind} entityId={provider.entity_id} />
          {provider.repo && (
            <RepoChip slug={provider.repo} groupId={groupId} maxLength={14} />
          )}
          <span
            className="ml-auto text-[11px] text-text-4 tabular-nums shrink-0"
            title={`injected into ${provider.consumers.length} consumer(s)`}
          >
            {provider.consumers.length} consumer{provider.consumers.length === 1 ? "" : "s"}
          </span>
        </div>
        <EntityRefLine
          repo={provider.repo}
          sourceFile={provider.source_file}
          startLine={provider.start_line}
          name={provider.name}
        />

        {/* Consumers */}
        <div className="border-l border-border/60 ml-1">
          {shown.map((c) => (
            <ConsumerRow key={c.entity_id} consumer={c} groupId={groupId} />
          ))}
        </div>
        {many && (
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            className="inline-flex items-center gap-1 text-[11px] text-text-3 hover:text-text pl-6"
          >
            <ChevronRight size={12} className={cn("transition-transform", open && "rotate-90")} />
            {open
              ? "Show fewer"
              : `Show ${provider.consumers.length - COLLAPSE_AT} more consumer${
                  provider.consumers.length - COLLAPSE_AT === 1 ? "" : "s"
                }`}
          </button>
        )}
      </CardBody>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// § Screen
// ---------------------------------------------------------------------------

function DIBody({ data, groupId }: { data: DIReport; groupId: string }) {
  const [framework, setFramework] = useState<string>("all");

  const filteredGroups = useMemo(
    () =>
      framework === "all"
        ? data.groups
        : data.groups.filter((g) => g.framework === framework),
    [data.groups, framework],
  );

  return (
    <div className="flex-1 min-h-0 overflow-y-auto ag-scroll px-4 py-4 space-y-4">
      {/* Summary */}
      <div className="flex flex-wrap gap-3">
        <SummaryStat label="Providers" value={data.total_providers} />
        <SummaryStat label="Consumers" value={data.total_consumers} />
        <SummaryStat label="Injections" value={data.total_injections} />
        <SummaryStat label="Frameworks" value={data.frameworks.length} />
      </div>

      {/* Framework filter */}
      {data.groups.length > 1 && (
        <div className="flex flex-wrap items-center gap-2">
          <ListFilter size={13} className="text-text-4" />
          <Pill active={framework === "all"} onClick={() => setFramework("all")}>
            All ({data.total_providers})
          </Pill>
          {data.groups.map((g) => (
            <Pill
              key={g.framework || "—"}
              active={framework === g.framework}
              onClick={() => setFramework(g.framework)}
            >
              {g.framework || "unknown"} ({g.count})
            </Pill>
          ))}
        </div>
      )}

      {/* Provider → consumers, grouped by framework */}
      <div className="space-y-5">
        {filteredGroups.map((g) => (
          <section key={g.framework || "—"} className="space-y-2">
            <div className="flex items-center gap-2">
              <Package size={14} className="text-text-4" />
              <Badge tone={frameworkTone(g.framework)} className="uppercase">
                {g.framework || "unknown"}
              </Badge>
              <span className="text-[11px] text-text-4 tabular-nums">
                {g.count} provider{g.count === 1 ? "" : "s"}
              </span>
            </div>
            <div className="space-y-2">
              {g.providers.map((p) => (
                <ProviderCard key={`${g.framework}:${p.entity_id}`} provider={p} groupId={groupId} />
              ))}
            </div>
          </section>
        ))}
      </div>

      <p className="text-[10px] text-text-4">
        Each row is a genuine INJECTED_INTO edge: the provider (service / token)
        is injected into the consumer (controller / service / handler) that
        declares it as a constructor or field dependency. Providers with the
        widest consumer fan-out — the DI hubs — sort first.
      </p>
    </div>
  );
}

export default function DIScreen() {
  const { groupId = "" } = useParams<{ groupId: string }>();
  const { data, isLoading, isError } = useDI(groupId);

  const hasAny = (data?.total_injections ?? 0) > 0;

  return (
    <div className="flex flex-col h-full bg-bg">
      {isLoading ? (
        <div className="flex-1 min-h-0 overflow-y-auto ag-scroll px-4 py-4">
          <SkeletonRows />
        </div>
      ) : isError ? (
        <div className="flex-1 min-h-0 overflow-y-auto ag-scroll px-4 py-4">
          <ErrorState />
        </div>
      ) : !hasAny ? (
        <div className="flex-1 min-h-0 overflow-y-auto ag-scroll px-4 py-4">
          <EmptyState
            title="No dependency-injection wiring found"
            hint="No INJECTED_INTO edges were extracted for this group. The DI map appears once an indexed repo uses a DI framework (NestJS / Angular / Spring / Micronaut / Quarkus / Guice / FastAPI / dependency-injector / ASP.NET) — the framework's DI pass then records each provider → consumer injection."
          />
        </div>
      ) : (
        <>
          <div className="border-b border-border shrink-0 px-4 py-2.5 flex items-center gap-2">
            <Boxes size={15} className="text-text-3" />
            <h2 className="text-sm font-medium text-text">Dependency Injection</h2>
            <span className="text-[11px] text-text-4">
              providers → the consumers they inject into
            </span>
          </div>
          <DIBody data={data!} groupId={groupId} />
        </>
      )}
    </div>
  );
}
