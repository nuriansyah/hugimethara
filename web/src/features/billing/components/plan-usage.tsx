"use client";

import { Check, Minus } from "lucide-react";
import { QueryError } from "@/components/query-error";
import { Badge } from "@hugimethara/ui/components/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@hugimethara/ui/components/card";
import { Progress } from "@hugimethara/ui/components/progress";
import { Skeleton } from "@hugimethara/ui/components/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@hugimethara/ui/components/table";
import { useEntitlements, usePlans, type EntitlementFeature } from "../api";

function formatPrice(cents: number | undefined, currency: string) {
  if (cents === undefined) return "Custom pricing";
  if (cents === 0) return "Free";
  return `${new Intl.NumberFormat(undefined, { style: "currency", currency }).format(cents / 100)} / month`;
}

export function PlanUsage() {
  const ent = useEntitlements();
  const plans = usePlans();

  if (ent.isPending) return <Skeleton className="h-64 w-full" />;
  if (ent.error) return <QueryError error={ent.error} onRetry={() => ent.refetch()} title="Could not load your plan" />;

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            {ent.data.plan.name} plan
            <Badge variant={ent.data.status === "active" ? "secondary" : "destructive"}>{ent.data.status}</Badge>
          </CardTitle>
          <CardDescription>
            Features and quotas available to this organization.
            {ent.data.current_period_end && ` Current period ends ${new Date(ent.data.current_period_end).toLocaleDateString()}.`}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <ul className="flex flex-col divide-y">
            {ent.data.features.map((f) => (
              <FeatureRow key={f.key} f={f} />
            ))}
          </ul>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Compare plans</CardTitle>
          <CardDescription>
            Plan changes are handled by your Hugimethara administrator; billing integration is coming.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {plans.isPending ? (
            <Skeleton className="h-40 w-full" />
          ) : plans.error ? (
            <QueryError error={plans.error} onRetry={() => plans.refetch()} title="Could not load plans" />
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Feature</TableHead>
                    {plans.data.data.map((p) => (
                      <TableHead key={p.key} className="text-center">
                        <span className="block font-semibold text-foreground">
                          {p.name} {p.key === ent.data.plan.key && <Badge className="ml-1">Current</Badge>}
                        </span>
                        <span className="block text-xs font-normal">{formatPrice(p.price_monthly_cents, p.currency)}</span>
                      </TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {plans.data.features.map((f) => (
                    <TableRow key={f.key}>
                      <TableCell className="text-sm">{f.name}</TableCell>
                      {plans.data.data.map((p) => {
                        const e = p.entitlements[f.key];
                        return (
                          <TableCell key={p.key} className="text-center text-sm">
                            {!e || !e.enabled ? (
                              <Minus className="mx-auto size-4 text-muted-foreground" aria-label="Not included" />
                            ) : f.kind === "flag" ? (
                              <Check className="mx-auto size-4" aria-label="Included" />
                            ) : e.limit === undefined ? (
                              "Unlimited"
                            ) : (
                              `${e.limit} ${f.unit}`
                            )}
                          </TableCell>
                        );
                      })}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function FeatureRow({ f }: { f: EntitlementFeature }) {
  const limited = f.kind === "limit" && f.limit !== undefined;
  const pct = limited && f.used !== undefined && f.limit! > 0 ? Math.min(100, Math.round((f.used / f.limit!) * 100)) : 0;
  return (
    <li className="flex flex-col gap-2 py-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <p className="flex items-center gap-2 text-sm font-medium">
          {f.name}
          {f.overridden && <Badge variant="outline">Custom</Badge>}
        </p>
        <p className="text-xs text-muted-foreground">{f.description}</p>
      </div>
      <div className="w-full text-sm sm:w-56 sm:text-right">
        {f.kind === "flag" ? (
          f.enabled ? <Badge variant="secondary">Included</Badge> : <Badge variant="outline">Not in plan</Badge>
        ) : !f.enabled ? (
          <Badge variant="outline">Not in plan</Badge>
        ) : f.limit === undefined ? (
          <span>{f.used !== undefined ? `${f.used} ${f.unit} · ` : ""}Unlimited</span>
        ) : f.used !== undefined ? (
          <div className="flex flex-col gap-1">
            <span>
              {f.used} of {f.limit} {f.unit}
            </span>
            <Progress value={pct} aria-label={`${f.name} usage`} />
          </div>
        ) : (
          <span>
            {f.limit} {f.unit}
          </span>
        )}
      </div>
    </li>
  );
}
