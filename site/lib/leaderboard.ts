import { promises as fs } from "fs";
import path from "path";

type ToolUseShimStatus = "enabled" | "disabled" | "unknown";

export interface TaskBreakdown {
  name: string;
  provider: string;
  model: string;
  result: string;
  failures?: { message: string }[];
  error?: string;
}

export interface RunSummary {
  id: string;
  agent: string;
  model: string;
  model_provider: string;
  total: number;
  num_success: number;
  num_failed: number;
  num_error: number;
  percentage: number;
  tasks: TaskBreakdown[];
}

export interface LeaderboardTotals {
  runs: number;
  success: number;
  fail: number;
  error: number;
  tasks: number;
  successRate: number;
}

export interface LeaderboardData {
  runs: RunSummary[];
  totals: LeaderboardTotals;
  generatedAt?: string;
}

export interface LeaderboardEntry {
  slug: string;
  provider: string;
  model: string;
  agent: string;
  categoryTitle: string;
  runs: number;
  success: number;
  fail: number;
  error: number;
  successRate: number;
  percentage: number;
  toolUseShim: ToolUseShimStatus;
}

export interface ProviderGroup {
  provider: string;
  entries: LeaderboardEntry[];
  totals: {
    runs: number;
    success: number;
    fail: number;
    error: number;
    successRate: number;
  };
}

export async function loadLeaderboardData(): Promise<LeaderboardData | null> {
  const filePath = path.join(process.cwd(), "public", "leaderboard.json");
  try {
    const raw = await fs.readFile(filePath, "utf-8");
    const parsed = JSON.parse(raw) as unknown;
    const runs = extractRuns(parsed);
    if (!runs) {
      return null;
    }
    const totals = computeTotals(runs);
    const generatedAt =
      parsed && typeof parsed === "object" && parsed !== null && "generatedAt" in parsed
        ? parseGeneratedAt((parsed as { generatedAt?: unknown }).generatedAt)
        : undefined;
    return { runs, totals, generatedAt };
  } catch (error: unknown) {
    if (
      typeof error === "object" &&
      error !== null &&
      "code" in error &&
      (error as { code?: string }).code === "ENOENT"
    ) {
      return null;
    }
    throw error;
  }
}

export async function loadLeaderboard(): Promise<RunSummary[]> {
  const data = await loadLeaderboardData();
  return data?.runs ?? [];
}

export function collectLeaderboardEntries(data: LeaderboardData): LeaderboardEntry[] {
  const entries = data.runs.map((run) => {
    const provider = formatProviderName(run.model_provider);
    const totalTasks = Math.max(run.total, run.num_success + run.num_failed + run.num_error);
    const successRate = totalTasks > 0 ? run.num_success / totalTasks : 0;
    return {
      slug: run.id,
      provider,
      model: run.model,
      agent: run.agent,
      categoryTitle: run.agent,
      runs: totalTasks,
      success: run.num_success,
      fail: run.num_failed,
      error: run.num_error,
      successRate,
      percentage: run.percentage,
      toolUseShim: "unknown" as ToolUseShimStatus,
    };
  });

  return entries.sort((a, b) => {
    if (b.successRate !== a.successRate) {
      return b.successRate - a.successRate;
    }
    return a.model.localeCompare(b.model);
  });
}

export function groupEntriesByProvider(entries: LeaderboardEntry[]): ProviderGroup[] {
  const groups = new Map<string, ProviderGroup>();

  for (const entry of entries) {
    const key = entry.provider || "Unknown";
    const group = groups.get(key);
    if (group) {
      group.entries.push(entry);
    } else {
      groups.set(key, {
        provider: key,
        entries: [entry],
        totals: {
          runs: 0,
          success: 0,
          fail: 0,
          error: 0,
          successRate: 0,
        },
      });
    }
  }

  const result = Array.from(groups.values());
  for (const group of result) {
    group.entries.sort((a, b) => {
      if (b.successRate !== a.successRate) {
        return b.successRate - a.successRate;
      }
      return a.model.localeCompare(b.model);
    });

    const totals = group.entries.reduce(
      (acc, entry) => {
        acc.runs += entry.runs;
        acc.success += entry.success;
        acc.fail += entry.fail;
        acc.error += entry.error;
        return acc;
      },
      { runs: 0, success: 0, fail: 0, error: 0 },
    );
    const totalTasks = totals.success + totals.fail + totals.error;
    group.totals = {
      runs: totals.runs,
      success: totals.success,
      fail: totals.fail,
      error: totals.error,
      successRate: totalTasks > 0 ? totals.success / totalTasks : 0,
    };
  }

  return result.sort((a, b) => a.provider.localeCompare(b.provider));
}

export function formatToolUseLabel(status: ToolUseShimStatus): string {
  switch (status) {
    case "enabled":
      return "Enabled";
    case "disabled":
      return "Disabled";
    default:
      return "Unknown";
  }
}

function extractRuns(value: unknown): RunSummary[] | null {
  if (Array.isArray(value) && value.every(isRunSummary)) {
    return value as RunSummary[];
  }

  if (value && typeof value === "object") {
    if (
      Array.isArray((value as { runs?: unknown }).runs) &&
      (value as { runs: unknown[] }).runs.every(isRunSummary)
    ) {
      return (value as { runs: RunSummary[] }).runs;
    }
    if (
      Array.isArray((value as { summaries?: unknown }).summaries) &&
      (value as { summaries: unknown[] }).summaries.every(isRunSummary)
    ) {
      return (value as { summaries: RunSummary[] }).summaries;
    }
  }

  return null;
}

function computeTotals(runs: RunSummary[]): LeaderboardTotals {
  const aggregate = runs.reduce(
    (acc, run) => {
      acc.runs += 1;
      acc.success += run.num_success;
      acc.fail += run.num_failed;
      acc.error += run.num_error;
      acc.tasks += Math.max(run.total, run.num_success + run.num_failed + run.num_error);
      return acc;
    },
    { runs: 0, success: 0, fail: 0, error: 0, tasks: 0 },
  );

  const denominator = aggregate.success + aggregate.fail + aggregate.error;
  return {
    ...aggregate,
    successRate: denominator > 0 ? aggregate.success / denominator : 0,
  };
}

function formatProviderName(value: string | undefined): string {
  if (!value) {
    return "Unknown";
  }
  return value
    .split(/[-_]/g)
    .filter(Boolean)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

function parseGeneratedAt(value: unknown): string | undefined {
  if (typeof value === "string" && value.trim().length > 0) {
    return value;
  }
  return undefined;
}

function isRunSummary(value: unknown): value is RunSummary {
  if (!value || typeof value !== "object") {
    return false;
  }
  const candidate = value as Partial<RunSummary>;
  return (
    typeof candidate.id === "string" &&
    typeof candidate.agent === "string" &&
    typeof candidate.model === "string" &&
    typeof candidate.model_provider === "string"
  );
}
