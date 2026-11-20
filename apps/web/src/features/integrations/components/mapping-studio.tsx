"use client";

import { Braces, FlaskConical, Plus, Rocket, Trash2 } from "lucide-react";
import { useMemo, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { PageHeader } from "@/components/page-header";
import { QueryError } from "@/components/query-error";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { useCan } from "@/features/access/api";
import { useWorkspaces } from "@/features/workspaces/api";
import { toastError } from "@/lib/errors";
import {
  useMappingMutations,
  useMappings,
  useMappingTests,
  useMappingVersions,
  type Mapping,
} from "../api";

const emptyDraft = '{\n  "rules": [\n    { "target": "name", "source": "$.customer_name", "transforms": [{ "op": "trim" }] }\n  ]\n}';

export function MappingStudio() {
  const can = useCan();
  const workspaces = useWorkspaces(false);
  const [workspaceId, setWorkspaceId] = useState("");
  const activeWorkspace = workspaces.data?.find((item) => item.id === workspaceId) ?? workspaces.data?.[0];
  const resolvedWorkspaceId = activeWorkspace?.id ?? "";
  const mappings = useMappings(resolvedWorkspaceId);
  const [selectedId, setSelectedId] = useState("");
  const selected = mappings.data?.find((item) => item.id === selectedId) ?? mappings.data?.[0];
  const [creating, setCreating] = useState(false);

  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      <PageHeader title="Mapping studio" description="Transform API payloads with explicit rules, replayable tests, and published versions." />
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-3">
          <div>
            <CardTitle>Mappings</CardTitle>
            <CardDescription>Draft transformations, test them against JSON, then publish a frozen version.</CardDescription>
          </div>
          <div className="flex gap-2">
            <Select value={resolvedWorkspaceId} onValueChange={setWorkspaceId}>
              <SelectTrigger aria-label="Workspace" className="w-52"><SelectValue placeholder="Choose workspace" /></SelectTrigger>
              <SelectContent>{workspaces.data?.map((item) => <SelectItem key={item.id} value={item.id}>{item.name}</SelectItem>)}</SelectContent>
            </Select>
            {activeWorkspace && can("mapping:create", activeWorkspace.id) && (
              <Button onClick={() => setCreating(true)}><Plus aria-hidden /> New mapping</Button>
            )}
          </div>
        </CardHeader>
        <CardContent>
          {workspaces.isPending ? <Skeleton className="h-32 w-full" /> : workspaces.error ? (
            <QueryError error={workspaces.error} onRetry={() => workspaces.refetch()} title="Could not load workspaces" />
          ) : !resolvedWorkspaceId ? (
            <div className="rounded-lg border border-dashed p-9 text-center text-sm text-muted-foreground">Choose a workspace to open its mappings.</div>
          ) : mappings.isPending ? <Skeleton className="h-32 w-full" /> : mappings.error ? (
            <QueryError error={mappings.error} onRetry={() => mappings.refetch()} title="Could not load mappings" />
          ) : mappings.data.length === 0 ? (
            <div className="rounded-lg border border-dashed p-9 text-center">
              <Braces className="mx-auto mb-3 size-5 text-muted-foreground" aria-hidden />
              <p className="text-sm font-medium">No mappings yet</p>
              <p className="mt-1 text-sm text-muted-foreground">Create a draft to start shaping API responses.</p>
            </div>
          ) : (
            <div className="grid gap-4 lg:grid-cols-[230px_minmax(0,1fr)]">
              <div className="space-y-2">
                {mappings.data.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setSelectedId(item.id)}
                    className={`w-full rounded-lg border p-3 text-left text-sm ${selected?.id === item.id ? "border-primary bg-primary/5" : "hover:bg-muted/50"}`}
                  >
                    <span className="font-medium">{item.name}</span>
                  </button>
                ))}
              </div>
              {selected && <MappingEditor key={selected.id} mapping={selected} canUpdate={can("mapping:update", resolvedWorkspaceId)} canPublish={can("mapping:publish", resolvedWorkspaceId)} canDelete={can("mapping:delete", resolvedWorkspaceId)} />}
            </div>
          )}
        </CardContent>
      </Card>
      <CreateMappingDialog open={creating} workspaceId={resolvedWorkspaceId} onOpenChange={setCreating} />
    </div>
  );
}

function MappingEditor({
  mapping,
  canUpdate,
  canPublish,
  canDelete,
}: {
  mapping: Mapping;
  canUpdate: boolean;
  canPublish: boolean;
  canDelete: boolean;
}) {
  const mutations = useMappingMutations(mapping.workspace_id);
  const tests = useMappingTests(mapping.id);
  const versions = useMappingVersions(mapping.id);
  const [draft, setDraft] = useState(JSON.stringify(mapping.draft, null, 2));
  const [input, setInput] = useState('{\n  "customer_name": "  Ada Lovelace  ",\n  "total_amount": 25\n}');
  const [output, setOutput] = useState("");
  const [testName, setTestName] = useState("");
  const [testInput, setTestInput] = useState("");
  const [testExpected, setTestExpected] = useState("");
  const draftChanged = useMemo(() => draft !== JSON.stringify(mapping.draft, null, 2), [draft, mapping.draft]);


  const runTest = async () => {
    try {
      const parsedInput = JSON.parse(input);
      const result = await mutations.test.mutateAsync({ id: mapping.id, input: parsedInput });
      setOutput(JSON.stringify(result.output, null, 2));
      toast.success(`${result.stored_tests_passed} stored test${result.stored_tests_passed === 1 ? "" : "s"} passed`);
      if (result.stored_tests_failed.length) toast.error(`${result.stored_tests_failed.length} stored test${result.stored_tests_failed.length === 1 ? "" : "s"} failed`);
    } catch (error) {
      toastError(error);
    }
  };

  const saveDraft = async () => {
    try {
      await mutations.update.mutateAsync({ id: mapping.id, body: { draft: JSON.parse(draft) } });
      toast.success("Draft saved.");
    } catch (error) {
      toastError(error);
    }
  };

  const publish = async () => {
    try {
      await mutations.publish.mutateAsync(mapping.id);
      toast.success("Mapping published.");
    } catch (error) {
      toastError(error);
    }
  };

  const createTest = async (event: FormEvent) => {
    event.preventDefault();
    try {
      await mutations.createTest.mutateAsync({
        id: mapping.id,
        body: { name: testName, input: JSON.parse(testInput), expected: JSON.parse(testExpected) },
      });
      setTestName("");
      setTestInput("");
      setTestExpected("");
      toast.success("Test case added.");
    } catch (error) {
      toastError(error);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold">{mapping.name}</h2>
          <p className="text-xs text-muted-foreground">Draft · {mapping.id}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {canUpdate && <Button size="sm" variant="outline" onClick={saveDraft} disabled={!draftChanged}>{draftChanged ? "Save draft" : "Saved"}</Button>}
          {canPublish && <Button size="sm" onClick={publish}><Rocket aria-hidden /> Publish</Button>}
          {canDelete && <ConfirmDialog
            trigger={<Button size="icon" variant="ghost" aria-label={`Delete ${mapping.name}`}><Trash2 /></Button>}
            title={`Delete “${mapping.name}”?`}
            description="The draft and all stored versions and tests will be removed."
            confirmLabel="Delete mapping"
            onConfirm={async () => {
              try { await mutations.remove.mutateAsync(mapping.id); toast.success("Mapping deleted."); }
              catch (error) { toastError(error); throw error; }
            }}
          />}
        </div>
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="mapping-draft">Mapping definition</Label>
          <Textarea id="mapping-draft" className="min-h-72 font-mono text-xs" value={draft} onChange={(event) => setDraft(event.target.value)} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="mapping-input">Input JSON</Label>
          <Textarea id="mapping-input" className="min-h-72 font-mono text-xs" value={input} onChange={(event) => setInput(event.target.value)} />
        </div>
      </div>
      <div className="flex items-center gap-2">
        <Button variant="secondary" onClick={runTest}><FlaskConical aria-hidden /> Run test</Button>
        {output && <Badge variant="outline" className="font-mono">{output}</Badge>}
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader><CardTitle className="text-sm">Stored tests</CardTitle></CardHeader>
          <CardContent>
            {tests.isPending ? <Skeleton className="h-20 w-full" /> : tests.error ? (
              <QueryError error={tests.error} onRetry={() => tests.refetch()} title="Could not load tests" />
            ) : (
              <div className="space-y-2">
                {tests.data?.map((test) => (
                  <div key={test.id} className="flex items-center justify-between rounded-md border p-2 text-sm">
                    <span>{test.name}</span>
                    {canUpdate && <Button size="icon" variant="ghost" aria-label={`Delete test ${test.name}`} onClick={async () => {
                      try { await mutations.removeTest.mutateAsync({ id: mapping.id, testId: test.id }); toast.success("Test removed."); } catch (error) { toastError(error); }
                    }}><Trash2 /></Button>}
                  </div>
                ))}
                {!tests.data?.length && <p className="text-sm text-muted-foreground">No stored tests. Add one to gate publishing.</p>}
              </div>
            )}
            {canUpdate && (
              <form className="mt-4 space-y-2" onSubmit={createTest}>
                <Input placeholder="Test name" value={testName} onChange={(event) => setTestName(event.target.value)} />
                <Textarea className="min-h-24 font-mono text-xs" placeholder="Input JSON" value={testInput} onChange={(event) => setTestInput(event.target.value)} />
                <Textarea className="min-h-24 font-mono text-xs" placeholder="Expected output JSON" value={testExpected} onChange={(event) => setTestExpected(event.target.value)} />
                <Button type="submit" size="sm">Add test</Button>
              </form>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle className="text-sm">Published versions</CardTitle></CardHeader>
          <CardContent>
            {versions.isPending ? <Skeleton className="h-20 w-full" /> : versions.error ? (
              <QueryError error={versions.error} onRetry={() => versions.refetch()} title="Could not load versions" />
            ) : (
              <div className="space-y-2">
                {versions.data?.map((version) => <div key={version.id} className="flex items-center justify-between rounded-md border p-2 text-sm"><span>Version {version.version}</span><code className="text-xs text-muted-foreground">{version.checksum.slice(0, 12)}</code></div>)}
                {!versions.data?.length && <p className="text-sm text-muted-foreground">Nothing published yet.</p>}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function CreateMappingDialog({ open, workspaceId, onOpenChange }: { open: boolean; workspaceId: string; onOpenChange: (open: boolean) => void }) {
  const mutations = useMappingMutations(workspaceId);
  const [name, setName] = useState("");
  const [draft, setDraft] = useState(emptyDraft);

  return (
    <Dialog open={open} onOpenChange={(next) => { onOpenChange(next); if (!next) { setName(""); setDraft(emptyDraft); } }}>
      <DialogContent>
        <DialogHeader><DialogTitle>New mapping</DialogTitle><DialogDescription>Create a draft mapping definition.</DialogDescription></DialogHeader>
        <form onSubmit={async (event) => {
          event.preventDefault();
          try {
            await mutations.create.mutateAsync({ workspace_id: workspaceId, name, draft: JSON.parse(draft) });
            toast.success("Mapping created.");
            onOpenChange(false);
            setName("");
            setDraft(emptyDraft);
          } catch (error) { toastError(error); }
        }} className="space-y-4">
          <div className="space-y-2"><Label htmlFor="mapping-name">Name</Label><Input id="mapping-name" value={name} onChange={(event) => setName(event.target.value)} /></div>
          <div className="space-y-2"><Label htmlFor="mapping-draft-create">Initial definition</Label><Textarea id="mapping-draft-create" className="min-h-56 font-mono text-xs" value={draft} onChange={(event) => setDraft(event.target.value)} /></div>
          <DialogFooter><Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button><Button type="submit" disabled={!name.trim() || !workspaceId}>Create mapping</Button></DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
