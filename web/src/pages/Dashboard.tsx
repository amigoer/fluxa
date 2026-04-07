import { useEffect, useState } from "react";
import {
  Activity,
  Coins,
  CalendarDays,
  CalendarClock,
  KeyRound,
  Server,
  Sigma,
  Waypoints,
  type LucideIcon,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Providers,
  Routes,
  Keys,
  Usage,
  type UsageSummary,
  type UsageTotals,
} from "@/lib/api";
import { useT, type TranslationKey } from "@/lib/i18n";

// Dashboard is the landing page: counts across the fleet plus a
// fleet-wide usage summary. Intentionally lightweight — anything more
// detailed lives inside its own tab so this view stays fast. The
// layout is two rows: three "tiles" (providers / routes / keys) on
// top, then two side-by-side usage cards (today / month).
export function DashboardPage() {
  const { t } = useT();
  const [counts, setCounts] = useState<{
    providers: number;
    routes: number;
    keys: number;
  } | null>(null);
  const [summary, setSummary] = useState<UsageSummary | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    (async () => {
      try {
        const [p, r, k, s] = await Promise.all([
          Providers.list(),
          Routes.list(),
          Keys.list(),
          Usage.summary(),
        ]);
        setCounts({ providers: p.length, routes: r.length, keys: k.length });
        setSummary(s);
      } catch (err) {
        setError(err instanceof Error ? err.message : t("common.loadFailed"));
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="space-y-8">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">
          {t("dashboard.title")}
        </h1>
        <p className="text-sm text-muted-foreground">
          {t("dashboard.subtitle")}
        </p>
      </header>

      {error && (
        <div className="rounded-md border border-destructive/40 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {/* Top tiles: provider / route / key counts. The icons mirror the
          sidebar navigation icons so the dashboard feels like a
          "table of contents" for the rest of the app. */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <Stat
          labelKey="dashboard.providers"
          value={counts?.providers}
          icon={Server}
        />
        <Stat
          labelKey="dashboard.routes"
          value={counts?.routes}
          icon={Waypoints}
        />
        <Stat
          labelKey="dashboard.keys"
          value={counts?.keys}
          icon={KeyRound}
        />
      </div>

      {/* Usage cards. Each card lists request count, token splits, and
          USD cost — same shape, different time window — so the
          metrics are visually comparable side by side. */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <UsageCard
          titleKey="dashboard.usageToday"
          icon={CalendarDays}
          totals={summary?.daily}
          loading={summary === null}
        />
        <UsageCard
          titleKey="dashboard.usageMonth"
          icon={CalendarClock}
          totals={summary?.monthly}
          loading={summary === null}
        />
      </div>
    </div>
  );
}

// Stat is a single big-number tile. While `value` is undefined we
// render a skeleton so the layout does not jump on data arrival.
function Stat({
  labelKey,
  value,
  icon: Icon,
}: {
  labelKey: TranslationKey;
  value: number | undefined;
  icon: LucideIcon;
}) {
  const { t } = useT();
  return (
    <Card className="hover:shadow-md">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
          {t(labelKey)}
        </CardTitle>
        <div className="flex h-8 w-8 items-center justify-center rounded-md bg-muted text-muted-foreground">
          <Icon className="h-4 w-4" />
        </div>
      </CardHeader>
      <CardContent>
        {value === undefined ? (
          <Skeleton className="h-9 w-16" />
        ) : (
          <div className="text-3xl font-semibold tabular-nums tracking-tight">
            {value}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// UsageCard renders one time-window column (daily or monthly). All
// totals share a single layout: a "headline" requests + cost row at
// the top, then a quiet token-breakdown grid below — keeps the most
// actionable numbers above the fold without a wall of identical rows.
function UsageCard({
  titleKey,
  icon: Icon,
  totals,
  loading,
}: {
  titleKey: TranslationKey;
  icon: LucideIcon;
  totals: UsageTotals | undefined;
  loading: boolean;
}) {
  const { t } = useT();
  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between space-y-0">
        <div className="space-y-1">
          <CardTitle className="text-base">{t(titleKey)}</CardTitle>
          <CardDescription>
            {t("dashboard.requests")} · {t("dashboard.totalTokens")} ·{" "}
            {t("dashboard.costUSD")}
          </CardDescription>
        </div>
        <div className="flex h-8 w-8 items-center justify-center rounded-md bg-muted text-muted-foreground">
          <Icon className="h-4 w-4" />
        </div>
      </CardHeader>
      <CardContent className="space-y-5">
        {/* Headline pair: requests on the left, cost on the right.
            Both numbers use tabular-nums so digits line up across
            cards even when the totals diverge by an order of
            magnitude. */}
        <div className="grid grid-cols-2 gap-4">
          <Headline
            labelKey="dashboard.requests"
            icon={Activity}
            value={loading ? undefined : (totals?.Requests ?? 0).toLocaleString()}
          />
          <Headline
            labelKey="dashboard.costUSD"
            icon={Coins}
            value={
              loading
                ? undefined
                : `$${(totals?.CostUSD ?? 0).toFixed(4)}`
            }
          />
        </div>

        <div className="h-px bg-border/60" />

        {/* Token breakdown: prompt / completion / total. Three columns
            on wide layouts, stacks on narrow. */}
        <div className="grid grid-cols-3 gap-4">
          <TokenStat
            labelKey="dashboard.promptTokens"
            value={totals?.PromptTokens}
            loading={loading}
          />
          <TokenStat
            labelKey="dashboard.completionTokens"
            value={totals?.CompletionTokens}
            loading={loading}
          />
          <TokenStat
            labelKey="dashboard.totalTokens"
            value={totals?.Tokens}
            loading={loading}
            icon={Sigma}
          />
        </div>
      </CardContent>
    </Card>
  );
}

// Headline is the prominent number row inside a UsageCard.
function Headline({
  labelKey,
  icon: Icon,
  value,
}: {
  labelKey: TranslationKey;
  icon: LucideIcon;
  value: string | undefined;
}) {
  const { t } = useT();
  return (
    <div className="space-y-1">
      <div className="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider text-muted-foreground">
        <Icon className="h-3.5 w-3.5" />
        {t(labelKey)}
      </div>
      {value === undefined ? (
        <Skeleton className="h-7 w-24" />
      ) : (
        <div className="text-2xl font-semibold tabular-nums tracking-tight">
          {value}
        </div>
      )}
    </div>
  );
}

// TokenStat is a single cell in the token-breakdown grid.
function TokenStat({
  labelKey,
  value,
  loading,
  icon: Icon,
}: {
  labelKey: TranslationKey;
  value: number | undefined;
  loading: boolean;
  icon?: LucideIcon;
}) {
  const { t } = useT();
  return (
    <div className="space-y-1">
      <div className="flex items-center gap-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
        {Icon && <Icon className="h-3 w-3" />}
        {t(labelKey)}
      </div>
      {loading ? (
        <Skeleton className="h-5 w-16" />
      ) : (
        <div className="text-base font-medium tabular-nums">
          {(value ?? 0).toLocaleString()}
        </div>
      )}
    </div>
  );
}
