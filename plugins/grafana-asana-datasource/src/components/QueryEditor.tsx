import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import { InlineField, InlineFieldRow, Input, Select, MultiSelect, RadioButtonGroup, InlineSwitch } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  AsanaDataSourceOptions,
  AsanaDateMode,
  AsanaQuery,
  AsanaQueryType,
  AsanaResource,
  DATE_MODE_OPTIONS,
} from '../types';

type Props = QueryEditorProps<DataSource, AsanaQuery, AsanaDataSourceOptions>;

const LABEL_WIDTH = 18;
const INPUT_WIDTH = 40;
const DATE_INPUT_WIDTH = 24;
const FIELDS_WIDTH = 80;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<AsanaQueryType>> = [
  { label: 'Tasks', value: 'tasks', description: 'List tasks in a project, section, or assigned to a user' },
  { label: 'Projects', value: 'projects', description: 'List the projects in a workspace or team' },
  { label: 'Sections', value: 'sections', description: 'List the sections in a project' },
  { label: 'Workspaces', value: 'workspaces', description: 'List the visible workspaces and organizations' },
  { label: 'Teams', value: 'teams', description: 'List the teams in a workspace (organizations only)' },
  { label: 'Users', value: 'users', description: 'List the users in a workspace' },
  { label: 'Tags', value: 'tags', description: 'List the tags in a workspace' },
  { label: 'Raw', value: 'raw', description: 'Run a custom REST GET request' },
];

const DATE_MODE_SELECT_OPTIONS: Array<SelectableValue<AsanaDateMode>> = DATE_MODE_OPTIONS.map((m) => ({
  label: m.label,
  value: m.value,
  description: m.description,
}));

const RAW_PLACEHOLDER = '/workspaces/123/tasks/search?...';

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

const resourceOptions = (items: AsanaResource[]): Array<SelectableValue<string>> =>
  items.map((i) => ({ label: i.name, value: i.gid, description: i.gid }));

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { workspace, team, project, section, assignee, incompleteOnly, modifiedSince, includeArchived, fields, rawPath, rawRoot, limit } =
    query;
  const queryType = query.queryType ?? 'tasks';
  const modifiedMode: AsanaDateMode = query.modifiedMode ?? 'any';

  const isRaw = queryType === 'raw';
  const isTasks = queryType === 'tasks';
  const isProjects = queryType === 'projects';
  const isSections = queryType === 'sections';
  const isWorkspaces = queryType === 'workspaces';

  // Workspace is needed for everything except the workspaces list and raw.
  const needsWorkspace = !isWorkspaces && !isRaw;
  const needsTeam = isTasks || isProjects || isSections;
  const needsProject = isTasks || isSections;
  const needsSection = isTasks;

  const workspaces = useResource<AsanaResource>(
    () => datasource.getWorkspaces(),
    resourceOptions,
    needsWorkspace,
    [datasource, needsWorkspace]
  );

  const teams = useResource<AsanaResource>(
    () => datasource.getTeams(workspace),
    resourceOptions,
    needsTeam && !!workspace,
    [datasource, needsTeam, workspace]
  );

  const projects = useResource<AsanaResource>(
    () => datasource.getProjects(workspace, team),
    resourceOptions,
    needsProject && (!!workspace || !!team),
    [datasource, needsProject, workspace, team]
  );

  const sections = useResource<AsanaResource>(
    () => datasource.getSections(project),
    resourceOptions,
    needsSection && !!project,
    [datasource, needsSection, project]
  );

  const users = useResource<AsanaResource>(
    () => datasource.getUsers(workspace),
    resourceOptions,
    isTasks && !!workspace,
    [datasource, isTasks, workspace]
  );

  const fieldList = useResource<string>(
    () => datasource.getTaskFields(),
    (items) => items.map((f) => ({ label: f, value: f })),
    isTasks,
    [datasource, isTasks]
  );

  const update = useCallback(
    (patch: Partial<AsanaQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const updateAndRun = useCallback(
    (patch: Partial<AsanaQuery>) => {
      update(patch);
      onRunQuery();
    },
    [update, onRunQuery]
  );

  const onQueryTypeChange = (value: AsanaQueryType) => updateAndRun({ queryType: value });

  // Changing a scope level invalidates the more-specific levels below it.
  const onWorkspaceChange = (value: SelectableValue<string> | null) =>
    updateAndRun({ workspace: value?.value ?? '', team: '', project: '', section: '', assignee: '' });
  const onTeamChange = (value: SelectableValue<string> | null) =>
    updateAndRun({ team: value?.value ?? '', project: '', section: '' });
  const onProjectChange = (value: SelectableValue<string> | null) =>
    updateAndRun({ project: value?.value ?? '', section: '' });
  const onSectionChange = (value: SelectableValue<string> | null) => updateAndRun({ section: value?.value ?? '' });
  const onAssigneeChange = (value: SelectableValue<string> | null) => updateAndRun({ assignee: value?.value ?? '' });

  const onModifiedSince = (e: ChangeEvent<HTMLInputElement>) => update({ modifiedSince: e.target.value });
  const onModifiedModeChange = (mode: AsanaDateMode) =>
    updateAndRun({ modifiedMode: mode, ...(mode === 'custom' ? {} : { modifiedSince: '' }) });

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
        <InlineField label="Query type" labelWidth={LABEL_WIDTH} tooltip="What to fetch from Asana.">
          <RadioButtonGroup<AsanaQueryType> options={QUERY_TYPE_OPTIONS} value={queryType} onChange={onQueryTypeChange} />
        </InlineField>
      </div>

      {needsWorkspace && (
        <InlineFieldRow>
          <InlineField
            label="Workspace"
            labelWidth={LABEL_WIDTH}
            tooltip="The Asana workspace or organization to query."
            error={workspaces.error}
            invalid={!!workspaces.error}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={workspaces.loading}
              options={workspaces.options}
              value={selected(workspace, workspaces.options)}
              onChange={onWorkspaceChange}
              allowCustomValue
              placeholder="Select a workspace"
              noOptionsMessage="No workspaces found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {needsTeam && (
        <InlineFieldRow>
          <InlineField
            label="Team"
            labelWidth={LABEL_WIDTH}
            tooltip="Restrict the project list to a team (organizations only). Optional."
            error={teams.error}
            invalid={!!teams.error}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={teams.loading}
              options={teams.options}
              value={selected(team, teams.options)}
              onChange={onTeamChange}
              allowCustomValue
              placeholder="All teams"
              noOptionsMessage="No teams found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {needsProject && (
        <InlineFieldRow>
          <InlineField
            label="Project"
            labelWidth={LABEL_WIDTH}
            tooltip="The project to query. Required for Sections; for Tasks it scopes the task list."
            error={projects.error}
            invalid={!!projects.error}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={projects.loading}
              options={projects.options}
              value={selected(project, projects.options)}
              onChange={onProjectChange}
              allowCustomValue
              placeholder={isTasks ? 'Select a project' : 'Select a project'}
              noOptionsMessage="No projects found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {needsSection && (
        <InlineFieldRow>
          <InlineField
            label="Section"
            labelWidth={LABEL_WIDTH}
            tooltip="Restrict tasks to a single section of the project. Optional."
            error={sections.error}
            invalid={!!sections.error}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={sections.loading}
              options={sections.options}
              value={selected(section, sections.options)}
              onChange={onSectionChange}
              allowCustomValue
              placeholder="All sections"
              noOptionsMessage="No sections found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {isTasks && (
        <>
          <InlineFieldRow>
            <InlineField
              label="Assignee"
              labelWidth={LABEL_WIDTH}
              tooltip="Restrict tasks to an assignee (uses the Workspace). Applies only when no Project or Section is selected. Use the literal value 'me' for the current user."
              error={users.error}
              invalid={!!users.error}
            >
              <Select<string>
                width={INPUT_WIDTH}
                isClearable
                isLoading={users.loading}
                options={users.options}
                value={selected(assignee, users.options)}
                onChange={onAssigneeChange}
                allowCustomValue
                placeholder="Any assignee"
                noOptionsMessage="No users found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField label="Incomplete only" labelWidth={LABEL_WIDTH} tooltip="Return only incomplete tasks.">
              <InlineSwitch
                value={!!incompleteOnly}
                onChange={(e) => updateAndRun({ incompleteOnly: e.currentTarget.checked })}
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Modified"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by task last-modified time. 'Dashboard range' follows the panel's time picker (from); 'Custom' lets you enter an explicit ISO-8601 bound."
            >
              <RadioButtonGroup<AsanaDateMode>
                options={DATE_MODE_SELECT_OPTIONS}
                value={modifiedMode}
                onChange={onModifiedModeChange}
              />
            </InlineField>
            {modifiedMode === 'custom' && (
              <InlineField label="Since" labelWidth={10} tooltip="Lower bound. ISO-8601, e.g. 2024-01-01 or 2024-01-01T00:00:00Z.">
                <Input
                  width={DATE_INPUT_WIDTH}
                  value={modifiedSince ?? ''}
                  placeholder="2024-01-01"
                  onChange={onModifiedSince}
                  onBlur={onRunQuery}
                />
              </InlineField>
            )}
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

      {isProjects && (
        <InlineFieldRow>
          <InlineField label="Archived" labelWidth={LABEL_WIDTH} tooltip="Include archived projects in the results.">
            <InlineSwitch
              value={!!includeArchived}
              onChange={(e) => updateAndRun({ includeArchived: e.currentTarget.checked })}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {isRaw && (
        <>
          <InlineFieldRow>
            <InlineField
              label="REST path"
              labelWidth={LABEL_WIDTH}
              tooltip="An Asana REST GET path relative to the API root, e.g. /workspaces. The first array of objects in the response (Asana wraps results under 'data') is flattened into a table."
              grow
            >
              <Input width={FIELDS_WIDTH} value={rawPath ?? ''} placeholder={RAW_PLACEHOLDER} onChange={onRawPathChange} onBlur={onRunQuery} />
            </InlineField>
          </InlineFieldRow>
          <InlineFieldRow>
            <InlineField
              label="Response key"
              labelWidth={LABEL_WIDTH}
              tooltip="Optional. The JSON key in the response holding the array (or object) to flatten into rows, e.g. 'data'. Leave empty to auto-detect."
            >
              <Input width={INPUT_WIDTH} value={rawRoot ?? ''} placeholder="auto-detect" onChange={onRawRootChange} onBlur={onRunQuery} />
            </InlineField>
          </InlineFieldRow>
        </>
      )}

      {!isRaw && (
        <InlineFieldRow>
          <InlineField label="Limit" labelWidth={LABEL_WIDTH} tooltip="Maximum number of records. 0 returns all (auto-paginated).">
            <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
          </InlineField>
        </InlineFieldRow>
      )}
    </div>
  );
}
