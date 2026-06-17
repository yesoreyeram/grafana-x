import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import { InlineField, InlineFieldRow, Input, Select, MultiSelect, RadioButtonGroup, InlineSwitch } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  ClickUpDataSourceOptions,
  ClickUpDateMode,
  ClickUpListInfo,
  ClickUpOrderBy,
  ClickUpQuery,
  ClickUpQueryType,
  DATE_MODE_OPTIONS,
  FolderInfo,
  MemberInfo,
  SpaceInfo,
  TeamInfo,
} from '../types';

type Props = QueryEditorProps<DataSource, ClickUpQuery, ClickUpDataSourceOptions>;

const LABEL_WIDTH = 18;
const INPUT_WIDTH = 40;
const DATE_LABEL_WIDTH = 8;
const DATE_INPUT_WIDTH = 18;
const FIELDS_WIDTH = 80;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<ClickUpQueryType>> = [
  { label: 'Tasks', value: 'tasks', description: 'List tasks' },
  { label: 'Lists', value: 'lists', description: 'List the Lists in a Folder or Space' },
  { label: 'Folders', value: 'folders', description: 'List the Folders in a Space' },
  { label: 'Spaces', value: 'spaces', description: 'List the Spaces in a Workspace' },
  { label: 'Workspaces', value: 'teams', description: 'List the authorized Workspaces' },
  { label: 'Raw', value: 'raw', description: 'Run a custom REST GET request' },
];

const ORDER_BY_OPTIONS: Array<SelectableValue<ClickUpOrderBy>> = [
  { label: 'Created', value: 'created' },
  { label: 'Updated', value: 'updated' },
  { label: 'Due date', value: 'due_date' },
  { label: 'ID', value: 'id' },
];

const DATE_MODE_SELECT_OPTIONS: Array<SelectableValue<ClickUpDateMode>> = DATE_MODE_OPTIONS.map((m) => ({
  label: m.label,
  value: m.value,
  description: m.description,
}));

const RAW_PLACEHOLDER = '/v2/team/123456/task?subtasks=true';

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
    spaceId,
    folderId,
    listId,
    statuses,
    assignees,
    tags,
    includeClosed,
    includeSubtasks,
    includeArchived,
    createdAfter,
    createdBefore,
    updatedAfter,
    updatedBefore,
    dueAfter,
    dueBefore,
    fields,
    rawPath,
    rawRoot,
    reverse,
    limit,
  } = query;
  const queryType = query.queryType ?? 'tasks';
  const orderBy = query.orderBy ?? 'created';
  const createdMode: ClickUpDateMode = query.createdMode ?? 'any';
  const updatedMode: ClickUpDateMode = query.updatedMode ?? 'any';
  const dueMode: ClickUpDateMode = query.dueMode ?? 'any';

  const isRaw = queryType === 'raw';
  const isTasks = queryType === 'tasks';
  const isTeams = queryType === 'teams';
  // Workspace is needed for everything except the workspaces list and raw.
  const needsTeam = !isTeams && !isRaw;
  const needsSpace = isTasks || queryType === 'folders' || queryType === 'lists';
  const needsFolder = isTasks || queryType === 'lists';
  const needsList = isTasks;

  const teams = useResource<TeamInfo>(
    () => datasource.getTeams(),
    (items) => items.map((t) => ({ label: t.name, value: t.id, description: t.id })),
    needsTeam,
    [datasource, needsTeam]
  );

  const spaces = useResource<SpaceInfo>(
    () => datasource.getSpaces(teamId),
    (items) => items.map((s) => ({ label: s.name, value: s.id, description: s.id })),
    needsSpace && !!teamId,
    [datasource, needsSpace, teamId]
  );

  const folders = useResource<FolderInfo>(
    () => datasource.getFolders(spaceId),
    (items) => items.map((f) => ({ label: f.name, value: f.id, description: f.id })),
    needsFolder && !!spaceId,
    [datasource, needsFolder, spaceId]
  );

  const lists = useResource<ClickUpListInfo>(
    () => datasource.getLists(spaceId, folderId),
    (items) => items.map((l) => ({ label: l.name, value: l.id, description: l.id })),
    needsList && (!!folderId || !!spaceId),
    [datasource, needsList, spaceId, folderId]
  );

  const memberList = useResource<MemberInfo>(
    () => datasource.getMembers(teamId),
    (items) =>
      items.map((m) => ({
        label: m.username ? `${m.username}${m.email ? ` (${m.email})` : ''}` : m.id,
        value: m.id,
      })),
    isTasks,
    [datasource, isTasks, teamId]
  );

  const fieldList = useResource<string>(
    () => datasource.getTaskFields(),
    (items) => items.map((f) => ({ label: f, value: f })),
    isTasks,
    [datasource, isTasks]
  );

  const update = useCallback(
    (patch: Partial<ClickUpQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const updateAndRun = useCallback(
    (patch: Partial<ClickUpQuery>) => {
      update(patch);
      onRunQuery();
    },
    [update, onRunQuery]
  );

  const onQueryTypeChange = (value: ClickUpQueryType) => updateAndRun({ queryType: value });

  // Changing a scope level invalidates the more-specific levels below it.
  const onTeamChange = (value: SelectableValue<string> | null) =>
    updateAndRun({ teamId: value?.value ?? '', spaceId: '', folderId: '', listId: '' });
  const onSpaceChange = (value: SelectableValue<string> | null) =>
    updateAndRun({ spaceId: value?.value ?? '', folderId: '', listId: '' });
  const onFolderChange = (value: SelectableValue<string> | null) =>
    updateAndRun({ folderId: value?.value ?? '', listId: '' });
  const onListChange = (value: SelectableValue<string> | null) => updateAndRun({ listId: value?.value ?? '' });

  const onCreatedAfter = (e: ChangeEvent<HTMLInputElement>) => update({ createdAfter: e.target.value });
  const onCreatedBefore = (e: ChangeEvent<HTMLInputElement>) => update({ createdBefore: e.target.value });
  const onUpdatedAfter = (e: ChangeEvent<HTMLInputElement>) => update({ updatedAfter: e.target.value });
  const onUpdatedBefore = (e: ChangeEvent<HTMLInputElement>) => update({ updatedBefore: e.target.value });
  const onDueAfter = (e: ChangeEvent<HTMLInputElement>) => update({ dueAfter: e.target.value });
  const onDueBefore = (e: ChangeEvent<HTMLInputElement>) => update({ dueBefore: e.target.value });

  const clearCustom = (mode: ClickUpDateMode, afterKey: keyof ClickUpQuery, beforeKey: keyof ClickUpQuery) =>
    mode === 'custom' ? {} : { [afterKey]: '', [beforeKey]: '' };

  const onCreatedModeChange = (mode: ClickUpDateMode) =>
    updateAndRun({ createdMode: mode, ...clearCustom(mode, 'createdAfter', 'createdBefore') });
  const onUpdatedModeChange = (mode: ClickUpDateMode) =>
    updateAndRun({ updatedMode: mode, ...clearCustom(mode, 'updatedAfter', 'updatedBefore') });
  const onDueModeChange = (mode: ClickUpDateMode) =>
    updateAndRun({ dueMode: mode, ...clearCustom(mode, 'dueAfter', 'dueBefore') });

  const onOrderByChange = (value: ClickUpOrderBy) => updateAndRun({ orderBy: value });

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
        <InlineField label="Query type" labelWidth={LABEL_WIDTH} tooltip="What to fetch from ClickUp.">
          <RadioButtonGroup<ClickUpQueryType> options={QUERY_TYPE_OPTIONS} value={queryType} onChange={onQueryTypeChange} />
        </InlineField>
      </div>

      {needsTeam && (
        <InlineFieldRow>
          <InlineField
            label="Workspace"
            labelWidth={LABEL_WIDTH}
            tooltip="The ClickUp Workspace (team) to query."
            error={teams.error}
            invalid={!!teams.error}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={teams.loading}
              options={teams.options}
              value={selected(teamId, teams.options)}
              onChange={onTeamChange}
              allowCustomValue
              placeholder="Select a workspace"
              noOptionsMessage="No workspaces found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {needsSpace && (
        <InlineFieldRow>
          <InlineField
            label="Space"
            labelWidth={LABEL_WIDTH}
            tooltip="Restrict results to a Space. Required for Folders queries."
            error={spaces.error}
            invalid={!!spaces.error}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={spaces.loading}
              options={spaces.options}
              value={selected(spaceId, spaces.options)}
              onChange={onSpaceChange}
              allowCustomValue
              placeholder={isTasks ? 'All spaces' : 'Select a space'}
              noOptionsMessage="No spaces found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {needsFolder && (
        <InlineFieldRow>
          <InlineField
            label="Folder"
            labelWidth={LABEL_WIDTH}
            tooltip="Restrict results to a Folder. For Lists queries, leave empty to list a Space's folderless Lists."
            error={folders.error}
            invalid={!!folders.error}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={folders.loading}
              options={folders.options}
              value={selected(folderId, folders.options)}
              onChange={onFolderChange}
              allowCustomValue
              placeholder={isTasks ? 'All folders' : 'Folderless lists'}
              noOptionsMessage="No folders found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {needsList && (
        <InlineFieldRow>
          <InlineField
            label="List"
            labelWidth={LABEL_WIDTH}
            tooltip="Restrict tasks to a single List. When set, tasks are read from that List directly."
            error={lists.error}
            invalid={!!lists.error}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={lists.loading}
              options={lists.options}
              value={selected(listId, lists.options)}
              onChange={onListChange}
              allowCustomValue
              placeholder="All lists"
              noOptionsMessage="No lists found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {isTasks && (
        <>
          <InlineFieldRow>
            <InlineField label="Statuses" labelWidth={LABEL_WIDTH} tooltip="Filter tasks by one or more status names (matches any).">
              <MultiSelect<string>
                width={INPUT_WIDTH}
                value={toMulti(statuses, [])}
                onChange={(v) => updateAndRun({ statuses: multiValues(v) })}
                allowCustomValue
                placeholder="Any status (type to add)"
                noOptionsMessage="Type a status name"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Assignees"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by one or more assignees (matches any). Uses ClickUp user IDs."
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
            <InlineField label="Tags" labelWidth={LABEL_WIDTH} tooltip="Filter tasks by one or more tag names (matches any).">
              <MultiSelect<string>
                width={INPUT_WIDTH}
                value={toMulti(tags, [])}
                onChange={(v) => updateAndRun({ tags: multiValues(v) })}
                allowCustomValue
                placeholder="Any tag (type to add)"
                noOptionsMessage="Type a tag name"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Created"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by task creation time. 'Dashboard range' follows the panel's time picker; 'Custom' lets you enter explicit bounds."
            >
              <RadioButtonGroup<ClickUpDateMode> options={DATE_MODE_SELECT_OPTIONS} value={createdMode} onChange={onCreatedModeChange} />
            </InlineField>
            {createdMode === 'custom' && (
              <>
                <InlineField label="After" labelWidth={DATE_LABEL_WIDTH} tooltip="Lower bound. ISO-8601 (2024-01-01) or Unix millis.">
                  <Input width={DATE_INPUT_WIDTH} value={createdAfter ?? ''} placeholder="2024-01-01" onChange={onCreatedAfter} onBlur={onRunQuery} />
                </InlineField>
                <InlineField label="Before" labelWidth={DATE_LABEL_WIDTH} tooltip="Upper bound. ISO-8601 or Unix millis.">
                  <Input width={DATE_INPUT_WIDTH} value={createdBefore ?? ''} placeholder="2024-12-31" onChange={onCreatedBefore} onBlur={onRunQuery} />
                </InlineField>
              </>
            )}
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Updated"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by task last-updated time. 'Dashboard range' follows the panel's time picker; 'Custom' lets you enter explicit bounds."
            >
              <RadioButtonGroup<ClickUpDateMode> options={DATE_MODE_SELECT_OPTIONS} value={updatedMode} onChange={onUpdatedModeChange} />
            </InlineField>
            {updatedMode === 'custom' && (
              <>
                <InlineField label="After" labelWidth={DATE_LABEL_WIDTH} tooltip="Lower bound. ISO-8601 or Unix millis.">
                  <Input width={DATE_INPUT_WIDTH} value={updatedAfter ?? ''} placeholder="2024-01-01" onChange={onUpdatedAfter} onBlur={onRunQuery} />
                </InlineField>
                <InlineField label="Before" labelWidth={DATE_LABEL_WIDTH} tooltip="Upper bound. ISO-8601 or Unix millis.">
                  <Input width={DATE_INPUT_WIDTH} value={updatedBefore ?? ''} placeholder="2024-12-31" onChange={onUpdatedBefore} onBlur={onRunQuery} />
                </InlineField>
              </>
            )}
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Due"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by task due date. 'Dashboard range' follows the panel's time picker; 'Custom' lets you enter explicit bounds."
            >
              <RadioButtonGroup<ClickUpDateMode> options={DATE_MODE_SELECT_OPTIONS} value={dueMode} onChange={onDueModeChange} />
            </InlineField>
            {dueMode === 'custom' && (
              <>
                <InlineField label="After" labelWidth={DATE_LABEL_WIDTH} tooltip="Lower bound. ISO-8601 or Unix millis.">
                  <Input width={DATE_INPUT_WIDTH} value={dueAfter ?? ''} placeholder="2024-01-01" onChange={onDueAfter} onBlur={onRunQuery} />
                </InlineField>
                <InlineField label="Before" labelWidth={DATE_LABEL_WIDTH} tooltip="Upper bound. ISO-8601 or Unix millis.">
                  <Input width={DATE_INPUT_WIDTH} value={dueBefore ?? ''} placeholder="2024-12-31" onChange={onDueBefore} onBlur={onRunQuery} />
                </InlineField>
              </>
            )}
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField label="Include closed" labelWidth={LABEL_WIDTH} tooltip="Include closed tasks in the results.">
              <InlineSwitch value={!!includeClosed} onChange={(e) => updateAndRun({ includeClosed: e.currentTarget.checked })} />
            </InlineField>
            <InlineField label="Subtasks" labelWidth={14} tooltip="Include subtasks in the results.">
              <InlineSwitch value={!!includeSubtasks} onChange={(e) => updateAndRun({ includeSubtasks: e.currentTarget.checked })} />
            </InlineField>
            <InlineField label="Archived" labelWidth={14} tooltip="Include archived tasks in the results.">
              <InlineSwitch value={!!includeArchived} onChange={(e) => updateAndRun({ includeArchived: e.currentTarget.checked })} />
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
              tooltip="A ClickUp REST GET path relative to the API root, e.g. /v2/team/123/task. The first array of objects in the response is flattened into a table."
              grow
            >
              <Input width={FIELDS_WIDTH} value={rawPath ?? ''} placeholder={RAW_PLACEHOLDER} onChange={onRawPathChange} onBlur={onRunQuery} />
            </InlineField>
          </InlineFieldRow>
          <InlineFieldRow>
            <InlineField
              label="Response key"
              labelWidth={LABEL_WIDTH}
              tooltip="Optional. The JSON key in the response holding the array (or object) to flatten into rows, e.g. 'tasks'. Leave empty to auto-detect."
            >
              <Input width={INPUT_WIDTH} value={rawRoot ?? ''} placeholder="auto-detect" onChange={onRawRootChange} onBlur={onRunQuery} />
            </InlineField>
          </InlineFieldRow>
        </>
      )}

      {!isRaw && !isTeams && (
        <InlineFieldRow>
          {isTasks && (
            <InlineField label="Order by" labelWidth={LABEL_WIDTH} tooltip="Order tasks by a field.">
              <RadioButtonGroup<ClickUpOrderBy> options={ORDER_BY_OPTIONS} value={orderBy} onChange={onOrderByChange} />
            </InlineField>
          )}
          {isTasks && (
            <InlineField label="Reverse" labelWidth={12} tooltip="Reverse the order direction.">
              <InlineSwitch value={!!reverse} onChange={(e) => updateAndRun({ reverse: e.currentTarget.checked })} />
            </InlineField>
          )}
          <InlineField label="Limit" labelWidth={10} tooltip="Maximum number of records. 0 returns all (auto-paginated).">
            <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
          </InlineField>
        </InlineFieldRow>
      )}
    </div>
  );
}
