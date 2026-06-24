import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import { InlineField, InlineFieldRow, Input, Select } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import { TodoistDataSourceOptions, TodoistQuery, TodoistQueryType, ProjectInfo, SectionInfo, LabelInfo } from '../types';

type Props = QueryEditorProps<DataSource, TodoistQuery, TodoistDataSourceOptions>;

const LABEL_WIDTH = 18;
const INPUT_WIDTH = 40;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<TodoistQueryType>> = [
  { label: 'Tasks', value: 'tasks', description: 'List active tasks with optional filters' },
  { label: 'Count', value: 'count', description: 'Count active tasks matching filters' },
];

const PROJECT_PLACEHOLDER = 'Select a project';

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
          const e = err as { data?: { error?: string }; message?: string };
          setError(e?.data?.error ?? e?.message ?? 'Failed to load');
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

const projectOptions = (items: ProjectInfo[]): Array<SelectableValue<string>> =>
  items.map((i) => ({ label: i.name, value: i.id, description: i.id }));

const sectionOptions = (items: SectionInfo[]): Array<SelectableValue<string>> =>
  items.map((i) => ({ label: i.name, value: i.id, description: i.id }));

// The Todoist `label` parameter filters by label NAME, so the picker stores the
// name as the option value.
const labelOptions = (items: LabelInfo[]): Array<SelectableValue<string>> =>
  items.map((i) => ({ label: i.name, value: i.name }));

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { projectId, sectionId, label, parentId, filter, lang, limit } = query;
  const queryType = query.queryType ?? 'tasks';

  const isTasks = queryType === 'tasks' || queryType === 'count';
  const isCount = queryType === 'count';
  const hasFilter = !!filter && filter.trim() !== '';

  const projects = useResource<ProjectInfo>(() => datasource.getProjects(), projectOptions, isTasks, [
    datasource,
    isTasks,
  ]);

  const sections = useResource<SectionInfo>(
    () => datasource.getSections(projectId),
    sectionOptions,
    isTasks && !!projectId,
    [datasource, isTasks, projectId]
  );

  const labels = useResource<LabelInfo>(() => datasource.getLabels(), labelOptions, isTasks, [datasource, isTasks]);

  const update = useCallback(
    (patch: Partial<TodoistQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const updateAndRun = useCallback(
    (patch: Partial<TodoistQuery>) => {
      update(patch);
      onRunQuery();
    },
    [update, onRunQuery]
  );

  const onQueryTypeChange = (value: TodoistQueryType) => updateAndRun({ queryType: value });

  const onProjectChange = (value: SelectableValue<string> | null) =>
    updateAndRun({ projectId: value?.value ?? '', sectionId: '' });
  const onSectionChange = (value: SelectableValue<string> | null) => updateAndRun({ sectionId: value?.value ?? '' });
  const onLabelChange = (value: SelectableValue<string> | null) => updateAndRun({ label: value?.value ?? '' });
  const onParentIdChange = (e: ChangeEvent<HTMLInputElement>) => update({ parentId: e.target.value });
  const onFilterChange = (e: ChangeEvent<HTMLInputElement>) => update({ filter: e.target.value });
  const onLangChange = (e: ChangeEvent<HTMLInputElement>) => update({ lang: e.target.value });
  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  const selected = (id: string | undefined, opts: Array<SelectableValue<string>>): SelectableValue<string> | null =>
    id ? opts.find((o) => o.value === id) ?? { label: id, value: id } : null;

  const scopeTooltipSuffix = hasFilter ? ' Ignored while a Filter is set.' : '';

  return (
    <div>
      <div className="gf-form">
        <InlineField label="Query type" labelWidth={LABEL_WIDTH} tooltip="What to fetch from Todoist.">
          <Select<TodoistQueryType>
            width={INPUT_WIDTH}
            options={QUERY_TYPE_OPTIONS}
            value={queryType}
            onChange={(v) => v.value && onQueryTypeChange(v.value)}
          />
        </InlineField>
      </div>

      {isTasks && (
        <>
          <InlineFieldRow>
            <InlineField
              label="Project"
              labelWidth={LABEL_WIDTH}
              tooltip={'Filter by project. Optional.' + scopeTooltipSuffix}
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
                placeholder={PROJECT_PLACEHOLDER}
                noOptionsMessage="No projects found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Section"
              labelWidth={LABEL_WIDTH}
              tooltip={'Filter by section. Requires a project. Optional.' + scopeTooltipSuffix}
              error={sections.error}
              invalid={!!sections.error}
            >
              <Select<string>
                width={INPUT_WIDTH}
                isClearable
                isLoading={sections.loading}
                options={sections.options}
                value={selected(sectionId, sections.options)}
                onChange={onSectionChange}
                allowCustomValue
                placeholder="All sections"
                noOptionsMessage="No sections found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Label"
              labelWidth={LABEL_WIDTH}
              tooltip={'Filter by label name. Optional.' + scopeTooltipSuffix}
              error={labels.error}
              invalid={!!labels.error}
            >
              <Select<string>
                width={INPUT_WIDTH}
                isClearable
                isLoading={labels.loading}
                options={labels.options}
                value={selected(label, labels.options)}
                onChange={onLabelChange}
                allowCustomValue
                placeholder="All labels"
                noOptionsMessage="No labels found"
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Parent task ID"
              labelWidth={LABEL_WIDTH}
              tooltip={'Return only the sub-tasks of this parent task. Optional.' + scopeTooltipSuffix}
            >
              <Input
                width={INPUT_WIDTH}
                value={parentId ?? ''}
                placeholder="e.g. 6XGgmFVcrG5RRjVr"
                onChange={onParentIdChange}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Filter"
              labelWidth={LABEL_WIDTH}
              tooltip="A Todoist filter query (e.g. 'today', 'overdue & !recurring', '#Work & p1'). When set, it overrides the project/section/label/parent scope above. Optional."
              grow
            >
              <Input
                width={INPUT_WIDTH}
                value={filter ?? ''}
                placeholder="e.g. today | overdue | #Work & p1"
                onChange={onFilterChange}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>

          {hasFilter && (
            <InlineFieldRow>
              <InlineField
                label="Filter language"
                labelWidth={LABEL_WIDTH}
                tooltip="IETF language tag used to parse the filter string (e.g. 'en', 'de', 'fr'). Optional; defaults to your Todoist language."
              >
                <Input
                  width={20}
                  value={lang ?? ''}
                  placeholder="en"
                  onChange={onLangChange}
                  onBlur={onRunQuery}
                />
              </InlineField>
            </InlineFieldRow>
          )}
        </>
      )}

      <InlineFieldRow>
        <InlineField
          label={isCount ? 'Max task scan' : 'Limit'}
          labelWidth={LABEL_WIDTH}
          tooltip="Maximum number of records. 0 returns all (auto-paginated, up to a safety cap)."
        >
          <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
        </InlineField>
      </InlineFieldRow>
    </div>
  );
}
