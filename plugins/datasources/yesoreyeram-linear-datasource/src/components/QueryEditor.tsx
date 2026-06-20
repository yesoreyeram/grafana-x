import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import { InlineField, InlineFieldRow, Input, Select, MultiSelect, RadioButtonGroup, TextArea, InlineSwitch } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  DATE_MODE_OPTIONS,
  LabelInfo,
  LinearDataSourceOptions,
  LinearDateMode,
  LinearOrderBy,
  LinearQuery,
  LinearQueryType,
  PRIORITY_OPTIONS,
  ProjectInfo,
  StateInfo,
  TeamInfo,
  UserInfo,
} from '../types';

type Props = QueryEditorProps<DataSource, LinearQuery, LinearDataSourceOptions>;

const LABEL_WIDTH = 18;
const INPUT_WIDTH = 40;
// Custom date inputs sit on the same row as the mode selector, so they use a
// compact label + input so After/Before fit alongside it.
const DATE_LABEL_WIDTH = 8;
const DATE_INPUT_WIDTH = 18;
// Fields can hold many selections; give it more room than the single-value inputs.
const FIELDS_WIDTH = 80;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<LinearQueryType>> = [
  { label: 'Issues', value: 'issues', description: 'List issues' },
  { label: 'Projects', value: 'projects', description: 'List projects' },
  { label: 'Teams', value: 'teams', description: 'List teams' },
  { label: 'Users', value: 'users', description: 'List users' },
  { label: 'Cycles', value: 'cycles', description: 'List cycles' },
  { label: 'Raw GraphQL', value: 'raw', description: 'Run a custom GraphQL query' },
];

const ORDER_BY_OPTIONS: Array<SelectableValue<LinearOrderBy>> = [
  { label: 'Created', value: 'createdAt' },
  { label: 'Updated', value: 'updatedAt' },
];

const PRIORITY_SELECT_OPTIONS: Array<SelectableValue<number>> = PRIORITY_OPTIONS.map((p) => ({
  label: p.label,
  value: p.value,
}));

const DATE_MODE_SELECT_OPTIONS: Array<SelectableValue<LinearDateMode>> = DATE_MODE_OPTIONS.map((m) => ({
  label: m.label,
  value: m.value,
  description: m.description,
}));

const RAW_PLACEHOLDER = `query {
  issues(first: 50) {
    nodes { identifier title createdAt state { name } }
  }
}`;

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
    teamId,
    states,
    assignees,
    labels,
    priorities,
    projects,
    creator,
    searchQuery,
    createdAfter,
    createdBefore,
    updatedAfter,
    updatedBefore,
    includeArchived,
    fields,
    rawQuery,
    rawVariables,
    limit,
  } = query;
  const queryType = query.queryType ?? 'issues';
  const orderBy = query.orderBy ?? 'createdAt';
  const createdMode: LinearDateMode = query.createdMode ?? 'any';
  const updatedMode: LinearDateMode = query.updatedMode ?? 'any';

  const isRaw = queryType === 'raw';
  const isIssues = queryType === 'issues';
  const isCycles = queryType === 'cycles';
  const needsTeams = isIssues || isCycles;

  // Each list loads independently. A failure in one does not affect the others.
  const teams = useResource<TeamInfo>(
    () => datasource.getTeams(),
    (items) => items.map((t) => ({ label: `${t.key} · ${t.name}`, value: t.id, description: t.id })),
    needsTeams,
    [datasource, needsTeams]
  );

  const stateList = useResource<StateInfo>(
    () => datasource.getStates(teamId),
    (items) => items.map((s) => ({ label: s.name, value: s.name, description: s.type })),
    isIssues,
    [datasource, isIssues, teamId]
  );

  const labelList = useResource<LabelInfo>(
    () => datasource.getLabels(teamId),
    (items) => items.map((l) => ({ label: l.name, value: l.name })),
    isIssues,
    [datasource, isIssues, teamId]
  );

  const userList = useResource<UserInfo>(
    () => datasource.getUsers(),
    (items) => items.map((u) => ({ label: u.email ? `${u.name} (${u.email})` : u.name, value: u.email || u.name })),
    isIssues,
    [datasource, isIssues]
  );

  const projectList = useResource<ProjectInfo>(
    () => datasource.getProjects(),
    (items) => items.map((p) => ({ label: p.name, value: p.name, description: p.id })),
    isIssues,
    [datasource, isIssues]
  );

  const fieldList = useResource<string>(
    () => datasource.getIssueFields(),
    (items) => items.map((f) => ({ label: f, value: f })),
    isIssues,
    [datasource, isIssues]
  );

  const update = useCallback(
    (patch: Partial<LinearQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const updateAndRun = useCallback(
    (patch: Partial<LinearQuery>) => {
      update(patch);
      onRunQuery();
    },
    [update, onRunQuery]
  );

  const onQueryTypeChange = (value: LinearQueryType) => updateAndRun({ queryType: value });

  const onTeamChange = (value: SelectableValue<string> | null) => {
    // Changing team invalidates team-scoped selections (states & labels).
    updateAndRun({ teamId: value?.value ?? '', states: [], labels: [] });
  };

  const onCreatorChange = (e: ChangeEvent<HTMLInputElement>) => update({ creator: e.target.value });
  const onSearchChange = (e: ChangeEvent<HTMLInputElement>) => update({ searchQuery: e.target.value });
  const onCreatedAfter = (e: ChangeEvent<HTMLInputElement>) => update({ createdAfter: e.target.value });
  const onCreatedBefore = (e: ChangeEvent<HTMLInputElement>) => update({ createdBefore: e.target.value });
  const onUpdatedAfter = (e: ChangeEvent<HTMLInputElement>) => update({ updatedAfter: e.target.value });
  const onUpdatedBefore = (e: ChangeEvent<HTMLInputElement>) => update({ updatedBefore: e.target.value });

  // Switching away from 'custom' clears the manual bounds so they don't linger.
  const onCreatedModeChange = (mode: LinearDateMode) =>
    updateAndRun(mode === 'custom' ? { createdMode: mode } : { createdMode: mode, createdAfter: '', createdBefore: '' });
  const onUpdatedModeChange = (mode: LinearDateMode) =>
    updateAndRun(mode === 'custom' ? { updatedMode: mode } : { updatedMode: mode, updatedAfter: '', updatedBefore: '' });

  const onOrderByChange = (value: LinearOrderBy) => updateAndRun({ orderBy: value });

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  const onRawQueryChange = (e: ChangeEvent<HTMLTextAreaElement>) => update({ rawQuery: e.target.value });
  const onRawVariablesChange = (e: ChangeEvent<HTMLTextAreaElement>) => update({ rawVariables: e.target.value });

  const selectedTeam: SelectableValue<string> | null = teamId
    ? teams.options.find((t) => t.value === teamId) ?? { label: teamId, value: teamId }
    : null;

  const selectedPriorities: Array<SelectableValue<number>> = (priorities ?? []).map(
    (p) => PRIORITY_SELECT_OPTIONS.find((o) => o.value === p) ?? { label: String(p), value: p }
  );

  return (
    <div>
      <div className="gf-form">
        <InlineField label="Query type" labelWidth={LABEL_WIDTH} tooltip="What to fetch from Linear.">
          <RadioButtonGroup<LinearQueryType> options={QUERY_TYPE_OPTIONS} value={queryType} onChange={onQueryTypeChange} />
        </InlineField>
      </div>

      {needsTeams && (
        <InlineFieldRow>
          <InlineField
            label="Team"
            labelWidth={LABEL_WIDTH}
            tooltip="Restrict results to a team. Leave empty for all teams."
            error={teams.error}
            invalid={!!teams.error}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={teams.loading}
              options={teams.options}
              value={selectedTeam}
              onChange={onTeamChange}
              allowCustomValue
              placeholder="All teams"
              noOptionsMessage="No teams found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {isIssues && (
        <>
          <InlineFieldRow>
            <InlineField
              label="States"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter issues by one or more workflow states (matches any)."
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
              tooltip="Filter by one or more assignees (matches any). Accepts email or name."
              error={userList.error}
              invalid={!!userList.error}
            >
              <MultiSelect<string>
                width={INPUT_WIDTH}
                isLoading={userList.loading}
                options={userList.options}
                value={toMulti(assignees, userList.options)}
                onChange={(v) => updateAndRun({ assignees: multiValues(v) })}
                allowCustomValue
                placeholder="Any assignee"
                noOptionsMessage="No users found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Labels"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter to issues that have any of these labels."
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
            <InlineField label="Priorities" labelWidth={LABEL_WIDTH} tooltip="Filter issues by one or more priorities.">
              <MultiSelect<number>
                width={INPUT_WIDTH}
                options={PRIORITY_SELECT_OPTIONS}
                value={selectedPriorities}
                onChange={(v) => updateAndRun({ priorities: v.map((o) => o.value).filter((n): n is number => n != null) })}
                placeholder="Any priority"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Projects"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter issues by one or more projects (matches any)."
              error={projectList.error}
              invalid={!!projectList.error}
            >
              <MultiSelect<string>
                width={INPUT_WIDTH}
                isLoading={projectList.loading}
                options={projectList.options}
                value={toMulti(projects, projectList.options)}
                onChange={(v) => updateAndRun({ projects: multiValues(v) })}
                allowCustomValue
                placeholder="Any project"
                noOptionsMessage="No projects found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField label="Creator" labelWidth={LABEL_WIDTH} tooltip="Filter by issue creator. Accepts email or name.">
              <Input width={INPUT_WIDTH} value={creator ?? ''} placeholder="email or name" onChange={onCreatorChange} onBlur={onRunQuery} />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField label="Title contains" labelWidth={LABEL_WIDTH} tooltip="Filter by text contained in the issue title.">
              <Input width={INPUT_WIDTH} value={searchQuery ?? ''} placeholder="search title" onChange={onSearchChange} onBlur={onRunQuery} />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Created"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by issue creation time. 'Dashboard range' follows the panel's time picker; 'Custom' lets you enter explicit bounds."
            >
              <RadioButtonGroup<LinearDateMode>
                options={DATE_MODE_SELECT_OPTIONS}
                value={createdMode}
                onChange={onCreatedModeChange}
              />
            </InlineField>
            {createdMode === 'custom' && (
              <>
                <InlineField label="After" labelWidth={DATE_LABEL_WIDTH} tooltip="Lower bound (inclusive). ISO-8601, e.g. 2024-01-01 or 2024-01-01T00:00:00Z.">
                  <Input width={DATE_INPUT_WIDTH} value={createdAfter ?? ''} placeholder="2024-01-01" onChange={onCreatedAfter} onBlur={onRunQuery} />
                </InlineField>
                <InlineField label="Before" labelWidth={DATE_LABEL_WIDTH} tooltip="Upper bound (inclusive). ISO-8601.">
                  <Input width={DATE_INPUT_WIDTH} value={createdBefore ?? ''} placeholder="2024-12-31" onChange={onCreatedBefore} onBlur={onRunQuery} />
                </InlineField>
              </>
            )}
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Updated"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by issue last-updated time. 'Dashboard range' follows the panel's time picker; 'Custom' lets you enter explicit bounds."
            >
              <RadioButtonGroup<LinearDateMode>
                options={DATE_MODE_SELECT_OPTIONS}
                value={updatedMode}
                onChange={onUpdatedModeChange}
              />
            </InlineField>
            {updatedMode === 'custom' && (
              <>
                <InlineField label="After" labelWidth={DATE_LABEL_WIDTH} tooltip="Lower bound (inclusive). ISO-8601, e.g. 2024-01-01 or 2024-01-01T00:00:00Z.">
                  <Input width={DATE_INPUT_WIDTH} value={updatedAfter ?? ''} placeholder="2024-01-01" onChange={onUpdatedAfter} onBlur={onRunQuery} />
                </InlineField>
                <InlineField label="Before" labelWidth={DATE_LABEL_WIDTH} tooltip="Upper bound (inclusive). ISO-8601.">
                  <Input width={DATE_INPUT_WIDTH} value={updatedBefore ?? ''} placeholder="2024-12-31" onChange={onUpdatedBefore} onBlur={onRunQuery} />
                </InlineField>
              </>
            )}
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField label="Include archived" labelWidth={LABEL_WIDTH} tooltip="Include archived issues in the results.">
              <InlineSwitch
                value={!!includeArchived}
                onChange={(e) => updateAndRun({ includeArchived: e.currentTarget.checked })}
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Fields"
              labelWidth={LABEL_WIDTH}
              tooltip="Columns to return. Leave empty for the default field set."
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
                placeholder="Default fields"
                noOptionsMessage="No fields found"
              />
            </InlineField>
          </InlineFieldRow>
        </>
      )}

      {isRaw && (
        <>
          <div className="gf-form" style={{ alignItems: 'flex-start' }}>
            <InlineField
              label="GraphQL"
              labelWidth={LABEL_WIDTH}
              tooltip="A Linear GraphQL query. The first connection (object with a `nodes` array) in the response is flattened into a table."
              grow
            >
              <TextArea rows={10} value={rawQuery ?? ''} placeholder={RAW_PLACEHOLDER} onChange={onRawQueryChange} onBlur={onRunQuery} />
            </InlineField>
          </div>
          <div className="gf-form" style={{ alignItems: 'flex-start' }}>
            <InlineField
              label="Variables"
              labelWidth={LABEL_WIDTH}
              tooltip="Optional JSON object of GraphQL variables for the query above."
              grow
            >
              <TextArea rows={3} value={rawVariables ?? ''} placeholder={'{ "first": 50 }'} onChange={onRawVariablesChange} onBlur={onRunQuery} />
            </InlineField>
          </div>
        </>
      )}

      {!isRaw && (
        <InlineFieldRow>
          <InlineField label="Order by" labelWidth={LABEL_WIDTH} tooltip="Order results by created or updated time.">
            <RadioButtonGroup<LinearOrderBy> options={ORDER_BY_OPTIONS} value={orderBy} onChange={onOrderByChange} />
          </InlineField>
          <InlineField label="Limit" labelWidth={LABEL_WIDTH} tooltip="Maximum number of records. 0 returns all (auto-paginated).">
            <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
          </InlineField>
        </InlineFieldRow>
      )}
    </div>
  );
}
