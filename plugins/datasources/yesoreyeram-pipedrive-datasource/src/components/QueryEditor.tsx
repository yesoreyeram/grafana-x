import React, { useCallback, useEffect, useState, type ChangeEvent } from 'react';
import {
  InlineField,
  InlineFieldRow,
  InlineSwitch,
  Input,
  RadioButtonGroup,
  Select,
} from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  PipedriveQuery,
  PipedriveDataSourceOptions,
  PipedriveQueryType,
  PipedriveCountEntity,
  Filter,
  FilterGroup,
  PipelineInfo,
  StageInfo,
  UserInfo,
  QUERY_TYPE_OPTIONS,
  COUNT_ENTITY_OPTIONS,
  DEAL_STATUS_OPTIONS,
  DEFAULT_QUERY,
} from '../types';

type Props = QueryEditorProps<DataSource, PipedriveQuery, PipedriveDataSourceOptions>;

const LABEL_WIDTH = 26;
const INPUT_WIDTH = 40;

const SORT_DIR_OPTIONS: Array<SelectableValue<'ASC' | 'DESC'>> = [
  { label: 'Ascending', value: 'ASC' },
  { label: 'Descending', value: 'DESC' },
];

function pipelineOptions(pipelines: PipelineInfo[]): Array<SelectableValue<string>> {
  return pipelines.map((p) => ({ label: p.name, value: String(p.id) }));
}

function stageOptions(stages: StageInfo[]): Array<SelectableValue<string>> {
  return stages.map((s) => ({ label: s.name, value: String(s.id) }));
}

function userOptions(users: UserInfo[]): Array<SelectableValue<string>> {
  return users.map((u) => ({ label: `${u.name} (${u.email})`, value: String(u.id) }));
}

function errMessage(err: unknown, fallback: string): string {
  const e = err as { data?: { error?: string }; message?: string };
  return e?.data?.error ?? e?.message ?? fallback;
}

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
    if (!enabled) { return; }
    let cancelled = false;
    setLoading(true);
    setError(undefined);
    loader()
      .then((items) => {
        if (!cancelled) { setOptions(toOptions(items)); }
      })
      .catch((err) => {
        if (!cancelled) {
          setOptions([]);
          setError(errMessage(err, 'Failed to load'));
        }
      })
      .finally(() => {
        if (!cancelled) { setLoading(false); }
      });
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return { options, loading, error };
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const {
    filterGroups,
    sortBy,
    sortDir,
    pipelineId,
    stageId,
    userId,
    filterId,
    limit,
    start,
  } = query;
  const queryType: PipedriveQueryType = query.queryType ?? DEFAULT_QUERY.queryType!;
  const statusFilter: string = query.statusFilter ?? 'all';
  const countEntity: PipedriveCountEntity = query.countEntity ?? 'deals';
  const mapCustomFields: boolean = query.mapCustomFields ?? true;

  const isEntityQuery = queryType === 'deals' || queryType === 'persons' || queryType === 'organizations' || queryType === 'products';
  const showPipelineFilter = queryType === 'deals';

  // Resource hooks
  const pipelinesResource = useResource<PipelineInfo>(
    () => datasource.getPipelines(),
    pipelineOptions,
    showPipelineFilter,
    [datasource]
  );

  const [stagesFull, setStagesFull] = useState<StageInfo[]>([]);
  useEffect(() => {
    if (!showPipelineFilter) { setStagesFull([]); return; }
    let cancelled = false;
    datasource.getStages(pipelineId).then((p) => { if (!cancelled) { setStagesFull(p); } }).catch(() => { });
    return () => { cancelled = true; };
  }, [datasource, pipelineId, showPipelineFilter]);

  const usersResource = useResource<UserInfo>(
    () => datasource.getUsers(),
    userOptions,
    true,
    [datasource]
  );

  const update = useCallback(
    (patch: Partial<PipedriveQuery>) => onChange({ ...query, ...patch }),
    [onChange, query]
  );

  const updateAndRun = useCallback(
    (patch: Partial<PipedriveQuery>) => { update(patch); onRunQuery(); },
    [update, onRunQuery]
  );

  const onQueryTypeChange = (value: PipedriveQueryType) => updateAndRun({ queryType: value });

  // Filter management
  const currentFilters: FilterGroup[] = filterGroups ?? [];

  const addFilter = () => {
    const newGroups = [...currentFilters];
    const lastGroup = newGroups[newGroups.length - 1];
    if (lastGroup && lastGroup.filters.length < 1) {
      lastGroup.filters.push({ field: '', operator: 'EQ', value: '' });
    } else {
      newGroups.push({ filters: [{ field: '', operator: 'EQ', value: '' }] });
    }
    update({ filterGroups: newGroups });
  };

  const removeFilter = (groupIdx: number, filterIdx: number) => {
    const newGroups = currentFilters.map((g) => ({ ...g, filters: [...g.filters] }));
    newGroups[groupIdx].filters.splice(filterIdx, 1);
    if (newGroups[groupIdx].filters.length === 0) {
      newGroups.splice(groupIdx, 1);
    }
    updateAndRun({ filterGroups: newGroups.length > 0 ? newGroups : [] });
  };

  const updateFilter = (groupIdx: number, filterIdx: number, patch: Partial<Filter>) => {
    const newGroups = currentFilters.map((g) => ({ ...g, filters: [...g.filters] }));
    newGroups[groupIdx].filters[filterIdx] = { ...newGroups[groupIdx].filters[filterIdx], ...patch };
    update({ filterGroups: newGroups });
  };

  return (
    <div>
      <InlineField label="Query type" labelWidth={LABEL_WIDTH} tooltip="Pipedrive query type.">
        <Select<string>
          width={INPUT_WIDTH}
          options={QUERY_TYPE_OPTIONS}
          value={queryType}
          onChange={(v) => { if (v?.value) { onQueryTypeChange(v.value as PipedriveQueryType); } }}
          placeholder="Select query type"
        />
      </InlineField>

      {/* Count entity selector */}
      {queryType === 'count' && (
        <InlineFieldRow>
          <InlineField label="Count entity" labelWidth={LABEL_WIDTH} tooltip="Which entity to count.">
            <Select<PipedriveCountEntity>
              width={INPUT_WIDTH}
              options={COUNT_ENTITY_OPTIONS}
              value={countEntity}
              onChange={(v) => v?.value && updateAndRun({ countEntity: v.value })}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {/* Deals-specific fields */}
      {showPipelineFilter && (
        <>
          <InlineFieldRow>
            <InlineField
              label="Deal status"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter deals by status."
            >
              <RadioButtonGroup<string>
                options={DEAL_STATUS_OPTIONS}
                value={statusFilter}
                onChange={(v) => updateAndRun({ statusFilter: v })}
              />
            </InlineField>
          </InlineFieldRow>
          <InlineFieldRow>
            <InlineField
              label="Pipeline"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by pipeline."
              error={pipelinesResource.error}
              invalid={!!pipelinesResource.error}
            >
              <Select<string>
                width={INPUT_WIDTH}
                isClearable
                isLoading={pipelinesResource.loading}
                options={pipelinesResource.options}
                value={pipelineId ? (pipelinesResource.options.find((o) => o.value === pipelineId) ?? { label: pipelineId, value: pipelineId }) : null}
                onChange={(v) => updateAndRun({ pipelineId: v?.value ?? '', stageId: '' })}
                placeholder="Any pipeline"
                noOptionsMessage="No pipelines found"
              />
            </InlineField>
          </InlineFieldRow>
          {pipelineId && (
            <InlineFieldRow>
              <InlineField label="Stage" labelWidth={LABEL_WIDTH} tooltip="Filter by stage within the selected pipeline.">
                <Select<string>
                  width={INPUT_WIDTH}
                  isClearable
                  options={stageOptions(stagesFull)}
                  value={stageId ? (stageOptions(stagesFull).find((o) => o.value === stageId) ?? { label: stageId, value: stageId }) : null}
                  onChange={(v) => updateAndRun({ stageId: v?.value ?? '' })}
                  placeholder="Any stage"
                />
              </InlineField>
            </InlineFieldRow>
          )}
        </>
      )}

      {/* User filter (deals, persons) */}
      {(queryType === 'deals' || queryType === 'persons') && (
        <InlineFieldRow>
          <InlineField
            label="User"
            labelWidth={LABEL_WIDTH}
            tooltip="Filter by Pipedrive user."
            error={usersResource.error}
            invalid={!!usersResource.error}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={usersResource.loading}
              options={usersResource.options}
              value={userId ? (usersResource.options.find((o) => o.value === userId) ?? { label: userId, value: userId }) : null}
              onChange={(v) => updateAndRun({ userId: v?.value ?? '' })}
              placeholder="Any user"
              noOptionsMessage="No users found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {/* Saved filter + custom field mapping */}
      {(isEntityQuery || queryType === 'count') && (
        <InlineFieldRow>
          <InlineField
            label="Saved filter ID"
            labelWidth={LABEL_WIDTH}
            tooltip="Optional Pipedrive saved filter id. When set, it takes precedence over the status/pipeline/stage/user filters above."
          >
            <Input
              width={INPUT_WIDTH}
              value={filterId ?? ''}
              placeholder="e.g. 123"
              onChange={(e: ChangeEvent<HTMLInputElement>) => update({ filterId: e.target.value })}
              onBlur={onRunQuery}
            />
          </InlineField>
        </InlineFieldRow>
      )}
      {isEntityQuery && (
        <InlineFieldRow>
          <InlineField
            label="Map custom fields"
            labelWidth={LABEL_WIDTH}
            tooltip="Translate 40-character custom field hash keys into their human-readable names using the {entity}Fields endpoint (one extra API call per query)."
          >
            <InlineSwitch
              value={mapCustomFields}
              onChange={(e: ChangeEvent<HTMLInputElement>) => updateAndRun({ mapCustomFields: e.currentTarget.checked })}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {/* Filter builder */}
      {isEntityQuery && (
        <InlineFieldRow>
          <InlineField
            label="Filters"
            labelWidth={LABEL_WIDTH}
            tooltip="Client-side field filters applied after fetch. Multiple filters in a group are AND'd; groups are OR'd."
          >
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              {currentFilters.map((group, gi) =>
                group.filters.map((f, fi) => (
                  <div key={`${gi}-${fi}`} style={{ display: 'flex', gap: '4px', alignItems: 'center' }}>
                    {fi === 0 && gi > 0 && <span style={{ fontWeight: 'bold', marginRight: '4px' }}>OR</span>}
                    {fi > 0 && <span style={{ fontWeight: 'bold', marginRight: '4px' }}>AND</span>}
                    <Input
                      width={16}
                      value={f.field}
                      placeholder="field name"
                      onChange={(e: ChangeEvent<HTMLInputElement>) => updateFilter(gi, fi, { field: e.target.value })}
                      onBlur={onRunQuery}
                    />
                    <Select<string>
                      width={14}
                      options={[
                        { label: 'EQ', value: 'EQ' },
                        { label: 'NEQ', value: 'NEQ' },
                        { label: 'GT', value: 'GT' },
                        { label: 'GTE', value: 'GTE' },
                        { label: 'LT', value: 'LT' },
                        { label: 'LTE', value: 'LTE' },
                        { label: 'LIKE', value: 'LIKE' },
                        { label: 'NOT_LIKE', value: 'NOT_LIKE' },
                      ]}
                      value={f.operator}
                      onChange={(v) => v?.value && updateFilter(gi, fi, { operator: v.value })}
                    />
                    <Input
                      width={18}
                      value={f.value}
                      placeholder="value"
                      onChange={(e: ChangeEvent<HTMLInputElement>) => updateFilter(gi, fi, { value: e.target.value })}
                      onBlur={onRunQuery}
                    />
                    <button
                      className="gf-form-label gf-form-label--btn"
                      onClick={() => removeFilter(gi, fi)}
                      title="Remove filter"
                      style={{ cursor: 'pointer', padding: '0 4px' }}
                    >
                      x
                    </button>
                  </div>
                ))
              )}
              <button
                className="gf-form-label gf-form-label--btn"
                onClick={addFilter}
                style={{ cursor: 'pointer', alignSelf: 'flex-start', marginTop: '2px' }}
              >
                + Add filter
              </button>
            </div>
          </InlineField>
        </InlineFieldRow>
      )}

      {/* Sort */}
      {isEntityQuery && (
        <InlineFieldRow>
          <InlineField
            label="Sort by"
            labelWidth={LABEL_WIDTH}
            tooltip="Field to sort results by."
          >
            <Input
              width={INPUT_WIDTH}
              value={sortBy ?? ''}
              placeholder="add_time"
              onChange={(e: ChangeEvent<HTMLInputElement>) => update({ sortBy: e.target.value })}
              onBlur={onRunQuery}
            />
          </InlineField>
          <InlineField label="Direction" labelWidth={12} tooltip="Sort direction.">
            <RadioButtonGroup<'ASC' | 'DESC'>
              options={SORT_DIR_OPTIONS}
              value={sortDir ?? 'DESC'}
              onChange={(v) => updateAndRun({ sortDir: v })}
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {/* Pagination */}
      {isEntityQuery && (
        <InlineFieldRow>
          <InlineField
            label="Limit"
            labelWidth={LABEL_WIDTH}
            tooltip="Maximum total records to return across all pages. The backend paginates automatically (500 per request). Set to 0 to fetch all matching records."
          >
            <Input
              width={20}
              type="number"
              min={0}
              value={limit ?? 100}
              onChange={(e: ChangeEvent<HTMLInputElement>) => {
                const n = parseInt(e.target.value, 10);
                update({ limit: isNaN(n) ? 100 : n });
              }}
              onBlur={onRunQuery}
            />
          </InlineField>
          <InlineField label="Start" labelWidth={12} tooltip="Pagination offset (start index).">
            <Input
              width={20}
              type="number"
              min={0}
              value={start ?? 0}
              onChange={(e: ChangeEvent<HTMLInputElement>) => {
                const n = parseInt(e.target.value, 10);
                update({ start: isNaN(n) ? 0 : n });
              }}
              onBlur={onRunQuery}
            />
          </InlineField>
        </InlineFieldRow>
      )}
    </div>
  );
}
