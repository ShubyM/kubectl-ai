import Link from "next/link";

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
import {
  collectLeaderboardEntries,
  loadLeaderboardData,
} from "@/lib/leaderboard";

const numberFormatter = new Intl.NumberFormat("en-US");

export const dynamic = "force-static";

function formatPercent(value: number) {
  if (!Number.isFinite(value)) {
    return "0%";
  }
  return `${(value * 100).toFixed(1)}%`;
}

export default async function HomePage() {
  const data = await loadLeaderboardData();
  const entries = data ? collectLeaderboardEntries(data) : [];

  return (
    <div className="min-h-screen bg-gradient-to-b from-zinc-50 via-white to-zinc-100 px-6 py-12 dark:from-zinc-950 dark:via-zinc-950 dark:to-black">
      <main className="mx-auto flex w-full max-w-6xl flex-col gap-10">
        <header className="space-y-3">
          <h1 className="text-4xl font-bold tracking-tight text-zinc-950 dark:text-zinc-50 md:text-5xl">
            Evaluation Leaderboard
          </h1>
          <p className="text-sm text-zinc-600 dark:text-zinc-400">
            Browse benchmark runs by model and agent. Select a row to inspect detailed task outcomes.
          </p>
        </header>

        <Card className="border-zinc-200/80 bg-white/90 dark:border-zinc-800/60 dark:bg-zinc-900/60">
          <CardHeader className="space-y-1">
            <CardTitle className="text-2xl font-semibold text-zinc-900 dark:text-zinc-50">
              Benchmark results
            </CardTitle>
            <CardDescription className="text-zinc-600 dark:text-zinc-400">
              Filter and investigate performance by drilling into individual runs.
            </CardDescription>
          </CardHeader>
          <CardContent className="mt-0">
            {entries.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Model</TableHead>
                    <TableHead>Agent</TableHead>
                    <TableHead className="text-right">Success</TableHead>
                    <TableHead className="text-right">Failure</TableHead>
                    <TableHead className="text-right">Success Rate</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {entries.map((entry) => {
                    const failure = entry.fail + entry.error;
                    return (
                      <Link key={entry.slug} href={`/run/${entry.slug}`} className="contents">
                        <TableRow className="cursor-pointer transition hover:bg-zinc-100/60 dark:hover:bg-zinc-800/40">
                          <TableCell className="font-medium text-zinc-900 dark:text-zinc-100">
                            {entry.model}
                          </TableCell>
                          <TableCell className="text-sm text-zinc-600 dark:text-zinc-400">
                            {entry.agent}
                          </TableCell>
                          <TableCell className="text-right text-emerald-600 dark:text-emerald-400">
                            {numberFormatter.format(entry.success)}
                          </TableCell>
                          <TableCell className="text-right text-rose-600 dark:text-rose-400">
                            {numberFormatter.format(failure)}
                            {entry.error > 0 ? (
                              <span className="block text-xs text-zinc-500 dark:text-zinc-400">
                                {numberFormatter.format(entry.error)} error
                                {entry.error === 1 ? "" : "s"}
                              </span>
                            ) : null}
                          </TableCell>
                          <TableCell className="text-right font-semibold text-emerald-700 dark:text-emerald-300">
                            {formatPercent(entry.successRate)}
                          </TableCell>
                        </TableRow>
                      </Link>
                    );
                  })}
                </TableBody>
              </Table>
            ) : (
              <p className="text-sm text-zinc-500 dark:text-zinc-400">
                Drop a generated <code className="rounded bg-zinc-200/60 px-1 dark:bg-zinc-800/80">leaderboard.json</code>{" "}
                file into <code className="rounded bg-zinc-200/60 px-1 dark:bg-zinc-800/80">public/</code> to see results.
              </p>
            )}
          </CardContent>
        </Card>
      </main>
    </div>
  );
}
