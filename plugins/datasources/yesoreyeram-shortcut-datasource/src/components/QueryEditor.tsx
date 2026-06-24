import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import { InlineField, InlineFieldRow, Input, Select, MultiSelect, RadioButtonGroup } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  ARCHIVED_OPTIONS,
  DATE_FIELD_OPTIONS,
  DATE_MODE_OPTIONS,
  DETAIL_OPTIONS,
  STORY_TYPE_OPTIONS,
  ShortcutArchived,
  ShortcutDataSourceOptions,
  ShortcutDateField,
  ShortcutDateMode,
  ShortcutDetail,
  ShortcutQuery,
  ShortcutQueryType,
  StoryType,
  ProjectInfo,
  WorkflowStateInfo,
  EpicInfo,
  IterationInfo,
  MemberInfo,
  TeamInfo,
  LabelInfo,
} from '../types';

type Props = QueryEditorProps<DataSource, ShortcutQuery, ShortcutDataSourceOptions>;

const LABEL_WIDTH = 18;
const INPUT_WIDTH = 40;
const DATE_LABEL_WIDTH = 8;
const DATE_INPUT_WIDTH = 18;
const FIELDS_WIDTH = 80;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<ShortcutQueryType>> = [
  { label: 'Stories', value: 'stories', description: 'List stories matching the search filters' },
  { label: 'Count', value: 'count', description: 'Return the number of matching stories (via the search total)' },
];

const DATE_MODE_SELECT_OPTIONS: Array<SelectableValue<ShortcutDateMode>> = DATE_MODE_OPTIONS.map((m) => ({
  label: m.label,
  value: m.value,
  description: m.description,
}));

const DATE_FIELD_SELECT_OPTIONS: Array<SelectableValue<ShortcutDateField>> = DATE_FIELD_OPTIONS.map((m) => ({
  label: m.label,
  value: m.value,
  description: m.description,
}));

const ARCHIVED_SELECT_OPTIONS: Array<SelectableValue<ShortcutArchived>> = ARCHIVED_OPTIONS.map((m) => ({
  label: m.label,
  value: m.value,
  description: m.description,
}));

const DETAIL_SELECT_OPTIONS: Array<SelectableValue<ShortcutDetail>> = DETAIL_OPTIONS.map((m) => ({
  label: m.label,
  value: m.value,
  description: m.description,
}));

/** Map a stored string[] value to selected options, preserving custom values. */
function toMulti(values: string[] | undefined, options: Array<SelectableValue<string>>): Array<SelectableValue<string>> {
  return (values ?? []).map((v) => options.find((o) => o.value === v) ?? { label: v, value: v });
}

/** Extract plain string values from selected options. */
function multiValues(values: Array<SelectableValue<string>>): string[] {
  return values.map((v) => v.value).filter((v): v is string => v != null && v !== '');
}

/** Resolve a single stored value to an option, preserving custom values. */
function selected(value: string | undefined, opts: Array<SelectableValue<string>>): SelectableValue<string> | null {
  return value ? opts.find((o) => o.value === value) ?? { label: value, value } : null;
}

/** Read a resource-call error into a short message. */
function errMessage(err: unknown, fallback: string): string {
  const e = err as { data?: { error?: string }; message?: string };
  return e?.data?.error ?? e?.message ?? fallback;
}

/**
 * useResource loads a list via the provided loader whenever `enabled` is true and
 * any dependency changes. Each list loads independently so one failure never
 * blanks the others.
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
    query: searchText,
    projects,
    workflowStates,
    epic,
    iteration,
    labels,
    owners,
    teams,
    fields,
    createdAfter,
    createdBefore,
    updatedAfter,
    updatedBefore,
    deadlineAfter,
    deadlineBefore,
    limit,
  } = query;
  const queryType = query.queryType ?? 'stories';
  const storyType = query.storyType ?? '';
  const dateMode: ShortcutDateMode = query.dateMode ?? 'any';
  const dateField: ShortcutDateField = query.dateField ?? 'created';
  const archived: ShortcutArchived = query.archived ?? 'any';
  const detail: ShortcutDetail = query.detail ?? 'full';

  // Stories and Count share the same filters.
  const isStories = queryType === 'stories' || queryType === 'count';
  const isStoriesOnly = queryType === 'stories';

  const projectList = useResource<ProjectInfo>(
    () => datasource.getProjects(),
    (items) => items.map((p) => ({ label: p.name, value: p.name })),
    isStories,
    [datasource, isStories]
  );

  const stateList = useResource<WorkflowStateInfo>(
    () => datasource.getWorkflows(),
    (items) => items.map((s) => ({ label: s.type ? `${s.name} (${s.type})` : s.name, value: s.name })),
    isStories,
    [datasource, isStories]
  );

  const epicList = useResource<EpicInfo>(
    () => datasource.getEpics(),
    (items) => items.map((e) => ({ label: e.name, value: e.name })),
    isStories,
    [datasource, isStories]
  );

  const iterationList = useResource<IterationInfo>(
    () => datasource.getIterations(),
    (items) => items.map((i) => ({ label: i.name, value: i.name })),
    isStories,
    [datasource, isStories]
  );

  const labelList = useResource<LabelInfo>(
    () => datasource.getLabels(),
    (items) => items.map((l) => ({ label: l.name, value: l.name })),
    isStories,
    [datasource, isStories]
  );

  // Owners filter by mention name (the owner: operator's expected value).
  const ownerList = useResource<MemberInfo>(
    () => datasource.getMembers(),
    (items) =>
      items.map((m) => ({
        label: m.name ? `${m.name} (@${m.mention_name})` : m.mention_name,
        value: m.mention_name,
      })),
    isStories,
    [datasource, isStories]
  );

  // Teams filter by name (the team: operator uses the name, not the mention name).
  const teamList = useResource<TeamInfo>(
    () => datasource.getTeams(),
    (items) => items.map((t) => ({ label: t.name, value: t.name })),
    isStories,
    [datasource, isStories]
  );

  const fieldList = useResource<string>(
    () => datasource.getStoryFields(),
    (items) => items.map((f) => ({ label: f, value: f })),
    isStoriesOnly,
    [datasource, isStoriesOnly]
  );

  const update = useCallback(
    (patch: Partial<ShortcutQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const updateAndRun = useCallback(
    (patch: Partial<ShortcutQuery>) => {
      update(patch);
      onRunQuery();
    },
    [update, onRunQuery]
  );

  const onQueryTypeChange = (value: ShortcutQueryType) => updateAndRun({ queryType: value });
  const onStoryTypeChange = (value: SelectableValue<string> | null) =>
    updateAndRun({ storyType: (value?.value ?? '') as StoryType | '' });

  const onSearchTextChange = (e: ChangeEvent<HTMLInputElement>) => update({ query: e.target.value });

  const onCreatedAfter = (e: ChangeEvent<HTMLInputElement>) => update({ createdAfter: e.target.value });
  const onCreatedBefore = (e: ChangeEvent<HTMLInputElement>) => update({ createdBefore: e.target.value });
  const onUpdatedAfter = (e: ChangeEvent<HTMLInputElement>) => update({ updatedAfter: e.target.value });
  const onUpdatedBefore = (e: ChangeEvent<HTMLInputElement>) => update({ updatedBefore: e.target.value });
  const onDeadlineAfter = (e: ChangeEvent<HTMLInputElement>) => update({ deadlineAfter: e.target.value });
  const onDeadlineBefore = (e: ChangeEvent<HTMLInputElement>) => update({ deadlineBefore: e.target.value });

  const onDateModeChange = (mode: ShortcutDateMode) =>
    updateAndRun(
      mode === 'custom'
        ? { dateMode: mode }
        : {
            dateMode: mode,
            createdAfter: '',
            createdBefore: '',
            updatedAfter: '',
            updatedBefore: '',
            deadlineAfter: '',
            deadlineBefore: '',
          }
    );

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  return (
    <div>
      <div className="gf-form">
        <InlineField label="Query type" labelWidth={LABEL_WIDTH} tooltip="What to fetch from Shortcut.">
          <RadioButtonGroup<ShortcutQueryType> options={QUERY_TYPE_OPTIONS} value={queryType} onChange={onQueryTypeChange} />
        </InlineField>
      </div>

      {isStories && (
        <>
          <InlineFieldRow>
            <InlineField
              label="Search query"
              labelWidth={LABEL_WIDTH}
              tooltip="Optional raw Shortcut search query (operators + free text), e.g. 'login is:started owner:alice'. Combined with the filters below using AND."
              grow
            >
              <Input
                width={FIELDS_WIDTH}
                value={searchText ?? ''}
                placeholder='e.g. login state:"In Progress" owner:alice'
                onChange={onSearchTextChange}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField label="Story type" labelWidth={LABEL_WIDTH} tooltip="Filter by story type (type:).">
              <Select<string>
                width={INPUT_WIDTH}
                isClearable
                options={STORY_TYPE_OPTIONS}
                value={selected(storyType || undefined, STORY_TYPE_OPTIONS)}
                onChange={onStoryTypeChange}
                placeholder="Any type"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Projects"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by project name (project:). Search uses AND, so multiple projects rarely match."
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
            <InlineField
              label="Workflow states"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by workflow state name (state:). Search uses AND, so multiple states rarely match."
              error={stateList.error}
              invalid={!!stateList.error}
            >
              <MultiSelect<string>
                width={INPUT_WIDTH}
                isLoading={stateList.loading}
                options={stateList.options}
                value={toMulti(workflowStates, stateList.options)}
                onChange={(v) => updateAndRun({ workflowStates: multiValues(v) })}
                allowCustomValue
                placeholder="Any state"
                noOptionsMessage="No states found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Epic"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by epic name (epic:)."
              error={epicList.error}
              invalid={!!epicList.error}
            >
              <Select<string>
                width={INPUT_WIDTH}
                isClearable
                isLoading={epicList.loading}
                options={epicList.options}
                value={selected(epic, epicList.options)}
                onChange={(v) => updateAndRun({ epic: v?.value ?? '' })}
                allowCustomValue
                placeholder="Any epic"
                noOptionsMessage="No epics found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Iteration"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by iteration name (iteration:)."
              error={iterationList.error}
              invalid={!!iterationList.error}
            >
              <Select<string>
                width={INPUT_WIDTH}
                isClearable
                isLoading={iterationList.loading}
                options={iterationList.options}
                value={selected(iteration, iterationList.options)}
                onChange={(v) => updateAndRun({ iteration: v?.value ?? '' })}
                allowCustomValue
                placeholder="Any iteration"
                noOptionsMessage="No iterations found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Labels"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by label name (label:). Multiple labels match stories carrying all of them (AND)."
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
              label="Owners"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by owner mention name (owner:). Multiple owners match stories owned by all of them (AND)."
              error={ownerList.error}
              invalid={!!ownerList.error}
            >
              <MultiSelect<string>
                width={INPUT_WIDTH}
                isLoading={ownerList.loading}
                options={ownerList.options}
                value={toMulti(owners, ownerList.options)}
                onChange={(v) => updateAndRun({ owners: multiValues(v) })}
                allowCustomValue
                placeholder="Any owner"
                noOptionsMessage="No members found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Teams"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by team name (team:)."
              error={teamList.error}
              invalid={!!teamList.error}
            >
              <MultiSelect<string>
                width={INPUT_WIDTH}
                isLoading={teamList.loading}
                options={teamList.options}
                value={toMulti(teams, teamList.options)}
                onChange={(v) => updateAndRun({ teams: multiValues(v) })}
                allowCustomValue
                placeholder="Any team"
                noOptionsMessage="No teams found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField label="Archived" labelWidth={LABEL_WIDTH} tooltip="Constrain on archived state (is:archived).">
              <RadioButtonGroup<ShortcutArchived>
                options={ARCHIVED_SELECT_OPTIONS}
                value={archived}
                onChange={(v) => updateAndRun({ archived: v })}
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Date mode"
              labelWidth={LABEL_WIDTH}
              tooltip="Source of date filters. 'Dashboard range' applies the panel's time picker to one date field; 'Custom' lets you enter explicit bounds. Shortcut search uses date-only (YYYY-MM-DD) precision."
            >
              <RadioButtonGroup<ShortcutDateMode>
                options={DATE_MODE_SELECT_OPTIONS}
                value={dateMode}
                onChange={onDateModeChange}
              />
            </InlineField>
            {dateMode === 'dashboard' && (
              <InlineField label="Date field" labelWidth={DATE_LABEL_WIDTH + 2} tooltip="Which date the dashboard range applies to.">
                <RadioButtonGroup<ShortcutDateField>
                  options={DATE_FIELD_SELECT_OPTIONS}
                  value={dateField}
                  onChange={(v) => updateAndRun({ dateField: v })}
                />
              </InlineField>
            )}
          </InlineFieldRow>

          {dateMode === 'custom' && (
            <>
              <InlineFieldRow>
                <InlineField label="Created after" labelWidth={DATE_LABEL_WIDTH} tooltip="Lower bound for created (created:). YYYY-MM-DD.">
                  <Input width={DATE_INPUT_WIDTH} value={createdAfter ?? ''} placeholder="2024-01-01" onChange={onCreatedAfter} onBlur={onRunQuery} />
                </InlineField>
                <InlineField label="Created before" labelWidth={DATE_LABEL_WIDTH} tooltip="Upper bound for created (created:). YYYY-MM-DD.">
                  <Input width={DATE_INPUT_WIDTH} value={createdBefore ?? ''} placeholder="2024-12-31" onChange={onCreatedBefore} onBlur={onRunQuery} />
                </InlineField>
              </InlineFieldRow>
              <InlineFieldRow>
                <InlineField label="Updated after" labelWidth={DATE_LABEL_WIDTH} tooltip="Lower bound for updated (updated:). YYYY-MM-DD.">
                  <Input width={DATE_INPUT_WIDTH} value={updatedAfter ?? ''} placeholder="2024-01-01" onChange={onUpdatedAfter} onBlur={onRunQuery} />
                </InlineField>
                <InlineField label="Updated before" labelWidth={DATE_LABEL_WIDTH} tooltip="Upper bound for updated (updated:). YYYY-MM-DD.">
                  <Input width={DATE_INPUT_WIDTH} value={updatedBefore ?? ''} placeholder="2024-12-31" onChange={onUpdatedBefore} onBlur={onRunQuery} />
                </InlineField>
              </InlineFieldRow>
              <InlineFieldRow>
                <InlineField label="Deadline after" labelWidth={DATE_LABEL_WIDTH} tooltip="Lower bound for deadline (due:). YYYY-MM-DD.">
                  <Input width={DATE_INPUT_WIDTH} value={deadlineAfter ?? ''} placeholder="2024-01-01" onChange={onDeadlineAfter} onBlur={onRunQuery} />
                </InlineField>
                <InlineField label="Deadline before" labelWidth={DATE_LABEL_WIDTH} tooltip="Upper bound for deadline (due:). YYYY-MM-DD.">
                  <Input width={DATE_INPUT_WIDTH} value={deadlineBefore ?? ''} placeholder="2024-12-31" onChange={onDeadlineBefore} onBlur={onRunQuery} />
                </InlineField>
              </InlineFieldRow>
            </>
          )}

          {isStoriesOnly && (
            <>
              <InlineFieldRow>
                <InlineField label="Detail" labelWidth={LABEL_WIDTH} tooltip="Amount of detail returned per story.">
                  <RadioButtonGroup<ShortcutDetail>
                    options={DETAIL_SELECT_OPTIONS}
                    value={detail}
                    onChange={(v) => updateAndRun({ detail: v })}
                  />
                </InlineField>
              </InlineFieldRow>

              <InlineFieldRow>
                <InlineField
                  label="Fields"
                  labelWidth={LABEL_WIDTH}
                  tooltip="Columns to return. Leave empty for the default story field catalog."
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

          <InlineFieldRow>
            <InlineField label="Limit" labelWidth={LABEL_WIDTH} tooltip="Maximum number of records. 0 returns all matches (Shortcut search caps results at 1000).">
              <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
            </InlineField>
          </InlineFieldRow>
        </>
      )}
    </div>
  );
}
