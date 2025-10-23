import Link from "next/link";
import { notFound } from "next/navigation";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { loadLeaderboard } from "@/lib/leaderboard";
import { cn } from "@/lib/utils";

const numberFormatter = new Intl.NumberFormat("en-US");

export const dynamic = "force-static";
export const dynamicParams = false;

type RunTask = Awaited<ReturnType<typeof loadLeaderboard>>[number]["tasks"][number];

function classifyTaskStatus(task: RunTask) {
  const value = (task.result ?? "").toLowerCase();
  if (value.includes("success")) {
    return "success";
  }
  if (value.includes("fail")) {
    return "fail";
  }
  if (value.includes("error")) {
    return "error";
  }
  if (task.error) {
    return "error";
  }
  if (task.failures && task.failures.length > 0) {
    return "fail";
  }
  return "error";
}

function formatPercent(value: number) {
  if (!Number.isFinite(value)) {
    return "0%";
  }
  return `${value.toFixed(1)}%`;
}

function formatProviderName(value: string) {
  if (!value) {
    return "Unknown provider";
  }
  return value
    .split(/[-_]/g)
    .filter(Boolean)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

export const generateStaticParams = async () => {
  const runs = await loadLeaderboard();
  return runs
    .filter((run) => typeof run.id === "string" && run.id.length > 0)
    .map((run) => ({ id: run.id }));
};

export default async function RunDetailPage({
  params,
}: {
  params: { id: string };
}) {
  const leaderboard = await loadLeaderboard();

  const resolvedParams = await params;
  const run = leaderboard.find((entry) => entry.id === resolvedParams.id);

  if (!run) {
    notFound();
  }

  const sortedTasks = [...run.tasks].sort((a, b) =>
    a.name.localeCompare(b.name, undefined, { sensitivity: "base" }),
  );

  return (
    <div className="min-h-screen bg-gradient-to-b from-zinc-50 via-white to-zinc-100 px-6 py-12 dark:from-zinc-950 dark:via-zinc-950 dark:to-black">
      <main className="mx-auto flex w-full max-w-4xl flex-col gap-10">
        <div className="flex items-center justify-between gap-4">
          <Button
            asChild
            variant="ghost"
            className="px-0 text-zinc-600 hover:text-zinc-900 dark:text-zinc-300 dark:hover:text-zinc-100"
          >
            <Link href="/">← Back to leaderboard</Link>
          </Button>
          <Badge variant="outline" className="border-emerald-200 px-3 py-1 text-xs uppercase tracking-[0.3em] text-emerald-700 dark:border-emerald-500/60 dark:text-emerald-300">
            {formatProviderName(run.model_provider)}
          </Badge>
        </div>

        <header className="space-y-6">
          <div className="space-y-2">
            <h1 className="text-4xl font-bold tracking-tight text-zinc-950 dark:text-zinc-50 md:text-5xl">
              {run.agent}
            </h1>
            <p className="text-base text-zinc-600 dark:text-zinc-400">
              {run.model} • {numberFormatter.format(run.num_success)} successes,{" "}
              {numberFormatter.format(run.num_failed)} failures,{" "}
              {numberFormatter.format(run.num_error)} errors.
            </p>
          </div>

          <Card className="border-zinc-200/80 bg-white/90 dark:border-zinc-800/60 dark:bg-zinc-900/60">
            <CardHeader className="space-y-1">
              <CardTitle className="text-2xl font-semibold text-zinc-900 dark:text-zinc-50">
                Run summary
              </CardTitle>
              <CardDescription className="text-zinc-600 dark:text-zinc-400">
                {numberFormatter.format(run.total)} tasks evaluated • Success rate{" "}
                {formatPercent(run.percentage)}
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-4 sm:grid-cols-3">
              <SummaryTile
                label="Successful"
                value={numberFormatter.format(run.num_success)}
                tone="emerald"
              />
              <SummaryTile
                label="Failed"
                value={numberFormatter.format(run.num_failed)}
                tone="rose"
              />
              <SummaryTile
                label="Errored"
                value={numberFormatter.format(run.num_error)}
                tone="amber"
              />
            </CardContent>
          </Card>
        </header>

        <Card className="border-zinc-200/80 bg-white/90 dark:border-zinc-800/60 dark:bg-zinc-900/60">
          <CardHeader className="space-y-1">
            <CardTitle className="text-2xl font-semibold text-zinc-900 dark:text-zinc-50">
              Task outcomes
            </CardTitle>
            <CardDescription className="text-zinc-600 dark:text-zinc-400">
              Review which benchmark tasks succeeded, failed, or surfaced infrastructure errors.
            </CardDescription>
          </CardHeader>
          <CardContent className="mt-0">
            {sortedTasks.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Task</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Result</TableHead>
                    <TableHead>Notes</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sortedTasks.map((task) => {
                    const status = classifyTaskStatus(task);
                    const badge = resolveStatusBadge(status);
                    return (
                      <TableRow key={task.name}>
                        <TableCell className="font-medium text-zinc-900 dark:text-zinc-100">
                          {task.name}
                        </TableCell>
                        <TableCell>
                          <Badge className={badge.className}>{badge.label}</Badge>
                        </TableCell>
                        <TableCell className="text-sm text-zinc-600 dark:text-zinc-400">
                          {task.result || "—"}
                        </TableCell>
                        <TableCell className="text-sm text-zinc-600 dark:text-zinc-400">
                          <TaskNotes task={task} />
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            ) : (
              <p className="text-sm text-zinc-600 dark:text-zinc-400">
                No recorded tasks for this run.
              </p>
            )}
          </CardContent>
        </Card>
      </main>
    </div>
  );
}

type SummaryTone = "emerald" | "rose" | "amber";

function SummaryTile({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: SummaryTone;
}) {
  const palette: Record<SummaryTone, string> = {
    emerald:
      "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/40 dark:bg-emerald-500/10 dark:text-emerald-200",
    rose:
      "border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-500/40 dark:bg-rose-500/10 dark:text-rose-200",
    amber:
      "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-200",
  };

  return (
    <div
      className={cn(
        "rounded-2xl border p-4 text-center text-sm font-medium shadow-sm",
        palette[tone],
      )}
    >
      <span className="text-xs uppercase tracking-[0.25em] opacity-70">{label}</span>
      <p className="mt-2 text-2xl font-semibold">{value}</p>
    </div>
  );
}

function TaskNotes({ task }: { task: RunTask }) {
  if (task.failures && task.failures.length > 0) {
    return (
      <ul className="space-y-1">
        {task.failures.map((failure, index) => (
          <li key={`${task.name}-failure-${index}`}>{failure.message}</li>
        ))}
      </ul>
    );
  }
  if (task.error) {
    return <span>{task.error}</span>;
  }
  return <span className="text-zinc-400 dark:text-zinc-500">—</span>;
}

function resolveStatusBadge(status: "success" | "fail" | "error") {
  switch (status) {
    case "success":
      return {
        label: "Success",
        className: "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300",
      };
    case "fail":
      return {
        label: "Failure",
        className: "bg-rose-100 text-rose-700 dark:bg-rose-500/20 dark:text-rose-300",
      };
    default:
      return {
        label: "Error",
        className: "bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-300",
      };
  }
}
