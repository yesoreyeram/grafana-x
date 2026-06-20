import React, { useCallback, useEffect, useState, type ChangeEvent } from 'react';
import {
  InlineField,
  InlineFieldRow,
  Input,
  MultiSelect,
  RadioButtonGroup,
  Select,
  TextArea,
} from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  HubSpotQuery,
  HubSpotDataSourceOptions,
  HubSpotQueryType,
  HubSpotDateMode,
  Filter,
  FilterGroup,
  PropertyInfo,
  PipelineInfo,
  QUERY_TYPE_OPTIONS,
  DATE_MODE_OPTIONS,
  SEARCH_OPERATORS,
  PIPELINE_OBJECT_TYPES,
  DEFAULT_QUERY,
} from '../types';

type Props = QueryEditorProps<DataSource, HubSpotQuery, HubSpotDataSourceOptions>;

const LABEL_WIDTH = 26;
const INPUT_WIDTH = 40;
const WIDE_WIDTH = 60;

const SORT_DIR_OPTIONS: Array<SelectableValue<'ASCENDING' | 'DESCENDING'>> = [
  { label: 'Ascending', value: 'ASCENDING' },
  { label: 'Descending', value: 'DESCENDING' },
];

const RAW_METHOD_OPTIONS: Array<SelectableValue<'GET' | 'POST'>> = [
  { label: 'GET', value: 'GET' },
  { label: 'POST', value: 'POST' },
];

function propertyOptions(props: PropertyInfo[]): Array<SelectableValue<string>> {
  return props.map((p) => ({ label: `${p.label} (${p.name})`, value: p.name, description: p.type }));
}

function pipelineOptions(pipelines: PipelineInfo[]): Array<SelectableValue<string>> {
  return pipelines.map((p) => ({ label: p.label, value: p.id }));
}

function stageOptions(pipelines: PipelineInfo[], pipelineId?: string): Array<SelectableValue<string>> {
  const pipeline = pipelines.find((p) => p.id === pipelineId);
  if (!pipeline) {return [];}
  return pipeline.stages.map((s) => ({ label: s.label, value: s.id }));
}

function toMulti(values: string[] | undefined, options: Array<SelectableValue<string>>): Array<SelectableValue<string>> {
  return (values ?? []).map((v) => options.find((o) => o.value === v) ?? { label: v, value: v });
}

function multiValues(values: Array<SelectableValue<string>>): string[] {
  return values.map((v) => v.value).filter((v): v is string => v != null && v !== '');
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
    if (!enabled) {return;}
    let cancelled = false;
    setLoading(true);
    setError(undefined);
    loader()
      .then((items) => {
        if (!cancelled) {setOptions(toOptions(items));}
      })
      .catch((err) => {
        if (!cancelled) {
          setOptions([]);
          setError(errMessage(err, 'Failed to load'));
        }
      })
      .finally(() => {
        if (!cancelled) {setLoading(false);}
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
    properties,
    pipelineId,
    stageId,
    createdAfter,
    createdBefore,
    updatedAfter,
    updatedBefore,
    limit,
    objectType,
    rawPath,
    rawMethod,
    rawBody,
    rawRoot,
  } = query;
  const queryType: HubSpotQueryType = query.queryType ?? DEFAULT_QUERY.queryType!;
  const createdMode: HubSpotDateMode = query.createdMode ?? 'any';
  const updatedMode: HubSpotDateMode = query.updatedMode ?? 'any';

  const isSearchType = !['pipelines', 'owners', 'properties', 'raw'].includes(queryType);
  const isPipelineOrStageSearch = PIPELINE_OBJECT_TYPES.includes(queryType);

  // Resource hooks for dynamic dropdowns
  const objectTypeForProps = queryType === 'properties' ? objectType : queryType;
  const propsResource = useResource<PropertyInfo>(
    () => datasource.getProperties(objectTypeForProps),
    propertyOptions,
    isSearchType || queryType === 'properties',
    [datasource, queryType, objectType]
  );

  const pipelinesResource = useResource<PipelineInfo>(
    () => datasource.getPipelines(isPipelineOrStageSearch ? queryType : 'deals'),
    pipelineOptions,
    isPipelineOrStageSearch,
    [datasource, queryType]
  );

  const [pipelinesFull, setPipelinesFull] = useState<PipelineInfo[]>([]);
  useEffect(() => {
    if (!isPipelineOrStageSearch) { setPipelinesFull([]); return; }
    let cancelled = false;
    datasource.getPipelines(queryType).then((p) => { if (!cancelled) {setPipelinesFull(p);} }).catch(() => {});
    return () => { cancelled = true; };
  }, [datasource, queryType, isPipelineOrStageSearch]);

  const stageOptionsList = stageOptions(pipelinesFull, pipelineId);

  const update = useCallback(
    (patch: Partial<HubSpotQuery>) => onChange({ ...query, ...patch }),
    [onChange, query]
  );

  const updateAndRun = useCallback(
    (patch: Partial<HubSpotQuery>) => { update(patch); onRunQuery(); },
    [update, onRunQuery]
  );

  const onInputChange = (field: string) => (e: ChangeEvent<HTMLInputElement>) => {
    update({ [field]: e.target.value } as Partial<HubSpotQuery>);
  };

  const onQueryTypeChange = (value: HubSpotQueryType) => updateAndRun({ queryType: value });

  // Filter management
  const currentFilters: FilterGroup[] = filterGroups ?? [];

  const addFilter = () => {
    const newGroups = [...currentFilters];
    const lastGroup = newGroups[newGroups.length - 1];
    if (lastGroup && lastGroup.filters.length < 1) {
      lastGroup.filters.push({ propertyName: '', operator: 'EQ', value: '' });
    } else {
      newGroups.push({ filters: [{ propertyName: '', operator: 'EQ', value: '' }] });
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

  const selectedSortBy: SelectableValue<string> | null = sortBy
    ? propsResource.options.find((o) => o.value === sortBy) ?? { label: sortBy, value: sortBy }
    : null;

  const selectedProperties = toMulti(properties, propsResource.options);
  const selectedPipeline: SelectableValue<string> | null = pipelineId
    ? pipelinesResource.options.find((o) => o.value === pipelineId) ?? { label: pipelineId, value: pipelineId }
    : null;
  const selectedStage: SelectableValue<string> | null = stageId
    ? stageOptionsList.find((o) => o.value === stageId) ?? { label: stageId, value: stageId }
    : null;

  return (
    <div>
      <InlineField label="Query type" labelWidth={LABEL_WIDTH} tooltip="HubSpot query type. CRM objects use the Search API; Pipelines/Owners/Properties use dedicated endpoints; Raw REST for custom API calls.">
        <Select<string>
          width={INPUT_WIDTH}
          options={QUERY_TYPE_OPTIONS}
          value={queryType}
          onChange={(v) => { if (v?.value) { onQueryTypeChange(v.value as HubSpotQueryType); } }}
          placeholder="Select query type"
        />
      </InlineField>

      {/* ---------- CRM Search Types (contacts, companies, deals, tickets, etc.) ---------- */}
      {isSearchType && (
        <>
          {/* Property filter builder */}
          <InlineFieldRow>
            <InlineField
              label="Filters"
              labelWidth={LABEL_WIDTH}
              tooltip="Add property filters using the CRM Search API. Multiple groups are OR'd; filters within a group are AND'd."
            >
              <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                {currentFilters.map((group, gi) =>
                  group.filters.map((f, fi) => (
                    <div key={`${gi}-${fi}`} style={{ display: 'flex', gap: '4px', alignItems: 'center' }}>
                      {fi === 0 && gi > 0 && <span style={{ fontWeight: 'bold', marginRight: '4px' }}>OR</span>}
                      {fi > 0 && <span style={{ fontWeight: 'bold', marginRight: '4px' }}>AND</span>}
                      <Select<string>
                        width={22}
                        isLoading={propsResource.loading}
                        options={propsResource.options}
                        value={f.propertyName ? propsResource.options.find((o) => o.value === f.propertyName) ?? { label: f.propertyName, value: f.propertyName } : null}
                        onChange={(v) => updateFilter(gi, fi, { propertyName: v?.value ?? '' })}
                        allowCustomValue
                        placeholder="Property"
                        noOptionsMessage="No properties found"
                      />
                      <Select<string>
                        width={18}
                        options={SEARCH_OPERATORS}
                        value={f.operator}
                        onChange={(v) => v?.value && updateFilter(gi, fi, { operator: v.value })}
                      />
                      <Input
                        width={20}
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

          {/* Pipeline / Stage (deals, tickets) */}
          {isPipelineOrStageSearch && (
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
                  value={selectedPipeline}
                  onChange={(v) => updateAndRun({ pipelineId: v?.value ?? '', stageId: '' })}
                  placeholder="Any pipeline"
                  noOptionsMessage="No pipelines found"
                />
              </InlineField>
              {pipelineId && (
                <InlineField label="Stage" labelWidth={12} tooltip="Filter by stage within the selected pipeline.">
                  <Select<string>
                    width={INPUT_WIDTH}
                    isClearable
                    options={stageOptionsList}
                    value={selectedStage}
                    onChange={(v) => updateAndRun({ stageId: v?.value ?? '' })}
                    placeholder="Any stage"
                  />
                </InlineField>
              )}
            </InlineFieldRow>
          )}

          {/* Sort */}
          <InlineFieldRow>
            <InlineField
              label="Sort by"
              labelWidth={LABEL_WIDTH}
              tooltip="Property to sort results by."
              error={propsResource.error}
              invalid={!!propsResource.error}
            >
              <Select<string>
                width={INPUT_WIDTH}
                isClearable
                isLoading={propsResource.loading}
                options={propsResource.options}
                value={selectedSortBy}
                onChange={(v) => updateAndRun({ sortBy: v?.value ?? '' })}
                allowCustomValue
                placeholder="createdate"
                noOptionsMessage="No properties found"
              />
            </InlineField>
            <InlineField label="Direction" labelWidth={12} tooltip="Sort direction.">
              <RadioButtonGroup<'ASCENDING' | 'DESCENDING'>
                options={SORT_DIR_OPTIONS}
                value={sortDir ?? 'DESCENDING'}
                onChange={(v) => updateAndRun({ sortDir: v })}
              />
            </InlineField>
          </InlineFieldRow>

          {/* Properties to return */}
          <InlineFieldRow>
            <InlineField
              label="Properties"
              labelWidth={LABEL_WIDTH}
              tooltip="Properties to return. Leave empty for HubSpot defaults."
              error={propsResource.error}
              invalid={!!propsResource.error}
            >
              <MultiSelect<string>
                width={WIDE_WIDTH}
                isLoading={propsResource.loading}
                options={propsResource.options}
                value={selectedProperties}
                onChange={(v) => update({ properties: multiValues(v) })}
                placeholder="Default properties"
                noOptionsMessage="No properties found"
              />
            </InlineField>
          </InlineFieldRow>

          {/* Created date */}
          <InlineFieldRow>
            <InlineField
              label="Created"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by created date (createdate property)."
            >
              <RadioButtonGroup<HubSpotDateMode>
                options={DATE_MODE_OPTIONS}
                value={createdMode}
                onChange={(v) => updateAndRun({ createdMode: v })}
              />
            </InlineField>
          </InlineFieldRow>
          {createdMode === 'custom' && (
            <InlineFieldRow>
              <InlineField label="After" labelWidth={LABEL_WIDTH} tooltip="Created after (ISO-8601 or RFC3339).">
                <Input
                  width={INPUT_WIDTH}
                  value={createdAfter ?? ''}
                  placeholder="2024-01-17T19:55:04Z"
                  onChange={onInputChange('createdAfter')}
                  onBlur={onRunQuery}
                />
              </InlineField>
              <InlineField label="Before" labelWidth={12} tooltip="Created before.">
                <Input
                  width={INPUT_WIDTH}
                  value={createdBefore ?? ''}
                  placeholder="2024-12-31T23:59:59Z"
                  onChange={onInputChange('createdBefore')}
                  onBlur={onRunQuery}
                />
              </InlineField>
            </InlineFieldRow>
          )}

          {/* Updated date */}
          <InlineFieldRow>
            <InlineField
              label="Updated"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter by last modified date (hs_lastmodifieddate property)."
            >
              <RadioButtonGroup<HubSpotDateMode>
                options={DATE_MODE_OPTIONS}
                value={updatedMode}
                onChange={(v) => updateAndRun({ updatedMode: v })}
              />
            </InlineField>
          </InlineFieldRow>
          {updatedMode === 'custom' && (
            <InlineFieldRow>
              <InlineField label="After" labelWidth={LABEL_WIDTH} tooltip="Updated after (ISO-8601 or RFC3339).">
                <Input
                  width={INPUT_WIDTH}
                  value={updatedAfter ?? ''}
                  placeholder="2024-01-17T19:55:04Z"
                  onChange={onInputChange('updatedAfter')}
                  onBlur={onRunQuery}
                />
              </InlineField>
              <InlineField label="Before" labelWidth={12} tooltip="Updated before.">
                <Input
                  width={INPUT_WIDTH}
                  value={updatedBefore ?? ''}
                  placeholder="2024-12-31T23:59:59Z"
                  onChange={onInputChange('updatedBefore')}
                  onBlur={onRunQuery}
                />
              </InlineField>
            </InlineFieldRow>
          )}

          {/* Limit */}
          <InlineFieldRow>
            <InlineField
              label="Limit"
              labelWidth={LABEL_WIDTH}
              tooltip="Maximum number of records. 0 returns all (auto-paginated)."
            >
              <Input
                width={20}
                type="number"
                min={0}
                value={limit ?? 0}
                onChange={(e: ChangeEvent<HTMLInputElement>) => {
                  const n = parseInt(e.target.value, 10);
                  update({ limit: isNaN(n) ? 0 : n });
                }}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>
        </>
      )}

      {/* ---------- Pipelines / Properties (need object type selector) ---------- */}
      {(queryType === 'pipelines' || queryType === 'properties') && (
        <InlineFieldRow>
          <InlineField
            label={queryType === 'pipelines' ? 'Pipeline object type' : 'Object type'}
            labelWidth={LABEL_WIDTH}
            tooltip={queryType === 'pipelines' ? 'Object to list pipelines for.' : 'Object type to list properties for.'}
          >
            <Select<string>
              width={INPUT_WIDTH}
              allowCustomValue
              options={[{ label: 'contacts', value: 'contacts' }, { label: 'companies', value: 'companies' }, { label: 'deals', value: 'deals' }, { label: 'tickets', value: 'tickets' }, { label: 'products', value: 'products' }, { label: 'line_items', value: 'line_items' }]}
              value={objectType ? { label: objectType, value: objectType } : null}
              onChange={(v) => updateAndRun({ objectType: v?.value ?? '' })}
              placeholder="Select object type"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {/* ---------- Raw REST ---------- */}
      {queryType === 'raw' && (
        <>
          <InlineFieldRow>
            <InlineField
              label="Method"
              labelWidth={LABEL_WIDTH}
              tooltip="HTTP method for the raw REST call."
            >
              <RadioButtonGroup<'GET' | 'POST'>
                options={RAW_METHOD_OPTIONS}
                value={rawMethod ?? 'GET'}
                onChange={(v) => update({ rawMethod: v })}
              />
            </InlineField>
          </InlineFieldRow>
          <InlineFieldRow>
            <InlineField
              label="Path"
              labelWidth={LABEL_WIDTH}
              tooltip="API path relative to the base URL (e.g. /crm/v3/objects/contacts)."
              grow
            >
              <Input
                value={rawPath ?? ''}
                placeholder="/crm/v3/objects/contacts"
                onChange={onInputChange('rawPath')}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>
          {rawMethod === 'POST' && (
            <div className="gf-form" style={{ alignItems: 'flex-start' }}>
              <InlineField
                label="Body"
                labelWidth={LABEL_WIDTH}
                tooltip="JSON body for the POST request."
                grow
              >
                <TextArea
                  rows={5}
                  value={rawBody ?? ''}
                  placeholder='{"filterGroups": [{"filters": [{"propertyName": "email", "operator": "CONTAINS_TOKEN", "value": "*@hubspot.com"}]}]}'
                  onChange={(e: React.FormEvent<HTMLTextAreaElement>) => update({ rawBody: e.currentTarget.value })}
                  onBlur={onRunQuery}
                />
              </InlineField>
            </div>
          )}
          <InlineFieldRow>
            <InlineField
              label="Root key"
              labelWidth={LABEL_WIDTH}
              tooltip="Optional JSON key whose value (an array) is flattened into rows. Defaults to 'results'."
            >
              <Input
                width={INPUT_WIDTH}
                value={rawRoot ?? ''}
                placeholder="results"
                onChange={onInputChange('rawRoot')}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>
        </>
      )}

      {/* ---------- Owners type needs no extra fields ---------- */}
    </div>
  );
}
