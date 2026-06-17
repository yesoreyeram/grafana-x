import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import { InlineField, InlineFieldRow, Input, Select, MultiSelect, RadioButtonGroup } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  PlaneDataSourceOptions,
  PlaneDateMode,
  PlaneQuery,
  PlaneQueryType,
  DATE_MODE_OPTIONS,
  EXPAND_OPTIONS,
  LabelInfo,
  MemberInfo,
  ProjectInfo,
  StateInfo,
} from '../types';

type Props = QueryEditorProps<DataSource, PlaneQuery, PlaneDataSourceOptions>;

const LABEL_WIDTH = 18;
const INPUT_WIDTH = 40;
const DATE_LABEL_WIDTH = 8;
const DATE_INPUT_WIDTH = 22;
const FIELDS_WIDTH = 80;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<PlaneQueryType>> = [
  { label: 'Work items', value: 'workitems', description: 'List work items (issues) in a project' },
  { label: 'Projects', value: 'projects', description: 'List the projects in a workspace' },
  { label: 'States', value: 'states', description: 'List the states in a project' },
  { label: 'Labels', value: 'labels', description: 'List the labels in a project' },
  { label: 'Cycles', value: 'cycles', description: 'List the cycles in a project' },
  { label: 'Modules', value: 'modules', description: 'List the modules in a project' },
  { label: 'Members', value: 'members', description: 'List the members of a workspace' },
  { label: 'Raw', value: 'raw', description: 'Run a custom REST GET request' },
];

const DATE_MODE_SELECT_OPTIONS: Array<SelectableValue<PlaneDateMode>> = DATE_MODE_OPTIONS.map((m) => ({
  label: m.label,
  value: m.value,
  description: m.description,
}));

const RAW_PLACEHOLDER = '/api/v1/workspaces/my-team/projects/';

/** Map a stored string[] value to selected options, preserving custom values. */
function toMulti(values: string[] | undefined, options: Array<SelectableValue<string>>): Array<SelectableValue<string>> {
  return (values ?? []).map((v) => options.find((o) => o.value === v) ?? { label: v, value: v });
}

/** Extract plain string values from selected options. */
function multiValues(values: Array<SelectableValue<string>>): string[] {
  return values.map((v) => v.value).filter((v): v is string => v != null && v !== '');
}

/** Read a resource-call error into a short message. */
function errMessage(err: unknown, fallback: string): string {
  const e = err as { data?: { error?: string }; message?: string };
  return e?.data?.error ?? e?.message ?? fallback;
}

/**
 * useResource loads a list via the provided loader whenever `enabled` is true and
 * any dependency changes. Each list is loaded independently so one failure never
 * blanks the others. Returns the options, a loading flag, and an error message.
 */
function useResource<T>(
  loader: () => Promise<T[]>,
  toOptions: (items: T[]) => Array<SelectableValue<string>>,
  enabled: boolean,
  deps: React.DependencyList
): { options: Array<SelectableValue<string>>; loading: boolean; error?: string } {
  const [options, setOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | undefined>();

  useEffect(() => {
    if (!enabled) {
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(undefined);
    loader()
      .then((items) => {
        if (!cancelled) {
          setOptions(toOptions(items));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setOptions([]);
          setError(errMessage(err, 'Failed to load'));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return { options, loading, error };
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const {
    workspaceSlug,
    projectId,
    priorities,
    states,
    assignees,
    labels,
    expand,
    createdAfter,
    createdBefore,
    updatedAfter,
    updatedBefore,
    fields,
    rawPath,
    rawRoot,
    limit,
  } = query;
  const queryType = query.queryType ?? 'workitems';
  const orderBy = query.orderBy ?? '-created_at';
  const createdMode: PlaneDateMode = query.createdMode ?? 'any';
  const updatedMode: PlaneDateMode = query.updatedMode ?? 'any';

  const isRaw = queryType === 'raw';
  const isWorkItems = queryType === 'workitems';
  const isProjects = queryType === 'projects';
  const isMembers = queryType === 'members';
  // Workspace applies to every predefined query type.
  const needsWorkspace = !isRaw;
  // A project is required for everything project-scoped (not projects/members).
  const needsProject = !isRaw && !isProjects && !isMembers;

  const projects = useResource<ProjectInfo>(
    () => datasource.getProjects(workspaceSlug),
    (items) =>
      items.map((p) => ({ label: p.identifier ? `${p.name} (${p.identifier})` : p.name, value: p.id, description: p.id })),
    needsProject,
    [datasource, needsProject, workspaceSlug]
  );

  const stateList = useResource<StateInfo>(
    () => datasource.getStates(workspaceSlug, projectId),
    (items) => items.map((s) => ({ label: s.group ? `${s.name} (${s.group})` : s.name, value: s.id })),
    isWorkItems && !!projectId,
    [datasource, isWorkItems, workspaceSlug, projectId]
  );

  const labelList = useResource<LabelInfo>(
    () => datasource.getLabels(workspaceSlug, projectId),
    (items) => items.map((l) => ({ label: l.name, value: l.id })),
    isWorkItems && !!projectId,
    [datasource, isWorkItems, workspaceSlug, projectId]
  );

  const memberList = useResource<MemberInfo>(
    () => datasource.getMembers(workspaceSlug),
    (items) =>
      items.map((m) => ({
        label: m.display_name ? `${m.display_name}${m.email ? ` (${m.email})` : ''}` : m.id,
        value: m.id,
      })),
    isWorkItems,
    [datasource, isWorkItems, workspaceSlug]
  );

  const priorityList = useResource<string>(
    () => datasource.getPriorities(),
    (items) => items.map((p) => ({ label: p, value: p })),
    isWorkItems,
    [datasource, isWorkItems]
  );

  const fieldList = useResource<string>(
    () => datasource.getWorkItemFields(),
    (items) => items.map((f) => ({ label: f, value: f })),
    isWorkItems,
    [datasource, isWorkItems]
  );

  const update = useCallback(
    (patch: Partial<PlaneQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const updateAndRun = useCallback(
    (patch: Partial<PlaneQuery>) => {
      update(patch);
      onRunQuery();
    },
    [update, onRunQuery]
  );

  const onQueryTypeChange = (value: PlaneQueryType) => updateAndRun({ queryType: value });

  const onWorkspaceChange = (e: ChangeEvent<HTMLInputElement>) => update({ workspaceSlug: e.target.value });
  // Changing the project invalidates the project-scoped filters below it.
  const onProjectChange = (value: SelectableValue<string> | null) =>
    updateAndRun({ projectId: value?.value ?? '', states: [], labels: [] });

  const onCreatedAfter = (e: ChangeEvent<HTMLInputElement>) => update({ createdAfter: e.target.value });
  const onCreatedBefore = (e: ChangeEvent<HTMLInputElement>) => update({ createdBefore: e.target.value });
  const onUpdatedAfter = (e: ChangeEvent<HTMLInputElement>) => update({ updatedAfter: e.target.value });
  const onUpdatedBefore = (e: ChangeEvent<HTMLInputElement>) => update({ updatedBefore: e.target.value });

  const clearCustom = (mode: PlaneDateMode, afterKey: keyof PlaneQuery, beforeKey: keyof PlaneQuery) =>
    mode === 'custom' ? {} : { [afterKey]: '', [beforeKey]: '' };

  const onCreatedModeChange = (mode: PlaneDateMode) =>
    updateAndRun({ createdMode: mode, ...clearCustom(mode, 'createdAfter', 'createdBefore') });
  const onUpdatedModeChange = (mode: PlaneDateMode) =>
    updateAndRun({ updatedMode: mode, ...clearCustom(mode, 'updatedAfter', 'updatedBefore') });

  const onOrderByChange = (e: ChangeEvent<HTMLInputElement>) => update({ orderBy: e.target.value });

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  const onRawPathChange = (e: ChangeEvent<HTMLInputElement>) => update({ rawPath: e.target.value });
  const onRawRootChange = (e: ChangeEvent<HTMLInputElement>) => update({ rawRoot: e.target.value });

  const selected = (id: string | undefined, opts: Array<SelectableValue<string>>): SelectableValue<string> | null =>
    id ? opts.find((o) => o.value === id) ?? { label: id, value: id } : null;

  return (
    <div>
      <div className="gf-form">
        <InlineField label="Query type" labelWidth={LABEL_WIDTH} tooltip="What to fetch from Plane.">
          <RadioButtonGroup<PlaneQueryType> options={QUERY_TYPE_OPTIONS} value={queryType} onChange={onQueryTypeChange} />
        </InlineField>
      </div>

      {needsWorkspace && (
        <InlineFieldRow>
          <InlineField
            label="Workspace"
            labelWidth={LABEL_WIDTH}
            tooltip="The Plane workspace slug. Leave empty to use the data source default."
          >
            <Input
              width={INPUT_WIDTH}
              value={workspaceSlug ?? ''}
              placeholder="Default workspace"
              onChange={onWorkspaceChange}
              onBlur={onRunQuery}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {needsProject && (
        <InlineFieldRow>
          <InlineField
            label="Project"
            labelWidth={LABEL_WIDTH}
            tooltip="The Plane project to query."
            error={projects.error}
            invalid={!!projects.error}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={projects.loading}
              options={projects.options}
              value={selected(projectId, projects.options)}
              onChange={onProjectChange}
              allowCustomValue
              placeholder="Select a project"
              noOptionsMessage="No projects found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {isWorkItems && (
        <>
          <InlineFieldRow>
            <InlineField
              label="Priorities"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter work items by one or more priorities (matches any)."
              error={priorityList.error}
              invalid={!!priorityList.error}
            >
              <MultiSelect<string>
                width={INPUT_WIDTH}
                isLoading={priorityList.loading}
                options={priorityList.options}
                value={toMulti(priorities, priorityList.options)}
                onChange={(v) => updateAndRun({ priorities: multiValues(v) })}
                allowCustomValue
                placeholder="Any priority"
                noOptionsMessage="No priorities found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="States"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter work items by one or more states (matches any)."
              error={stateList.error}
              invalid={!!stateList.error}
            >
              <MultiSelect<string>
                width={INPUT_WIDTH}
                isLoading={stateList.loading}
                options={stateList.options}
                value={toMulti(states, stateList.options)}
                onChange={(v) => updateAndRun({ states: multiValues(v) })}
                allowCustomValue
                placeholder="Any state"
                noOptionsMessage="No states found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Assignees"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by one or more assignees (matches any). Uses Plane member user IDs."
              error={memberList.error}
              invalid={!!memberList.error}
            >
              <MultiSelect<string>
                width={INPUT_WIDTH}
                isLoading={memberList.loading}
                options={memberList.options}
                value={toMulti(assignees, memberList.options)}
                onChange={(v) => updateAndRun({ assignees: multiValues(v) })}
                allowCustomValue
                placeholder="Any assignee"
                noOptionsMessage="No members found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Labels"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter work items by one or more labels (matches any)."
              error={labelList.error}
              invalid={!!labelList.error}
            >
              <MultiSelect<string>
                width={INPUT_WIDTH}
                isLoading={labelList.loading}
                options={labelList.options}
                value={toMulti(labels, labelList.options)}
                onChange={(v) => updateAndRun({ labels: multiValues(v) })}
                allowCustomValue
                placeholder="Any label"
                noOptionsMessage="No labels found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Created"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by work item creation time. 'Dashboard range' follows the panel's time picker; 'Custom' lets you enter explicit bounds."
            >
              <RadioButtonGroup<PlaneDateMode>
                options={DATE_MODE_SELECT_OPTIONS}
                value={createdMode}
                onChange={onCreatedModeChange}
              />
            </InlineField>
            {createdMode === 'custom' && (
              <>
                <InlineField label="After" labelWidth={DATE_LABEL_WIDTH} tooltip="Lower bound. ISO-8601 (2024-01-01 or 2024-01-01T00:00:00Z).">
                  <Input width={DATE_INPUT_WIDTH} value={createdAfter ?? ''} placeholder="2024-01-01" onChange={onCreatedAfter} onBlur={onRunQuery} />
                </InlineField>
                <InlineField label="Before" labelWidth={DATE_LABEL_WIDTH} tooltip="Upper bound. ISO-8601.">
                  <Input width={DATE_INPUT_WIDTH} value={createdBefore ?? ''} placeholder="2024-12-31" onChange={onCreatedBefore} onBlur={onRunQuery} />
                </InlineField>
              </>
            )}
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Updated"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by work item last-updated time. 'Dashboard range' follows the panel's time picker; 'Custom' lets you enter explicit bounds."
            >
              <RadioButtonGroup<PlaneDateMode>
                options={DATE_MODE_SELECT_OPTIONS}
                value={updatedMode}
                onChange={onUpdatedModeChange}
              />
            </InlineField>
            {updatedMode === 'custom' && (
              <>
                <InlineField label="After" labelWidth={DATE_LABEL_WIDTH} tooltip="Lower bound. ISO-8601.">
                  <Input width={DATE_INPUT_WIDTH} value={updatedAfter ?? ''} placeholder="2024-01-01" onChange={onUpdatedAfter} onBlur={onRunQuery} />
                </InlineField>
                <InlineField label="Before" labelWidth={DATE_LABEL_WIDTH} tooltip="Upper bound. ISO-8601.">
                  <Input width={DATE_INPUT_WIDTH} value={updatedBefore ?? ''} placeholder="2024-12-31" onChange={onUpdatedBefore} onBlur={onRunQuery} />
                </InlineField>
              </>
            )}
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Expand"
              labelWidth={LABEL_WIDTH}
              tooltip="Ask Plane to inline related objects so they show readable names instead of IDs."
            >
              <MultiSelect<string>
                width={INPUT_WIDTH}
                options={EXPAND_OPTIONS}
                value={toMulti(expand, EXPAND_OPTIONS)}
                onChange={(v) => updateAndRun({ expand: multiValues(v) })}
                allowCustomValue
                placeholder="None"
                noOptionsMessage="No expand options"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Fields"
              labelWidth={LABEL_WIDTH}
              tooltip="Columns to return. Leave empty for all fields."
              error={fieldList.error}
              invalid={!!fieldList.error}
              grow
            >
              <MultiSelect<string>
                width={FIELDS_WIDTH}
                isLoading={fieldList.loading}
                options={fieldList.options}
                value={toMulti(fields, fieldList.options)}
                onChange={(v) => updateAndRun({ fields: multiValues(v) })}
                allowCustomValue
                placeholder="All fields"
                noOptionsMessage="No fields found"
              />
            </InlineField>
          </InlineFieldRow>
        </>
      )}

      {isRaw && (
        <>
          <InlineFieldRow>
            <InlineField
              label="REST path"
              labelWidth={LABEL_WIDTH}
              tooltip="A Plane REST GET path relative to the API root, e.g. /api/v1/workspaces/my-team/projects/. The 'results' array (or the first array of objects found) is flattened into a table."
              grow
            >
              <Input width={FIELDS_WIDTH} value={rawPath ?? ''} placeholder={RAW_PLACEHOLDER} onChange={onRawPathChange} onBlur={onRunQuery} />
            </InlineField>
          </InlineFieldRow>
          <InlineFieldRow>
            <InlineField
              label="Response key"
              labelWidth={LABEL_WIDTH}
              tooltip="Optional. The JSON key in the response holding the array (or object) to flatten into rows. Defaults to 'results'."
            >
              <Input width={INPUT_WIDTH} value={rawRoot ?? ''} placeholder="results" onChange={onRawRootChange} onBlur={onRunQuery} />
            </InlineField>
          </InlineFieldRow>
        </>
      )}

      {!isRaw && !isMembers && (
        <InlineFieldRow>
          <InlineField
            label="Order by"
            labelWidth={LABEL_WIDTH}
            tooltip="Field to order by. Prefix with '-' for descending, e.g. -created_at, priority, sequence_id."
          >
            <Input width={INPUT_WIDTH} value={orderBy} placeholder="-created_at" onChange={onOrderByChange} onBlur={onRunQuery} />
          </InlineField>
          <InlineField label="Limit" labelWidth={10} tooltip="Maximum number of records. 0 returns all (auto-paginated).">
            <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
          </InlineField>
        </InlineFieldRow>
      )}
    </div>
  );
}
